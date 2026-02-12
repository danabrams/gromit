package runner

import (
	"github.com/danabrams/gromit/internal/provider"
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
)

// setupDecomposeATDDTest creates a runner with mocks for testing ATDDActive wiring
// in DecomposeTask. It captures the DecomposeContext passed to RenderDecompose.
func setupDecomposeATDDTest(t *testing.T, cfgATDD bool) (*Runner, **prompt.DecomposeContext) {
	t.Helper()

	var capturedCtx *prompt.DecomposeContext
	mockRend := &mockPromptRenderer{
		RenderDecomposeFn: func(ctx *prompt.DecomposeContext) (string, error) {
			capturedCtx = ctx
			return "mock decompose prompt", nil
		},
	}
	cfg := &config.Config{
		Methodology: config.MethodologyConfig{ATDD: cfgATDD},
	}
	var buf strings.Builder
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
		Deps{Beads: &mockBeadClient{}, Router: newMockRouter(), Analyzer: &mockFailureAnalyzer{}, Renderer: mockRend, Logger: &mockIterationLogger{}})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}
	return r, &capturedCtx
}

// TestDecomposeTaskATDDActiveWiring verifies that DecomposeTask() correctly
// computes ATDD-active status from bead labels and config, then passes it
// to the renderer via DecomposeContext.ATDDActive.
func TestDecomposeTaskATDDActiveWiring(t *testing.T) {
	tests := []struct {
		name           string
		labels         []string
		cfgATDD        bool
		wantATDDActive bool
	}{
		{
			name:           "label atdd:true overrides config false",
			labels:         []string{"atdd:true"},
			cfgATDD:        false,
			wantATDDActive: true,
		},
		{
			name:           "label atdd:false overrides config true",
			labels:         []string{"atdd:false"},
			cfgATDD:        true,
			wantATDDActive: false,
		},
		{
			name:           "no label falls back to config true",
			labels:         []string{"complexity:high"},
			cfgATDD:        true,
			wantATDDActive: true,
		},
		{
			name:           "no label falls back to config false",
			labels:         []string{},
			cfgATDD:        false,
			wantATDDActive: false,
		},
		{
			name:           "atdd:true with other labels still activates ATDD",
			labels:         []string{"complexity:low", "atdd:true", "spec:something"},
			cfgATDD:        false,
			wantATDDActive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, capturedCtxPtr := setupDecomposeATDDTest(t, tt.cfgATDD)

			b := &bead.Bead{
				ID:              "decompose-1",
				Title:           "Feature to decompose",
				Priority:        1,
				Labels:          tt.labels,
				ExpectedOutputs: []string{},
			}

			_, err := r.DecomposeTask(context.Background(), b)
			if err != nil {
				t.Fatalf("DecomposeTask() error = %v", err)
			}

			capturedCtx := *capturedCtxPtr
			if capturedCtx == nil {
				t.Fatal("RenderDecompose was not called")
			}

			if capturedCtx.ATDDActive != tt.wantATDDActive {
				t.Errorf("DecomposeContext.ATDDActive = %v, want %v (labels=%v, config=%v)",
					capturedCtx.ATDDActive, tt.wantATDDActive, tt.labels, tt.cfgATDD)
			}
		})
	}
}

// TestDecomposeTaskForwardsBeadWithATDDActive verifies that DecomposeTask
// correctly populates the full DecomposeContext — the bead, parent, and
// ATDDActive field are all set together when passed to the renderer.
func TestDecomposeTaskForwardsBeadWithATDDActive(t *testing.T) {
	r, capturedCtxPtr := setupDecomposeATDDTest(t, true)

	b := &bead.Bead{
		ID:              "decompose-ctx-check",
		Title:           "Feature requiring ATDD",
		Priority:        1,
		Labels:          []string{"atdd:true"},
		ExpectedOutputs: []string{},
	}

	_, err := r.DecomposeTask(context.Background(), b)
	if err != nil {
		t.Fatalf("DecomposeTask() error = %v", err)
	}

	capturedCtx := *capturedCtxPtr
	if capturedCtx == nil {
		t.Fatal("RenderDecompose was not called")
	}

	// Verify bead is forwarded alongside ATDDActive
	if capturedCtx.Bead == nil {
		t.Fatal("DecomposeContext.Bead is nil")
	}
	if capturedCtx.Bead.ID != "decompose-ctx-check" {
		t.Errorf("DecomposeContext.Bead.ID = %q, want %q", capturedCtx.Bead.ID, "decompose-ctx-check")
	}

	// Verify ATDDActive is true when bead has atdd:true label
	if !capturedCtx.ATDDActive {
		t.Error("DecomposeContext.ATDDActive = false, want true when bead has atdd:true label")
	}
}
