package prepare

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
)

// SpecSPCBlocker implements DataQualityBlocker to block beads whose spec:* label
// maps to a high-severity rework anomaly (special cause classification) in the process trend.
type SpecSPCBlocker struct {
	records []logger.CauseClassificationRecord
}

// NewSpecSPCBlocker creates a new SpecSPCBlocker with the given cause classification records.
func NewSpecSPCBlocker(records []logger.CauseClassificationRecord) *SpecSPCBlocker {
	return &SpecSPCBlocker{
		records: records,
	}
}

// ShouldBlock checks whether a bead should be blocked based on its spec label and
// special cause anomalies in the process trend.
// Returns (blocked, reason, error).
// A bead is blocked if its spec:* label matches a record with:
// - Class == CauseClassSpecial
// - Severity == "high"
func (s *SpecSPCBlocker) ShouldBlock(ctx context.Context, b *bead.Bead) (bool, string, error) {
	if b == nil {
		return false, "", nil
	}

	spec := bead.FindSpecLabel(b.Labels)
	if spec == "" {
		return false, "", nil
	}

	// Look for a matching record with special cause classification
	for _, rec := range s.records {
		if rec.Stratum == fmt.Sprintf("spec:%s", spec) && rec.Class == logger.CauseClassSpecial && rec.Severity == "high" {
			return true, fmt.Sprintf("spec:%s maintenance warning", spec), nil
		}
	}

	return false, "", nil
}
