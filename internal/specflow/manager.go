package specflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrStageNotFound = errors.New("spec stage not found")
var ErrStageMismatch = errors.New("spec stage mismatch")

type SpecStore interface {
	Stage(context.Context, string) (Stage, error)
	StoreStage(context.Context, string, Stage) error
}

type Manager struct {
	store SpecStore
	mu    sync.Mutex
}

var specLocks = newSpecLockStore()

func NewManager(store SpecStore) *Manager {
	return &Manager{store: store}
}

func (m *Manager) Resume(ctx context.Context, specID string) (Stage, error) {
	stage, _, err := m.ResumeWithBootstrap(ctx, specID)
	return stage, err
}

// ResumeWithBootstrap returns the current stage and whether the stage was bootstrapped
// (i.e., no previous stage data existed for the spec).
func (m *Manager) ResumeWithBootstrap(ctx context.Context, specID string) (Stage, bool, error) {
	stage, err := m.store.Stage(ctx, specID)
	if errors.Is(err, ErrStageNotFound) || stage == "" {
		return StagePlanning, true, nil
	}
	if err != nil {
		return "", false, err
	}
	return stage, false, nil
}

func (m *Manager) Advance(ctx context.Context, specID string, next Stage) error {
	unlock := specLocks.Lock(specID)
	defer unlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	current, _, err := m.ResumeWithBootstrap(ctx, specID)
	if err != nil {
		return err
	}

	if err := ValidateTransition(current, next); err != nil {
		return err
	}

	return m.store.StoreStage(ctx, specID, next)
}

func (m *Manager) Guard(ctx context.Context, specID string, expected Stage) error {
	stage, _, err := m.ResumeWithBootstrap(ctx, specID)
	if err != nil {
		return err
	}

	if stage != expected {
		return fmt.Errorf("%w: expected %s, got %s", ErrStageMismatch, expected, stage)
	}

	return nil
}

type specLockEntry struct {
	mu   sync.Mutex
	refs int
}

type specLockStore struct {
	mu    sync.Mutex
	locks map[string]*specLockEntry
}

func newSpecLockStore() *specLockStore {
	return &specLockStore{
		locks: make(map[string]*specLockEntry),
	}
}

func (s *specLockStore) Lock(specID string) func() {
	s.mu.Lock()
	entry, ok := s.locks[specID]
	if !ok {
		entry = &specLockEntry{}
		s.locks[specID] = entry
	}
	entry.refs++
	s.mu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		s.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(s.locks, specID)
		}
		s.mu.Unlock()
	}
}
