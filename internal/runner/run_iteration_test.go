package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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

	r := &Runner{
		cfg: &config.Config{SpecGate: config.SpecGateConfig{Enabled: false}},
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
	r := &Runner{
		cfg: &config.Config{
			Paths:    config.PathsConfig{Specs: specsDir},
			SpecGate: config.SpecGateConfig{Enabled: true, AutoTrigger: &auto, MaxCycles: 1},
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
