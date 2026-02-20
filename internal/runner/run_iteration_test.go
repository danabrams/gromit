package runner

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/specgate"
)

const (
	demoSpecName  = "demo-spec"
	demoSpecLabel = "spec:demo-spec"
)

func TestMaybeRunSpecGate_AutoTriggerDecisionMatrix(t *testing.T) {
	specsDir := writeSpecGateTestSpec(t, demoSpecName)

	tests := []struct {
		name              string
		enabled           bool
		autoTrigger       bool
		beadLabels        []string
		labelFilters      []string
		openSpecBeads     bool
		maxCycles         int
		initialCycles     int
		hasGate           bool
		hasBeadsClient    bool
		wantListCalls     int
		wantRunTestsCalls int
		wantCycles        int
	}{
		{
			name:              "runs when enabled scoped no open beads and cycle budget remains",
			enabled:           true,
			autoTrigger:       true,
			beadLabels:        []string{demoSpecLabel},
			labelFilters:      []string{demoSpecLabel},
			openSpecBeads:     false,
			maxCycles:         2,
			initialCycles:     0,
			hasGate:           true,
			hasBeadsClient:    true,
			wantListCalls:     1,
			wantRunTestsCalls: 1,
			wantCycles:        1,
		},
		{
			name:              "skips when bead has no spec label",
			enabled:           true,
			autoTrigger:       true,
			beadLabels:        []string{"priority:p1"},
			labelFilters:      []string{demoSpecLabel},
			maxCycles:         2,
			initialCycles:     0,
			hasGate:           true,
			hasBeadsClient:    true,
			wantListCalls:     0,
			wantRunTestsCalls: 0,
			wantCycles:        0,
		},
		{
			name:              "skips when run is not scoped",
			enabled:           true,
			autoTrigger:       true,
			beadLabels:        []string{demoSpecLabel},
			labelFilters:      nil,
			maxCycles:         2,
			initialCycles:     0,
			hasGate:           true,
			hasBeadsClient:    true,
			wantListCalls:     0,
			wantRunTestsCalls: 0,
			wantCycles:        0,
		},
		{
			name:              "skips when auto-trigger disabled",
			enabled:           true,
			autoTrigger:       false,
			beadLabels:        []string{demoSpecLabel},
			labelFilters:      []string{demoSpecLabel},
			maxCycles:         2,
			initialCycles:     0,
			hasGate:           true,
			hasBeadsClient:    true,
			wantListCalls:     0,
			wantRunTestsCalls: 0,
			wantCycles:        0,
		},
		{
			name:              "skips when spec gate disabled",
			enabled:           false,
			autoTrigger:       true,
			beadLabels:        []string{demoSpecLabel},
			labelFilters:      []string{demoSpecLabel},
			maxCycles:         2,
			initialCycles:     0,
			hasGate:           true,
			hasBeadsClient:    true,
			wantListCalls:     0,
			wantRunTestsCalls: 0,
			wantCycles:        0,
		},
		{
			name:              "skips when open beads remain",
			enabled:           true,
			autoTrigger:       true,
			beadLabels:        []string{demoSpecLabel},
			labelFilters:      []string{demoSpecLabel},
			openSpecBeads:     true,
			maxCycles:         2,
			initialCycles:     0,
			hasGate:           true,
			hasBeadsClient:    true,
			wantListCalls:     1,
			wantRunTestsCalls: 0,
			wantCycles:        0,
		},
		{
			name:              "skips when max cycles exhausted",
			enabled:           true,
			autoTrigger:       true,
			beadLabels:        []string{demoSpecLabel},
			labelFilters:      []string{demoSpecLabel},
			maxCycles:         2,
			initialCycles:     2,
			hasGate:           true,
			hasBeadsClient:    true,
			wantListCalls:     1,
			wantRunTestsCalls: 0,
			wantCycles:        2,
		},
		{
			name:              "skips when gate is not wired",
			enabled:           true,
			autoTrigger:       true,
			beadLabels:        []string{demoSpecLabel},
			labelFilters:      []string{demoSpecLabel},
			maxCycles:         2,
			initialCycles:     0,
			hasGate:           false,
			hasBeadsClient:    true,
			wantListCalls:     0,
			wantRunTestsCalls: 0,
			wantCycles:        0,
		},
		{
			name:              "skips when bead client is not wired",
			enabled:           true,
			autoTrigger:       true,
			beadLabels:        []string{demoSpecLabel},
			labelFilters:      []string{demoSpecLabel},
			maxCycles:         2,
			initialCycles:     0,
			hasGate:           true,
			hasBeadsClient:    false,
			wantListCalls:     0,
			wantRunTestsCalls: 0,
			wantCycles:        0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var listCalls int
			var runTestsCalls int

			beads := &mockBeadClient{
				ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
					listCalls++
					if tt.openSpecBeads {
						return []*bead.Bead{{ID: "bead-1", Status: "open"}}, nil
					}
					return []*bead.Bead{{ID: "bead-1", Status: "closed"}}, nil
				},
			}

			r := &Runner{
				cfg: &config.Config{
					Paths:    config.PathsConfig{Specs: specsDir},
					SpecGate: config.SpecGateConfig{Enabled: &tt.enabled, AutoTrigger: &tt.autoTrigger, MaxCycles: tt.maxCycles},
				},
				labelFilters: tt.labelFilters,
			}
			if tt.hasBeadsClient {
				r.beads = beads
			}
			if tt.hasGate {
				r.specGate = newSpecGateStub(
					func(ctx context.Context) (string, error) {
						runTestsCalls++
						return "VALIDATION_PASSED", nil
					},
					`{"passed":true,"results":[]}`,
				)
			}

			st := &runLoopState{specGateCycles: map[string]int{demoSpecName: tt.initialCycles}}
			b := &bead.Bead{ID: "b1", Title: "Spec bead", Labels: tt.beadLabels}

			if err := r.maybeRunSpecGate(context.Background(), b, st); err != nil {
				t.Fatalf("maybeRunSpecGate() error: %v", err)
			}
			if listCalls != tt.wantListCalls {
				t.Fatalf("ListWithLabel calls = %d, want %d", listCalls, tt.wantListCalls)
			}
			if runTestsCalls != tt.wantRunTestsCalls {
				t.Fatalf("RunTests calls = %d, want %d", runTestsCalls, tt.wantRunTestsCalls)
			}
			if got := st.specGateCycles[demoSpecName]; got != tt.wantCycles {
				t.Fatalf("specGateCycles[%s] = %d, want %d", demoSpecName, got, tt.wantCycles)
			}
		})
	}
}

