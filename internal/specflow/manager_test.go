package specflow

import (
	"context"
	"errors"
	"testing"
	"time"
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

func TestManagerAdvanceConcurrentTOCTOU(t *testing.T) {
	ctx := context.Background()
	store := newBlockingSpecStore(StagePlanning)
	mgr := NewManager(store)
	specID := "spec-advance"

	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)

	go func() {
		firstDone <- mgr.Advance(ctx, specID, StageAcceptanceTests)
	}()

	<-store.stageCalled
	<-store.storeReached

	go func() {
		secondDone <- mgr.Advance(ctx, specID, StageAcceptanceTests)
	}()

	select {
	case <-store.stageCalled:
	case <-time.After(50 * time.Millisecond):
	}

	close(store.releaseStore)

	if err := <-firstDone; err != nil {
		t.Fatalf("first advance failed: %v", err)
	}
	if err := <-secondDone; !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("second advance expected ErrInvalidTransition, got %v", err)
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

type blockingSpecStore struct {
	*fakeSpecStore
	stageCalled  chan struct{}
	storeReached chan struct{}
	releaseStore chan struct{}
}

func newBlockingSpecStore(initial Stage) *blockingSpecStore {
	return &blockingSpecStore{
		fakeSpecStore: &fakeSpecStore{stage: initial},
		stageCalled:   make(chan struct{}, 2),
		storeReached:  make(chan struct{}, 1),
		releaseStore:  make(chan struct{}),
	}
}

func (b *blockingSpecStore) Stage(ctx context.Context, specID string) (Stage, error) {
	select {
	case b.stageCalled <- struct{}{}:
	default:
	}
	return b.fakeSpecStore.Stage(ctx, specID)
}

func (b *blockingSpecStore) StoreStage(ctx context.Context, specID string, stage Stage) error {
	select {
	case b.storeReached <- struct{}{}:
	default:
	}
	<-b.releaseStore
	return b.fakeSpecStore.StoreStage(ctx, specID, stage)
}

func (f *fakeSpecStore) StoreStage(_ context.Context, _ string, stage Stage) error {
	f.storeCalls++
	if f.storeErr != nil {
		return f.storeErr
	}
	f.stage = stage
	return nil
}
