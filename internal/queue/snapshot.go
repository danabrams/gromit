package queue

import (
	"fmt"
	"strings"

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
		// In-progress beads are active work and should not be shown as blocked.
		if strings.EqualFold(strings.TrimSpace(b.Status), "in_progress") {
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

// GetReason returns a human-readable reason why a bead is blocked.
func GetReason(b *bead.Bead, allBeads []*bead.Bead) string {
	if b == nil {
		return "unknown"
	}

	if b.Parent != "" {
		// Check if parent still exists in open beads
		for _, openB := range allBeads {
			if openB.ID == b.Parent {
				return fmt.Sprintf("blocked by: %s", b.Parent)
			}
		}
		return fmt.Sprintf("blocked by parent: %s", b.Parent)
	}

	if depIDs := DependencyIDs(b.BlockedBy); len(depIDs) > 0 {
		return fmt.Sprintf("blocked by: %s", strings.Join(depIDs, ", "))
	}
	if depIDs := DependencyIDs(b.DependsOn); len(depIDs) > 0 {
		return fmt.Sprintf("blocked by: %s", strings.Join(depIDs, ", "))
	}
	if depIDs := DependencyIDs(b.Dependencies); len(depIDs) > 0 {
		return fmt.Sprintf("blocked by: %s", strings.Join(depIDs, ", "))
	}
	if b.DependencyCount != nil && *b.DependencyCount > 0 {
		return fmt.Sprintf("blocked by %d dependencies", *b.DependencyCount)
	}

	return "dependencies unresolved"
}

// DependencyIDs extracts IDs from a list of dependencies.
func DependencyIDs(deps []bead.Dependency) []string {
	ids := make([]string, 0, len(deps))
	for _, dep := range deps {
		if strings.TrimSpace(dep.ID) == "" {
			continue
		}
		ids = append(ids, dep.ID)
	}
	return ids
}

// EnrichReadyBeads merges labels and other fields from all beads into ready beads.
func EnrichReadyBeads(readyBeads, allBeads []*bead.Bead) []*bead.Bead {
	if len(readyBeads) == 0 {
		return readyBeads
	}
	openByID := make(map[string]*bead.Bead, len(allBeads))
	for _, b := range allBeads {
		if b == nil || strings.TrimSpace(b.ID) == "" {
			continue
		}
		openByID[b.ID] = b
	}
	enriched := make([]*bead.Bead, 0, len(readyBeads))
	for _, b := range readyBeads {
		if b == nil {
			continue
		}
		if open, ok := openByID[b.ID]; ok && open != nil {
			clone := *b
			clone.Labels = mergeLabels(b.Labels, open.Labels)
			if clone.Parent == "" {
				clone.Parent = open.Parent
			}
			enriched = append(enriched, &clone)
			continue
		}
		enriched = append(enriched, b)
	}
	return enriched
}

// mergeLabels combines two label lists, removing duplicates and empty strings.
func mergeLabels(primary, secondary []string) []string {
	if len(primary) == 0 && len(secondary) == 0 {
		return []string{}
	}
	seen := make(map[string]bool, len(primary)+len(secondary))
	out := make([]string, 0, len(primary)+len(secondary))
	for _, label := range primary {
		if strings.TrimSpace(label) == "" || seen[label] {
			continue
		}
		out = append(out, label)
		seen[label] = true
	}
	for _, label := range secondary {
		if strings.TrimSpace(label) == "" || seen[label] {
			continue
		}
		out = append(out, label)
		seen[label] = true
	}
	return out
}
