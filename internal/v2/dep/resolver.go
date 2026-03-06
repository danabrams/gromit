package dep

import (
	"fmt"
	"strings"
)

// Resolver tracks bead dependencies and determines the next eligible bead.
type Resolver struct {
	beads    map[string][]string
	addOrder []string
}

// CycleError reports a dependency cycle and the nodes involved.
type CycleError struct {
	Path []string
}

func (e *CycleError) Error() string {
	if len(e.Path) == 0 {
		return "cycle detected"
	}
	return fmt.Sprintf("cycle detected: %s", strings.Join(e.Path, " -> "))
}

// NewResolver constructs a Resolver.
func NewResolver() *Resolver {
	return &Resolver{
		beads: make(map[string][]string),
	}
}

// Add registers a bead and its dependencies.
func (r *Resolver) Add(beadID string, dependsOn []string) {
	if beadID == "" {
		return
	}
	if _, exists := r.beads[beadID]; !exists {
		r.addOrder = append(r.addOrder, beadID)
	}
	r.beads[beadID] = normalizeDependencies(dependsOn)
}

// Next returns the next bead whose dependencies are satisfied or an error when a cycle exists.
func (r *Resolver) Next(completed []string) (string, error) {
	completedSet := make(map[string]struct{}, len(completed))
	for _, beadID := range completed {
		if beadID == "" {
			continue
		}
		completedSet[beadID] = struct{}{}
	}

	pending := r.pendingNodes(completedSet)
	if len(pending) == 0 {
		return "", nil
	}

	order, err := r.topologicalOrder(pending, completedSet)
	if err != nil {
		return "", err
	}

	for _, beadID := range order {
		if r.dependenciesSatisfied(beadID, completedSet) {
			return beadID, nil
		}
	}

	return "", nil
}

func normalizeDependencies(dependsOn []string) []string {
	if len(dependsOn) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(dependsOn))
	result := make([]string, 0, len(dependsOn))
	for _, dep := range dependsOn {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		if _, ok := seen[dep]; ok {
			continue
		}
		seen[dep] = struct{}{}
		result = append(result, dep)
	}
	return result
}

func (r *Resolver) pendingNodes(completed map[string]struct{}) map[string]struct{} {
	pending := make(map[string]struct{})
	for _, beadID := range r.addOrder {
		if _, done := completed[beadID]; done {
			continue
		}
		pending[beadID] = struct{}{}
	}
	return pending
}

func (r *Resolver) topologicalOrder(pending, completed map[string]struct{}) ([]string, error) {
	indegree := make(map[string]int, len(pending))
	dependents := make(map[string][]string, len(pending))

	for _, beadID := range r.addOrder {
		if _, ok := pending[beadID]; !ok {
			continue
		}
		indegree[beadID] = 0
	}

	for _, beadID := range r.addOrder {
		if _, ok := pending[beadID]; !ok {
			continue
		}
		for _, dep := range r.beads[beadID] {
			if _, done := completed[dep]; done {
				continue
			}
			if _, ok := pending[dep]; !ok {
				continue
			}
			indegree[beadID]++
			dependents[dep] = append(dependents[dep], beadID)
		}
	}

	queue := make([]string, 0, len(pending))
	for _, beadID := range r.addOrder {
		if _, ok := pending[beadID]; !ok {
			continue
		}
		if indegree[beadID] == 0 {
			queue = append(queue, beadID)
		}
	}

	order := make([]string, 0, len(pending))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)
		for _, dependent := range dependents[current] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(order) != len(pending) {
		if cycle, ok := r.findCycle(pending, completed); ok {
			return nil, &CycleError{Path: cycle}
		}
		return nil, fmt.Errorf("cycle detected in dependency graph")
	}

	return order, nil
}

func (r *Resolver) dependenciesSatisfied(beadID string, completed map[string]struct{}) bool {
	for _, dep := range r.beads[beadID] {
		if _, ok := completed[dep]; !ok {
			return false
		}
	}
	return true
}

func (r *Resolver) findCycle(pending, completed map[string]struct{}) ([]string, bool) {
	visited := make(map[string]struct{}, len(pending))
	onStack := make(map[string]struct{}, len(pending))
	stack := []string{}

	var dfs func(string) ([]string, bool)
	dfs = func(node string) ([]string, bool) {
		visited[node] = struct{}{}
		onStack[node] = struct{}{}
		stack = append(stack, node)

		for _, dep := range r.beads[node] {
			if _, done := completed[dep]; done {
				continue
			}
			if _, ok := pending[dep]; !ok {
				continue
			}
			if _, ok := onStack[dep]; ok {
				return buildCycle(stack, dep), true
			}
			if _, ok := visited[dep]; ok {
				continue
			}
			if cycle, ok := dfs(dep); ok {
				return cycle, true
			}
		}

		stack = stack[:len(stack)-1]
		delete(onStack, node)
		return nil, false
	}

	for _, beadID := range r.addOrder {
		if _, ok := pending[beadID]; !ok {
			continue
		}
		if _, ok := visited[beadID]; ok {
			continue
		}
		if cycle, ok := dfs(beadID); ok {
			return cycle, true
		}
	}

	return nil, false
}

func buildCycle(stack []string, target string) []string {
	start := -1
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == target {
			start = i
			break
		}
	}
	if start == -1 {
		return []string{target, target}
	}

	cycle := make([]string, 0, len(stack)-start+1)
	cycle = append(cycle, stack[start:]...)
	cycle = append(cycle, target)
	return cycle
}
