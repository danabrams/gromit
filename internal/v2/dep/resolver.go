package dep

import "fmt"

// Resolver tracks dependencies between beads and determines execution order.
type Resolver struct {
	beads map[string][]string // beadID -> list of dependencies
}

// NewResolver creates a new dependency resolver.
func NewResolver() *Resolver {
	return &Resolver{
		beads: make(map[string][]string),
	}
}

// Add registers a bead and its dependencies.
func (r *Resolver) Add(beadID string, dependsOn []string) {
	if dependsOn == nil {
		dependsOn = []string{}
	}
	r.beads[beadID] = dependsOn
}

// Next returns the next bead that can be executed given the completed beads.
// Returns error if there are no eligible beads (deadlock/cycle detected).
func (r *Resolver) Next(completed []string) (string, error) {
	// First, detect cycles in the unresolved beads
	if err := r.detectCycles(completed); err != nil {
		return "", err
	}

	completedSet := make(map[string]bool)
	for _, c := range completed {
		completedSet[c] = true
	}

	// Find first bead whose dependencies are all completed
	for beadID, deps := range r.beads {
		if completedSet[beadID] {
			continue // Skip already completed beads
		}

		allDepsCompleted := true
		for _, dep := range deps {
			if !completedSet[dep] {
				allDepsCompleted = false
				break
			}
		}

		if allDepsCompleted {
			return beadID, nil
		}
	}

	return "", nil
}

// detectCycles checks if there are any cycles in the unresolved beads.
func (r *Resolver) detectCycles(completed []string) error {
	completedSet := make(map[string]bool)
	for _, c := range completed {
		completedSet[c] = true
	}

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for beadID := range r.beads {
		if completedSet[beadID] {
			continue
		}
		if !visited[beadID] {
			if hasCycle := r.dfs(beadID, visited, recStack, completedSet); hasCycle {
				return fmt.Errorf("cycle detected in dependencies involving %s", beadID)
			}
		}
	}

	return nil
}

// dfs performs depth-first search to detect cycles.
func (r *Resolver) dfs(beadID string, visited, recStack, completed map[string]bool) bool {
	visited[beadID] = true
	recStack[beadID] = true

	deps := r.beads[beadID]
	for _, dep := range deps {
		if completed[dep] {
			continue // Skip completed dependencies
		}

		if !visited[dep] {
			if r.dfs(dep, visited, recStack, completed) {
				return true
			}
		} else if recStack[dep] {
			return true // Cycle detected
		}
	}

	recStack[beadID] = false
	return false
}
