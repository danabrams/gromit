package specflow

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	origWrite := store.writeFile
	store.writeFile = func(name string, data []byte, perm os.FileMode) error {
		if atomic.AddInt32(&writeCalls, 1) == 1 {
			close(saveStarted)
			<-allowFirstSave
		}
		return origWrite(name, data, perm)
	}
	t.Cleanup(func() { store.writeFile = origWrite })

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
		path:      filepath.Join(t.TempDir(), "specflow.json"),
		stages:    make(map[string]Stage),
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
	}
}

func TestFileStoreUsesInjectedWriter(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	store := newTestFileStore(t)
	writeCalled := make(chan struct{}, 1)
	store.writeFile = func(name string, data []byte, perm os.FileMode) error {
		select {
		case writeCalled <- struct{}{}:
		default:
		}
		return nil
	}
	if err := store.StoreStage(ctx, "spec-1", StagePlanning); err != nil {
		t.Fatalf("StoreStage failed: %v", err)
	}
	select {
	case <-writeCalled:
	default:
		t.Fatalf("no writeFile call detected")
	}
}

func TestFileStoreLoadUsesInjectedReader(t *testing.T) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "specflow.json")
	store := &fileStore{
		path:   path,
		stages: make(map[string]Stage),
	}

	var readCalls int32
	store.readFile = func(p string) ([]byte, error) {
		if p != path {
			t.Fatalf("unexpected path: %s", p)
		}
		atomic.AddInt32(&readCalls, 1)
		return []byte(`{"spec-1": "` + string(StageImplementation) + `"}`), nil
	}

	if err := store.load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if atomic.LoadInt32(&readCalls) == 0 {
		t.Fatalf("injected reader was not used")
	}

	ctx := context.Background()
	stage, err := store.Stage(ctx, "spec-1")
	if err != nil {
		t.Fatalf("expected stage to be loaded, got %v", err)
	}
	if stage != StageImplementation {
		t.Fatalf("stage = %s, want %s", stage, StageImplementation)
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

func TestFileStoreSaveMethodHasNoCallers(t *testing.T) {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read specflow directory: %v", err)
	}

	var matches []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}

		scanner := bufio.NewScanner(bytes.NewReader(data))
		line := 0
		for scanner.Scan() {
			line++
			text := scanner.Text()
			if strings.Contains(text, "save()") {
				matches = append(matches, fmt.Sprintf("%s:%d: %s", entry.Name(), line, strings.TrimSpace(text)))
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatalf("scan %s: %v", entry.Name(), err)
		}
	}

	if len(matches) > 0 {
		t.Fatalf("found unexpected save() occurrences:\n%s", strings.Join(matches, "\n"))
	}
}
