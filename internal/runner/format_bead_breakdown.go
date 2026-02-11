package runner

import (
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/pipeline"
)

// formatBeadBreakdown builds the comma-separated status breakdown for the Beads line
func formatBeadBreakdown(ps *pipeline.PipelineStatus) string {
	if ps == nil {
		return "none"
	}

	var parts []string

	// Build parts in lifecycle order: ready, in-progress, blocked, deferred, closed
	if ps.ReadyBeadCount > 0 {
		parts = append(parts, fmt.Sprintf("%d ready", ps.ReadyBeadCount))
	}
	if ps.InProgressCount > 0 {
		parts = append(parts, fmt.Sprintf("%d in-progress", ps.InProgressCount))
	}
	if ps.BlockedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked", ps.BlockedCount))
	}
	if ps.DeferredCount > 0 {
		parts = append(parts, fmt.Sprintf("%d deferred", ps.DeferredCount))
	}

	// Handle closed with optional "this run" parenthetical
	if ps.ClosedCount > 0 {
		closedPart := fmt.Sprintf("%d closed", ps.ClosedCount)
		if ps.HasRunInfo && ps.ClosedThisRunCount > 0 {
			closedPart += fmt.Sprintf(" (%d this run)", ps.ClosedThisRunCount)
		}
		parts = append(parts, closedPart)
	}

	// If all counts are zero, return "none"
	if len(parts) == 0 {
		return "none"
	}

	return strings.Join(parts, ", ")
}
