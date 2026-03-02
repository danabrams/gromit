package specflow

import (
	"context"
	"errors"
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

func TestManagerAdvanceInvalidTransition(t *testing.T) {
	ctx := context.Background()
	store := &fakeSpecStore{stage: StagePlanning}
	mgr := NewManager(store)

	if err := mgr.Advance(ctx, "spec-2", StageReview); err == nil {
		t.Fatal("expected invalid transition error")
	} else if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}

	if store.stage != StagePlanning {
		t.Fatalf("store stage changed unexpectedly: %s", store.stage)
	}
	if store.storeCalls != 0 {
		t.Fatalf("advance should not persist invalid transition, storeCalls=%d", store.storeCalls)
	}
}

func TestManagerGuard(t *testing.T) {
	ctx := context.Background()
	store := &fakeSpecStore{stage: StageReview}
	mgr := NewManager(store)

	if err := mgr.Guard(ctx, "spec-3", StageReview); err != nil {
		t.Fatalf("unexpected guard failure: %v", err)
	}

	if err := mgr.Guard(ctx, "spec-3", StagePlanning); err == nil {
		t.Fatal("expected guard to report mismatch")
	} else if !errors.Is(err, ErrStageMismatch) {
		t.Fatalf("expected ErrStageMismatch, got %v", err)
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
