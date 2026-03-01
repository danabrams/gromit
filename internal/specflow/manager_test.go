package specflow

import (
    "context"
    "testing"
)

func TestManagerResumeIdempotent(t *testing.T) {
    ctx := context.Background()
    store := &fakeSpecStore{stageErr: ErrStageNotFound}
    mgr := NewManager(store)

    stage, err := mgr.Resume(ctx, "spec-1")
    if err != nil {
        t.Fatalf("resume failed: %v", err)
    }
    if stage != StagePlanning {
        t.Fatalf("expected planning, got %s", stage)
    }

    stage, err = mgr.Resume(ctx, "spec-1")
    if err != nil {
        t.Fatalf("second resume failed: %v", err)
    }
    if stage != StagePlanning {
        t.Fatalf("second resume expected planning, got %s", stage)
    }

    if store.storeCalls != 0 {
        t.Fatalf("resume should not have persisted stage, storeCalls=%d", store.storeCalls)
    }
}

type fakeSpecStore struct {
    stage      Stage
    stageErr   error
    storeErr   error
    storeCalls int
}

func (f *fakeSpecStore) Stage(_ context.Context, _ string) (Stage, error) {
    if f.stageErr != nil {
        return "", f.stageErr
    }
    return f.stage, nil
}

func (f *fakeSpecStore) StoreStage(_ context.Context, _ string, stage Stage) error {
    f.storeCalls++
    if f.storeErr != nil {
        return f.storeErr
    }
    f.stage = stage
    return nil
}
