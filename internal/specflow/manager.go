package specflow

import (
	"context"
	"errors"
	"fmt"
)

var ErrStageNotFound = errors.New("spec stage not found")
var ErrStageMismatch = errors.New("spec stage mismatch")

type SpecStore interface {
	Stage(context.Context, string) (Stage, error)
	StoreStage(context.Context, string, Stage) error
}

type Manager struct {
	store SpecStore
}

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
