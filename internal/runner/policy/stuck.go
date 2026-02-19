package policy

import (
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
)

// StuckPolicy decides whether a bead is considered stuck based on stats.
type StuckPolicy interface {
	IsStuck(b *bead.Bead, stats map[string]logger.BeadStats) bool
}

// ThresholdStuckPolicy marks beads as stuck when failures meet/exceed a threshold.
type ThresholdStuckPolicy struct {
	threshold int
}

var _ StuckPolicy = (*ThresholdStuckPolicy)(nil)

// NewConfigStuckPolicy returns a StuckPolicy backed by cfg.
func NewConfigStuckPolicy(cfg *config.Config) StuckPolicy {
	return NewThresholdStuckPolicy(cfg.Loop.StuckBeadThreshold)
}

// NewThresholdStuckPolicy returns a StuckPolicy that uses the provided threshold.
func NewThresholdStuckPolicy(threshold int) StuckPolicy {
	return &ThresholdStuckPolicy{threshold: threshold}
}

// IsStuck returns true when the bead's failures meet/exceed the threshold.
func (p *ThresholdStuckPolicy) IsStuck(b *bead.Bead, stats map[string]logger.BeadStats) bool {
	if p == nil || b == nil {
		return false
	}
	if p.threshold <= 0 {
		return false
	}
	beadStats, ok := stats[b.ID]
	if !ok {
		return false
	}
	return beadStats.Failures >= p.threshold
}
