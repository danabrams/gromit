package specflow

import (
    "context"
    "errors"
)

type Stage string

const StagePlanning Stage = "planning"

var ErrStageNotFound = errors.New("spec stage not found")

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
