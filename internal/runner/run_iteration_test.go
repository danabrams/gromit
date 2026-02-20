package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/specgate"
)

func TestMaybeRunSpecGate_DisabledSkips(t *testing.T) {
	var listCalls int
	var runTestsCalls int
	var invokeCalls int

	beads := &mockBeadClient{
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			listCalls++
			return []*bead.Bead{}, nil
		},
	}

	gate := &specgate.Gate{
		RunTests: func(ctx context.Context) (string, error) {
			runTestsCalls++
			return "", nil
		},
		GetDiff: func(ctx context.Context) (string, error) {
			return "", nil
		},
		RenderPrompt: func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
			return "", nil
		},
		InvokeLLM: func(ctx context.Context, model, prompt string) ([]byte, error) {
			invokeCalls++
			return []byte(`{"passed": true, "results": []}`), nil
		},
		Model:     "sonnet",
		MaxCycles: 1,
	}

	enabled := false
	r := &Runner{
		cfg:   &config.Config{SpecGate: config.SpecGateConfig{Enabled: &enabled}},
		beads: beads,
		// specGate should be ignored because the feature is disabled
		specGate: gate,
	}

	st := &runLoopState{specGateCycles: make(map[string]int)}

	if err := r.maybeRunSpecGate(context.Background(), st, "demo-spec"); err != nil {
		t.Fatalf("maybeRunSpecGate() error: %v", err)
	}
	if listCalls != 0 {
		t.Fatalf("expected no ListWithLabel calls, got %d", listCalls)
	}
	if runTestsCalls != 0 {
		t.Fatalf("expected no RunTests calls, got %d", runTestsCalls)
	}
	if invokeCalls != 0 {
		t.Fatalf("expected no InvokeLLM calls, got %d", invokeCalls)
	}
}

