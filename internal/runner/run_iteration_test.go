package runner

import (
	"context"
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
