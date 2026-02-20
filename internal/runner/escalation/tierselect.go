package escalation

import (
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

const (
	lowComplexitySignalThreshold  = 2
	lowFileCountMin               = 1
	lowFileCountMax               = 3
	complexityHighLabel           = "complexity:high"
	lowComplexityTDDDisabledLabel = "tdd:false"
)

// SelectTier returns the abstract tier (high/medium/low) for a bead based on
// its priority, labels, and low-complexity signals. Beads marked with
// complexity:high bypass low-complexity routing and use config selection.
// Returns TierMedium as a safe default for nil inputs.
func SelectTier(cfg *config.Config, b *bead.Bead) string {
	if cfg == nil {
		return provider.TierMedium
	}
	if b == nil {
		return provider.TierMedium
	}

	if bead.HasLabel(b.Labels, complexityHighLabel) {
		return cfg.SelectTier(b.Priority, b.Labels)
	}
	if isLowComplexity(cfg, b) {
		return provider.TierLow
	}
	return cfg.SelectTier(b.Priority, b.Labels)
}

// SelectModel returns the legacy model name (opus/sonnet/haiku) for a bead based on
// its priority and labels. Routes test-only beads to haiku unless a complexity
// label overrides the selection. Returns "sonnet" as a safe default for nil inputs.
func SelectModel(cfg *config.Config, b *bead.Bead) string {
	if b == nil {
		return "sonnet"
	}
	if cfg == nil {
		return "sonnet"
	}
	// Route test-only beads to haiku unless an explicit complexity label overrides
	if bead.IsTestOnlyBead(b.Title) {
		for _, label := range b.Labels {
			if _, ok := cfg.Models.Labels[label]; ok {
				return cfg.SelectModel(b.Priority, b.Labels)
			}
		}
		return "haiku"
	}
	return cfg.SelectModel(b.Priority, b.Labels)
}

func countLowComplexitySignals(_ *config.Config, b *bead.Bead) int {
	if b == nil {
		return 0
	}

	count := 0
	if bead.IsLowComplexityTitle(b.Title) {
		count++
	}
	if bead.IsTestOnlyBead(b.Title) {
		count++
	}
	if bead.HasLabel(b.Labels, lowComplexityTDDDisabledLabel) {
		count++
	}
	fileCount := bead.EstimatedFileCount(b)
	if fileCount >= lowFileCountMin && fileCount <= lowFileCountMax {
		count++
	}
	if bead.IsLeafBead(b) {
		count++
	}

	return count
}

func isLowComplexity(cfg *config.Config, b *bead.Bead) bool {
	return countLowComplexitySignals(cfg, b) >= lowComplexitySignalThreshold
}
