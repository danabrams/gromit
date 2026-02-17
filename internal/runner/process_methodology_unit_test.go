package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// newMinimalRunnerForMethodology creates the smallest possible Runner for
// testing prepareMethodologyForBead without needing a full Deps setup.
func newMinimalRunnerForMethodology(t *testing.T, cfg *config.Config, renderer PromptRenderer) *Runner {
	t.Helper()
	if cfg == nil {
		cfg = &config.Config{}
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	var buf strings.Builder
	sw := newSyncWriter(&buf)
	return &Runner{
		cfg:      cfg,
		renderer: renderer,
		output:   sw,
		syncOut:  sw,
	}
}

// newBeadContext creates a minimal BeadContext for testing prepareMethodologyForBead.
func newBeadContextForMethodology(b *bead.Bead) *runtypes.BeadContext {
	return &runtypes.BeadContext{
		Bead:      b,
		Result:    &runtypes.IterationResult{},
		PromptCtx: &prompt.Context{},
	}
}

// --- ATDD skip tests ---

// TestPrepareMethodology_ATDDSkippedForTestOnlyBead verifies that
// prepareMethodologyForBead returns atddActive=false when the bead title
// matches a test-only pattern, even when ATDD is globally enabled.
func TestPrepareMethodology_ATDDSkippedForTestOnlyBead(t *testing.T) {
	titles := []string{
		"Add tests for bead validation",
		"Add unit tests for config loading",
		"Write tests for runner loop",
		"Write unit tests for prompt rendering",
	}

	for _, title := range titles {
		t.Run(title, func(t *testing.T) {
			cfg := &config.Config{
				Methodology: config.MethodologyConfig{ATDD: true},
			}
			r := newMinimalRunnerForMethodology(t, cfg, &mockPromptRenderer{})
			b := &bead.Bead{
				ID:     "test-skip-1",
				Title:  title,
				Labels: []string{},
			}
			bc := newBeadContextForMethodology(b)

			atddActive, _, _ := r.prepareMethodologyForBead(context.Background(), bc)

			if atddActive {
				t.Errorf("prepareMethodologyForBead should skip ATDD for test-only bead title %q, got atddActive=true", title)
			}
		})
	}
}