func TestMaybeRunSpecGate_OpenBeadsSkip(t *testing.T) {
	var listCalls int
	var runTestsCalls int

	beads := &mockBeadClient{
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			listCalls++
			return []*bead.Bead{{ID: "bead-1", Status: "open"}}, nil
		},
	}

	gate := &specgate.Gate{
		RunTests: func(ctx context.Context) (string, error) {
			runTestsCalls++
			return "", nil
		},
		GetDiff: func(ctx context.Context) (string, error) {
			return "", nil
		},
		RenderPrompt: func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
			return "", nil
		},
		InvokeLLM: func(ctx context.Context, model, prompt string) ([]byte, error) {
			return []byte(`{"passed": true, "results": []}`), nil
		},
		Model:     "sonnet",
		MaxCycles: 1,
	}

	specsDir := t.TempDir()
	specPath := filepath.Join(specsDir, "demo-spec.md")
	if err := os.WriteFile(specPath, []byte("spec body"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	auto := true
	enabled := true
	r := &Runner{
		cfg: &config.Config{
			Paths:    config.PathsConfig{Specs: specsDir},
			SpecGate: config.SpecGateConfig{Enabled: &enabled, AutoTrigger: &auto, MaxCycles: 1},
		},
		beads:    beads,
		specGate: gate,
	}

	st := &runLoopState{specGateCycles: make(map[string]int)}

	if err := r.maybeRunSpecGate(context.Background(), st, "demo-spec"); err != nil {
		t.Fatalf("maybeRunSpecGate() error: %v", err)
	}
	if listCalls != 1 {
		t.Fatalf("expected one ListWithLabel call, got %d", listCalls)
	}
	if runTestsCalls != 0 {
		t.Fatalf("expected no RunTests calls, got %d", runTestsCalls)
	}
	if st.specGateCycles["demo-spec"] != 0 {
		t.Fatalf("expected cycle count to remain 0, got %d", st.specGateCycles["demo-spec"])
	}
}

func TestMaybeRunSpecGate_RunsAndIncrements(t *testing.T) {
	var runTestsCalls int
	var renderCalls int
	var invokeCalls int

	beads := &mockBeadClient{
		ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
			return []*bead.Bead{{ID: "bead-1", Status: "closed"}}, nil
		},
	}

	gate := &specgate.Gate{
		RunTests: func(ctx context.Context) (string, error) {
			runTestsCalls++
			return "tests ok", nil
		},
		GetDiff: func(ctx context.Context) (string, error) {
			return "diff", nil
		},
		RenderPrompt: func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
			renderCalls++
			if specName != "demo-spec" {
				t.Fatalf("unexpected specName %q", specName)
			}
			if len(criteria) != 1 || criteria[0] != "works" {
				t.Fatalf("unexpected criteria: %#v", criteria)
			}
			return "prompt", nil
		},
		InvokeLLM: func(ctx context.Context, model, prompt string) ([]byte, error) {
			invokeCalls++
			return []byte(`{"passed": true, "results": [{"criterion":"works","passed":true,"evidence":"ok"}]}`), nil
		},
		Model:     "sonnet",
		MaxCycles: 1,
	}

	specsDir := t.TempDir()
	specBody := "## Acceptance Criteria\n- works\n"
	specPath := filepath.Join(specsDir, "demo-spec.md")
	if err := os.WriteFile(specPath, []byte(specBody), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	auto := true
	enabled := true
	r := &Runner{
		cfg: &config.Config{
			Paths:    config.PathsConfig{Specs: specsDir},
			SpecGate: config.SpecGateConfig{Enabled: &enabled, AutoTrigger: &auto, MaxCycles: 2},
		},
		beads:    beads,
		specGate: gate,
	}

	st := &runLoopState{specGateCycles: make(map[string]int)}

	if err := r.maybeRunSpecGate(context.Background(), st, "demo-spec"); err != nil {
		t.Fatalf("maybeRunSpecGate() error: %v", err)
	}
	if runTestsCalls != 1 {
		t.Fatalf("expected RunTests to be called once, got %d", runTestsCalls)
	}
	if renderCalls != 1 {
		t.Fatalf("expected RenderPrompt to be called once, got %d", renderCalls)
	}
	if invokeCalls != 1 {
		t.Fatalf("expected InvokeLLM to be called once, got %d", invokeCalls)
	}
	if st.specGateCycles["demo-spec"] != 1 {
		t.Fatalf("expected cycle count to be 1, got %d", st.specGateCycles["demo-spec"])
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

	gate := &specgate.Gate{
		RunTests: func(ctx context.Context) (string, error) {
			if !syncCalled {
				t.Fatalf("expected sync to occur before spec gate")
			}
			runTestsCalls++
			return "tests ok", nil
		},
		GetDiff: func(ctx context.Context) (string, error) {
			return "diff", nil
		},
		RenderPrompt: func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
			return "prompt", nil
		},
		InvokeLLM: func(ctx context.Context, model, prompt string) ([]byte, error) {
			return []byte(`{"passed": true, "results": [{"criterion":"works","passed":true,"evidence":"ok"}]}`), nil
		},
		Model:     "sonnet",
		MaxCycles: 1,
	}

	specsDir := t.TempDir()
	specBody := "## Acceptance Criteria\n- works\n"
	specPath := filepath.Join(specsDir, "demo-spec.md")
	if err := os.WriteFile(specPath, []byte(specBody), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	auto := true
	enabled := true
	r := &Runner{
		cfg: &config.Config{
			Paths:    config.PathsConfig{Specs: specsDir},
			SpecGate: config.SpecGateConfig{Enabled: &enabled, AutoTrigger: &auto, MaxCycles: 2},
		},
		beads:    beads,
		specGate: gate,
	}

	st := &runLoopState{specGateCycles: make(map[string]int)}
	b := &bead.Bead{ID: "b1", Title: "Spec bead", Labels: []string{"spec:demo-spec"}}
	result := &IterationResult{Model: "sonnet"}

	if err := r.handleSuccessfulIteration(context.Background(), b, st, result, 1, time.Time{}, func(int) {}); err != nil {
		t.Fatalf("handleSuccessfulIteration() error: %v", err)
	}
	if runTestsCalls != 1 {
		t.Fatalf("expected RunTests to be called once, got %d", runTestsCalls)
	}
}
