package dep

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
