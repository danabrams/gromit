package queue

import (
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
)

// PartitionQueueBeads separates beads into ready, blocked, and stuck categories.
func PartitionQueueBeads(
	readyBeads, allBeads []*bead.Bead,
	beadStats map[string]logger.BeadStats,
	stuckThreshold int,
) (ready []*bead.Bead, blocked []*bead.Bead, stuck []*bead.Bead) {
	stuckMap := FindStuckBeadIDs(beadStats, stuckThreshold)

	readyMap := make(map[string]bool, len(readyBeads))
	for _, b := range readyBeads {
		readyMap[b.ID] = true
		if !stuckMap[b.ID] {
			ready = append(ready, b)
		}
	}

	for _, b := range allBeads {
		if stuckMap[b.ID] {
			stuck = append(stuck, b)
			continue
		}
		if !readyMap[b.ID] {
			blocked = append(blocked, b)
		}
	}

	return ready, blocked, stuck
}

// FindStuckBeadIDs identifies beads that have exceeded the failure threshold.
func FindStuckBeadIDs(beadStats map[string]logger.BeadStats, threshold int) map[string]bool {
	stuck := make(map[string]bool)
	if threshold <= 0 {
		return stuck
	}
	for beadID, stats := range beadStats {
		if stats.Failures >= threshold {
			stuck[beadID] = true
		}
	}
	return stuck
}
