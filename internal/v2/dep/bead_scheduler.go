package dep

import (
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/queue"
)

// BeadScheduler selects beads whose dependencies are satisfied before they run.
type BeadScheduler struct {
	beads     []*bead.Bead
	completed map[string]struct{}
}

// NewBeadScheduler returns a scheduler seeded with the provided bead list.
func NewBeadScheduler(beads []*bead.Bead) *BeadScheduler {
	copyBeads := append([]*bead.Bead(nil), beads...)
	return &BeadScheduler{
		beads:     copyBeads,
		completed: map[string]struct{}{},
	}
}

// Next returns the next bead without unmet dependencies or nil when none are ready.
func (s *BeadScheduler) Next() *bead.Bead {
	for _, b := range s.beads {
		if b == nil {
			continue
		}
		if s.isCompleted(b.ID) {
			continue
		}
		if s.hasUnresolvedDependencies(b) {
			continue
		}
		return b
	}
	return nil
}

// MarkComplete records that a bead has finished, unlocking dependents that rely on it.
func (s *BeadScheduler) MarkComplete(id string) {
	if id == "" {
		return
	}
	s.completed[id] = struct{}{}
}

func (s *BeadScheduler) isCompleted(id string) bool {
	if id == "" {
		return false
	}
	_, ok := s.completed[id]
	return ok
}

func (s *BeadScheduler) hasUnresolvedDependencies(b *bead.Bead) bool {
	deps := queue.DependencyIDs(b.DependsOn)
	deps = append(deps, queue.DependencyIDs(b.Dependencies)...)
	deps = append(deps, queue.DependencyIDs(b.BlockedBy)...)

	for _, dep := range deps {
		if dep == "" {
			continue
		}
		if !s.isCompleted(dep) {
			return true
		}
	}
	return false
}