func TestMaybeRunSpecGate_FailedVerdictSynthesizesSpecLabeledFixBeads(t *testing.T) {
	specsDir := writeSpecGateTestSpec(t, demoSpecName)
	var createdLabels []string

	beads := &mockBeadClient{
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			return []*bead.Bead{{ID: "bead-1", Status: "closed"}}, nil
		},
		CreateFn: func(title string, priority int, labels []string, expectedOutputs []string) (*bead.Bead, error) {
			createdLabels = append(createdLabels, labels...)
			return &bead.Bead{ID: "fix-1"}, nil
		},
	}

	auto := true
	enabled := true
	r := &Runner{
		cfg: &config.Config{
			Paths:    config.PathsConfig{Specs: specsDir},
			SpecGate: config.SpecGateConfig{Enabled: &enabled, AutoTrigger: &auto, MaxCycles: 2},
		},
		beads:        beads,
		labelFilters: []string{demoSpecLabel},
		specGate: newSpecGateStub(
			func(ctx context.Context) (string, error) {
				return "tests failing", nil
			},
			`{"passed":false,"results":[{"criterion":"works","passed":false,"evidence":"failed"}]}`,
		),
	}

	st := &runLoopState{specGateCycles: map[string]int{}}
	b := &bead.Bead{ID: "b1", Title: "Spec bead", Labels: []string{demoSpecLabel}}
	if err := r.maybeRunSpecGate(context.Background(), b, st); err != nil {
		t.Fatalf("maybeRunSpecGate() error: %v", err)
	}
	if len(createdLabels) != 1 || createdLabels[0] != demoSpecLabel {
		t.Fatalf("created labels = %v, want [%s]", createdLabels, demoSpecLabel)
	}
}

func TestVerifyScopedSpecAcceptance_GateFailureSchedulesRetry(t *testing.T) {
	specsDir := writeSpecGateTestSpec(t, demoSpecName)
	var created int

	enabled := true
	r := &Runner{
		cfg: &config.Config{
			Paths:       config.PathsConfig{Specs: specsDir},
			SpecGate:    config.SpecGateConfig{Enabled: &enabled},
			Methodology: config.MethodologyConfig{SpecGateMaxRetries: 2},
		},
		output:           io.Discard,
		labelFilters:     []string{demoSpecLabel},
		specOrchestrator: &SpecOrchestrator{},
		beads: &mockBeadClient{
			CreateFn: func(title string, priority int, labels []string, expectedOutputs []string) (*bead.Bead, error) {
				created++
				return &bead.Bead{ID: "fix-1"}, nil
			},
		},
		specGate: newSpecGateStub(
			func(ctx context.Context) (string, error) {
				return "tests failing", nil
			},
			`{"passed":false,"results":[{"criterion":"works","passed":false,"evidence":"failed"}]}`,
		),
	}

	st := &runLoopState{specGateRetries: map[string]int{}}
	retry, err := r.verifyScopedSpecAcceptance(context.Background(), st, true)
	if err != nil {
		t.Fatalf("verifyScopedSpecAcceptance() error: %v", err)
	}
	if !retry {
		t.Fatal("verifyScopedSpecAcceptance() retry = false, want true")
	}
	if created != 1 {
		t.Fatalf("created fix beads = %d, want 1", created)
	}
	if got := st.specGateRetries[demoSpecName]; got != 1 {
		t.Fatalf("specGateRetries[%s] = %d, want 1", demoSpecName, got)
	}
}

