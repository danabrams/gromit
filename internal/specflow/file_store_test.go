package specflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileStoreStoreStageBlocksWhileSaving(t *testing.T) {
	t.Helper()

	store := newTestFileStore(t)

	saveStarted := make(chan struct{})
	allowFirstSave := make(chan struct{})
	var writeCalls int32

	origWrite := writeFileFunc
	writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
		if atomic.AddInt32(&writeCalls, 1) == 1 {
			close(saveStarted)
			<-allowFirstSave
		}
		return origWrite(name, data, perm)
	}
	t.Cleanup(func() { writeFileFunc = origWrite })

	ctx := context.Background()

	go func() {
		if err := store.StoreStage(ctx, "spec-1", StagePlanning); err != nil {
			t.Errorf("StoreStage first call failed: %v", err)
		}
	}()

	<-saveStarted

	secondDone := make(chan struct{})
	go func() {
		if err := store.StoreStage(ctx, "spec-1", StageReview); err != nil {
			t.Errorf("StoreStage second call failed: %v", err)
		}
		close(secondDone)
	}()

	select {
	case <-secondDone:
		t.Fatalf("second StoreStage returned while first save still held lock")
	case <-time.After(50 * time.Millisecond):
	}

	close(allowFirstSave)

	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatalf("second StoreStage did not finish after first save released lock")
	}
}

func newTestFileStore(t *testing.T) *fileStore {
	t.Helper()
	return &fileStore{
		path:   filepath.Join(t.TempDir(), "specflow.json"),
		stages: make(map[string]Stage),
	}
}

func TestFileStoreStoreStageRace(t *testing.T) {
	ctx := context.Background()
	store := newTestFileStore(t)
	runConcurrentStoreStageRace(t, ctx, store, 100)
}

func runConcurrentStoreStageRace(t *testing.T, ctx context.Context, store SpecStore, operations int) {
	t.Helper()

	const specIDCount = 3
	specIDs := make([]string, specIDCount)
	for i := range specIDs {
		specIDs[i] = fmt.Sprintf("race-%d", i+1)
	}

	stages := []Stage{
		StagePlanning,
		StageAcceptanceTests,
		StageImplementation,
		StageReview,
		StageGlobalGate,
		StageDone,
	}

	errCh := make(chan error, operations)
	var wg sync.WaitGroup

	for i := 0; i < operations; i++ {
		wg.Add(1)
		specID := specIDs[i%len(specIDs)]
		stage := stages[i%len(stages)]
		go func(specID string, stage Stage) {
			defer wg.Done()
			if err := store.StoreStage(ctx, specID, stage); err != nil {
				errCh <- fmt.Errorf("store stage %s for %s: %w", stage, specID, err)
			}
		}(specID, stage)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent store stage error: %v", err)
	}

	for _, specID := range specIDs {
		stage, err := store.Stage(ctx, specID)
		if err != nil {
			t.Fatalf("load stage for %s failed: %v", specID, err)
		}
		if stage == "" {
			t.Fatalf("expected stage for %s to be set", specID)
		}
	}
}
