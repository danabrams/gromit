package specflow

import (
	"context"
	"os"
	"path/filepath"
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
