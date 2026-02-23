package escalation

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// RED: Test that P0 bead with 2 low-complexity signals routes to TierLow
func TestSelectTier_LowComplexitySignalsOverrideHighPriority(t *testing.T) {
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0: provider.TierHigh,
			P1: provider.TierMedium,
			P2: provider.TierLow,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// P0 bead: rename title (1 signal) + nil DependentCount/leaf (1 signal) = 2 signals
	b := &bead.Bead{
		ID:       "high-priority-low-complexity",
		Title:    "Rename FooBar to BazQux",
		Priority: 0,
		Labels:   []string{},
	}

	result := SelectTier(cfg, b)
	if result != provider.TierLow {
		t.Errorf("SelectTier() = %q, want %q for P0 low-complexity bead", result, provider.TierLow)
	}
}

// RED: Test that file count 4 does NOT contribute a signal
func TestCountLowComplexitySignals_FileCountAboveMax(t *testing.T) {
	one := 1
	b := &bead.Bead{
		Title: "Update auth logic",
		ExpectedOutputs: []string{
			"file1.go", "file2.go", "file3.go", "file4.go",
		},
		DependentCount: &one, // not leaf
	}
	got := countLowComplexitySignals(&config.Config{}, b)
	if got != 0 {
		t.Errorf("countLowComplexitySignals() = %d, want 0 for 4-file non-leaf no-pattern bead", got)
	}
}
