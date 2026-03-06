package testutil

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
)

// FakeTaskTracker keeps beads in memory and tracks dependency relationships.
type FakeTaskTracker struct {
	mu          sync.Mutex
	beads       map[string]*tasktracker.Bead
	order       []string
	nextID      int
}

// NewFakeTaskTracker constructs a tracker seeded with no beads.
func NewFakeTaskTracker() *FakeTaskTracker {
	return &FakeTaskTracker{beads: make(map[string]*tasktracker.Bead)}
}

// NextBead returns the earliest open bead whose dependencies are cleared.
func (f *FakeTaskTracker) NextBead(_ context.Context, req tasktracker.NextBeadRequest) (*tasktracker.NextBeadResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, id := range f.order {
		bead := f.beads[id]
		if bead == nil || isClosed(bead.Status) {
			continue
		}
		if f.hasPendingDependencies(bead) {
			continue
		}
		return &tasktracker.NextBeadResponse{Bead: cloneBead(bead)}, nil
	}
	return nil, nil
}

// ShowBead returns the bead matching the provided ID or nil if not found.
func (f *FakeTaskTracker) ShowBead(_ context.Context, beadID string) (*tasktracker.Bead, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if bead, ok := f.beads[beadID]; ok {
		return cloneBead(bead), nil
	}
	return nil, nil
}

// CreateBead appends a new bead and wires up dependency links.
func (f *FakeTaskTracker) CreateBead(_ context.Context, req tasktracker.CreateBeadRequest) (*tasktracker.CreateBeadResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nextID++
	id := fmt.Sprintf("bead-%d", f.nextID)
	bead := &tasktracker.Bead{
		ID:          id,
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		Labels:      cloneStrings(req.Labels),
		DependsOn:   cloneStrings(req.Dependencies),
		BlockedBy:   cloneStrings(req.Dependencies),
		Status:      tracker.StatusOpen,
	}
	f.beads[id] = bead
	f.order = append(f.order, id)

	for _, depID := range req.Dependencies {
		if dep, ok := f.beads[depID]; ok {
			dep.Dependents = appendIfMissing(dep.Dependents, id)
		}
	}

	return &tasktracker.CreateBeadResponse{Bead: cloneBead(bead)}, nil
}

// CloseBead marks a bead closed but leaves other metadata intact.
func (f *FakeTaskTracker) CloseBead(_ context.Context, req tasktracker.CloseBeadRequest) (*tasktracker.CloseBeadResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	closed := false
	if bead, ok := f.beads[req.BeadID]; ok {
		bead.Status = tracker.StatusClosed
		closed = true
	}
	return &tasktracker.CloseBeadResponse{Closed: closed}, nil
}

// QueryBeads filters beads by labels and status.
func (f *FakeTaskTracker) QueryBeads(_ context.Context, req tasktracker.QueryBeadsRequest) (*tasktracker.QueryBeadsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var filtered []tasktracker.Bead
	for _, id := range f.order {
		bead := f.beads[id]
		if bead == nil {
			continue
		}
		if req.Status != "" && !strings.EqualFold(strings.TrimSpace(req.Status), strings.TrimSpace(bead.Status)) {
			continue
		}
		if !containsAllLabels(bead.Labels, req.Labels) {
			continue
		}
		filtered = append(filtered, *cloneBead(bead))
	}
	return &tasktracker.QueryBeadsResponse{Beads: filtered}, nil
}

func (f *FakeTaskTracker) hasPendingDependencies(bead *tasktracker.Bead) bool {
	for _, depID := range mergeDependencies(bead) {
		if dep, ok := f.beads[depID]; !ok || !isClosed(dep.Status) {
			return true
		}
	}
	return false
}

func isClosed(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), tracker.StatusClosed)
}

func mergeDependencies(bead *tasktracker.Bead) []string {
	deps := append([]string{}, bead.BlockedBy...)
	deps = append(deps, bead.DependsOn...)
	return deps
}

func appendIfMissing(list []string, value string) []string {
	for _, v := range list {
		if v == value {
			return list
		}
	}
	return append(list, value)
}

func containsAllLabels(source, target []string) bool {
	if len(target) == 0 {
		return true
	}
	for _, want := range target {
		found := false
		for _, have := range source {
			if have == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func cloneStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func cloneBead(b *tasktracker.Bead) *tasktracker.Bead {
	if b == nil {
		return nil
	}
	return &tasktracker.Bead{
		ID:          b.ID,
		Title:       b.Title,
		Description: b.Description,
		Priority:    b.Priority,
		Labels:      cloneStrings(b.Labels),
		Status:      b.Status,
		DependsOn:   cloneStrings(b.DependsOn),
		BlockedBy:   cloneStrings(b.BlockedBy),
		Dependents:  cloneStrings(b.Dependents),
	}
}