func TestVerifyScopedSpecAcceptance_StopsWhenRetryBudgetExhausted(t *testing.T) {
	specsDir := writeSpecGateTestSpec(t, demoSpecName)
	var created int
	var out bytes.Buffer

	enabled := true
	r := &Runner{
		cfg: &config.Config{
			Paths:       config.PathsConfig{Specs: specsDir},
			SpecGate:    config.SpecGateConfig{Enabled: &enabled},
			Methodology: config.MethodologyConfig{SpecGateMaxRetries: 2},
		},
		output:           &out,
		labelFilters:     []string{demoSpecLabel},
		specOrchestrator: &SpecOrchestrator{},
		beads: &mockBeadClient{
			CreateFn: func(title string, priority int, labels []string, expectedOutputs []string) (*bead.Bead, error) {
				created++
				return &bead.Bead{ID: "fix-1"}, nil
			},
		},
		specGate: newSpecGateStub(
			func(ctx context.Context) (string, error) {
				return "tests failing", nil
			},
			`{"passed":false,"results":[{"criterion":"works","passed":false,"evidence":"failed"}]}`,
		),
	}

	st := &runLoopState{specGateRetries: map[string]int{demoSpecName: 2}}
	retry, err := r.verifyScopedSpecAcceptance(context.Background(), st, true)
	if err != nil {
		t.Fatalf("verifyScopedSpecAcceptance() error: %v", err)
	}
	if retry {
		t.Fatal("verifyScopedSpecAcceptance() retry = true, want false")
	}
	if created != 0 {
		t.Fatalf("created fix beads = %d, want 0", created)
	}
	if got := st.specGateRetries[demoSpecName]; got != 2 {
		t.Fatalf("specGateRetries[%s] = %d, want 2", demoSpecName, got)
	}
	if !strings.Contains(out.String(), "spec_gate_retry_exhausted spec=demo-spec retries=2 max_retries=2") {
		t.Fatalf("expected structured retry exhaustion log, got: %q", out.String())
	}
}

func TestHandleSuccessfulIteration_RunsSpecGateAfterSync(t *testing.T) {
	var syncCalled bool
	var runTestsCalls int

	beads := &mockBeadClient{
		CloseFn: func(id string) error {
			return nil
		},
		SyncFn: func() error {
			syncCalled = true
			return nil
		},
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			return []*bead.Bead{}, nil
		},
	}

	specsDir := writeSpecGateTestSpec(t, demoSpecName)
	auto := true
	enabled := true
	r := &Runner{
		cfg: &config.Config{
			Paths:    config.PathsConfig{Specs: specsDir},
			SpecGate: config.SpecGateConfig{Enabled: &enabled, AutoTrigger: &auto, MaxCycles: 2},
		},
		beads:        beads,
		labelFilters: []string{demoSpecLabel},
		specGate: newSpecGateStub(
			func(ctx context.Context) (string, error) {
				if !syncCalled {
					t.Fatalf("expected sync to occur before spec gate")
				}
				runTestsCalls++
				return "VALIDATION_PASSED", nil
			},
			`{"passed":true,"results":[]}`,
		),
	}

	st := &runLoopState{specGateCycles: map[string]int{}}
	b := &bead.Bead{ID: "b1", Title: "Spec bead", Labels: []string{demoSpecLabel}}
	result := &IterationResult{Model: "sonnet"}

	if err := r.handleSuccessfulIteration(context.Background(), b, st, result, 1, time.Time{}, func(int) {}); err != nil {
		t.Fatalf("handleSuccessfulIteration() error: %v", err)
	}
	if runTestsCalls != 1 {
		t.Fatalf("expected RunTests to be called once, got %d", runTestsCalls)
	}
}

func writeSpecGateTestSpec(t *testing.T, specName string) string {
	t.Helper()

	specsDir := t.TempDir()
	specBody := "## Acceptance Criteria\n- works\n"
	specPath := filepath.Join(specsDir, specName+".md")
	if err := os.WriteFile(specPath, []byte(specBody), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	return specsDir
}

func newSpecGateStub(runTestsFn func(ctx context.Context) (string, error), verdictJSON string) *specgate.Gate {
	return &specgate.Gate{
		RunTests: runTestsFn,
		GetDiff: func(ctx context.Context) (string, error) {
			return "", nil
		},
		RenderPrompt: func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
			return "prompt", nil
		},
		InvokeLLM: func(ctx context.Context, model, prompt string) ([]byte, error) {
			return []byte(verdictJSON), nil
		},
		Model: "sonnet",
	}
}
