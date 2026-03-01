package specflow

import (
	"context"
	"errors"
	"fmt"
)

type Stage string

const (
	StagePlanning Stage = "planning"
	StageDrafting Stage = "drafting"
	StageReview   Stage = "review"
)

var ErrStageNotFound = errors.New("spec stage not found")
var ErrInvalidTransition = errors.New("invalid spec stage transition")
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
	stage, err := m.store.Stage(ctx, specID)
	if errors.Is(err, ErrStageNotFound) || stage == "" {
		return StagePlanning, nil
	}
	if err != nil {
		return "", err
	}
	return stage, nil
}

func (m *Manager) Advance(ctx context.Context, specID string, next Stage) error {
	current, err := m.Resume(ctx, specID)
	if err != nil {
		return err
	}

	expected, ok := stageTransitions[current]
	if !ok || expected != next {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current, next)
	}

	return m.store.StoreStage(ctx, specID, next)
}

func (m *Manager) Guard(ctx context.Context, specID string, expected Stage) error {
	stage, err := m.Resume(ctx, specID)
	if err != nil {
		return err
	}

	if stage != expected {
		return fmt.Errorf("%w: expected %s, got %s", ErrStageMismatch, expected, stage)
	}

	return nil
}

var stageTransitions = map[Stage]Stage{
	StagePlanning: StageDrafting,
	StageDrafting: StageReview,
}
