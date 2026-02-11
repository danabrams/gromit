package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
)

// TestDecomposeTaskATDDActiveWiring verifies that DecomposeTask() correctly
// computes ATDD-active status from bead labels and config, then passes it
// to the renderer via DecomposeContext.ATDDActive.
//
// Expected failure: DecomposeContext.ATDDActive field does not exist or
// DecomposeTask does not set it based on bead.IsMethodologyActive
func TestDecomposeTaskATDDActiveWiring(t *testing.T) {
	validJSON := `[{"title":"Sub 1","description":"First","depends_on":null,"acceptance_criteria":["Done"]}]`

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedCtx *prompt.DecomposeContext

			mockRend := &mockPromptRenderer{
				RenderDecomposeFn: func(ctx *prompt.DecomposeContext) (string, error) {
					capturedCtx = ctx
					return "mock decompose prompt", nil
				},
			}

			mockClaude := &mockClaudeClient{
				RunFn: func(ctx context.Context, p string, model string) (*claude.Result, error) {
					return &claude.Result{Success: true, Output: validJSON}, nil
				},
			}

			cfg := &config.Config{
				Methodology: config.MethodologyConfig{ATDD: tt.cfgATDD},
			}

			var buf strings.Builder
			r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(),
				Deps{Beads: &mockBeadClient{}, Claude: mockClaude, Analyzer: &mockFailureAnalyzer{}, Renderer: mockRend, Logger: &mockIterationLogger{}})
			if err != nil {
				t.Fatalf("NewRunnerWithDeps() error = %v", err)
			}

			b := &bead.Bead{
				ID:              "decompose-1",
				Title:           "Feature to decompose",
				Priority:        1,
				Labels:          tt.labels,
				ExpectedOutputs: []string{},
			}

			_, err = r.DecomposeTask(context.Background(), b)
			if err != nil {
				t.Fatalf("DecomposeTask() error = %v", err)
			}

			if capturedCtx == nil {
				t.Fatal("RenderDecompose was not called")
			}

			if capturedCtx.ATDDActive != tt.wantATDDActive {
				t.Errorf("ATDDActive = %v, want %v", capturedCtx.ATDDActive, tt.wantATDDActive)
			}
		})
	}
}
