package loop

import (
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/queue"
)

// TopologicalSort returns beads sorted so all dependencies appear before their
// dependents. Beads with no dependency relationship preserve their original
// relative order (stable). Returns an error if a dependency cycle is detected.
func TopologicalSort(beads []*bead.Bead) ([]*bead.Bead, error) {
	// Build an index of bead position for stable ordering
	index := make(map[string]int, len(beads))
	for i, b := range beads {
		if b == nil {
			continue
		}
		index[b.ID] = i
	}

	// Compute indegrees and adjacency within the provided set
	indegree := make(map[string]int, len(beads))
	dependents := make(map[string][]string, len(beads))

	for _, b := range beads {
		if b == nil {
			continue
		}
		if _, ok := indegree[b.ID]; !ok {
			indegree[b.ID] = 0
		}
	}

	for _, b := range beads {
		if b == nil {
			continue
		}
		deps := beadDependencyIDs(b)
		for _, dep := range deps {
			if dep == "" {
				continue
			}
			if _, ok := indegree[dep]; !ok {
				// dep not in set — treat as already satisfied
				continue
			}
			indegree[b.ID]++
			dependents[dep] = append(dependents[dep], b.ID)
		}
	}

	// Kahn's algorithm; break ties by original position for stable output
	queue := make([]string, 0, len(beads))
	for _, b := range beads {
		if b == nil {
			continue
		}
		if indegree[b.ID] == 0 {
			queue = append(queue, b.ID)
		}
	}

	beadByID := make(map[string]*bead.Bead, len(beads))
	for _, b := range beads {
		if b != nil {
			beadByID[b.ID] = b
		}
	}

	result := make([]*bead.Bead, 0, len(beads))
	for len(queue) > 0 {
		// Pick next by original position (stable)
		minIdx := 0
		for i := 1; i < len(queue); i++ {
			if index[queue[i]] < index[queue[minIdx]] {
				minIdx = i
			}
		}
		cur := queue[minIdx]
		queue = append(queue[:minIdx], queue[minIdx+1:]...)
		result = append(result, beadByID[cur])

		for _, dep := range dependents[cur] {
			indegree[dep]--
			if indegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	// Count non-nil beads to check for cycle
	total := 0
	for _, b := range beads {
		if b != nil {
			total++
		}
	}
	if len(result) != total {
		return nil, fmt.Errorf("cycle detected in bead dependency graph")
	}

	return result, nil
}

// beadDependencyIDs collects all dependency IDs from a bead's dependency fields.
func beadDependencyIDs(b *bead.Bead) []string {
	if b == nil {
		return nil
	}
	var deps []string
	deps = append(deps, queue.DependencyIDs(b.DependsOn)...)
	deps = append(deps, queue.DependencyIDs(b.Dependencies)...)
	deps = append(deps, queue.DependencyIDs(b.BlockedBy)...)
	return filterEmpty(deps)
}

func filterEmpty(ss []string) []string {
	out := ss[:0]
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
