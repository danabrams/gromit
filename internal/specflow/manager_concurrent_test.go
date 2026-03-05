package specflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/frontmatter"
)

func TestManagerAdvanceConcurrentSafety(t *testing.T) {
	ctx := context.Background()
	specID := "concurrent-spec"

	gromitDir := filepath.Join(t.TempDir(), "gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	if err := writeSpecStage(t, specsDir, specID, StagePlanning); err != nil {
		t.Fatalf("failed to seed spec stage: %v", err)
	}

	delegate, err := NewSpecFrontmatterStore(gromitDir)
	if err != nil {
		t.Fatalf("failed to build spec store: %v", err)
	}
	store := newInstrumentedSpecStore(delegate)

	start := make(chan struct{})
	ready := make(chan struct{}, 2)
	results := make(chan error, 2)

	run := func(m *Manager) {
		ready <- struct{}{}
		<-start
		results <- m.Advance(ctx, specID, StageAcceptanceTests)
	}

	go run(NewManager(store))
	go run(NewManager(store))

	for i := 0; i < 2; i++ {
		<-ready
	}
	close(start)

	stageCount := waitForStageCalls(t, store.stageCalled, 2, 50*time.Millisecond)
	close(store.allowStore)

	errs := []error{<-results, <-results}
	if stageCount >= 2 {
		t.Fatalf("concurrent Advance detected: stage calls=%d, results=%v", stageCount, errs)
	}

	successCount, invalidCount := countAdvanceResults(errs)
	if successCount != 1 || invalidCount != 1 {
		t.Fatalf("expected one success and one ErrInvalidTransition, got success=%d invalid=%d", successCount, invalidCount)
	}

	stage, err := delegate.Stage(ctx, specID)
	if err != nil {
		t.Fatalf("failed to read stage: %v", err)
	}
	if stage != StageAcceptanceTests {
		t.Fatalf("expected stage %s, got %s", StageAcceptanceTests, stage)
	}
}

func TestManagerAdvanceConcurrentStoreStage(t *testing.T) {
	ctx := context.Background()
	specID := "store-stage-concurrent"

	gromitDir := filepath.Join(t.TempDir(), "gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	if err := writeSpecStage(t, specsDir, specID, StagePlanning); err != nil {
		t.Fatalf("failed to seed spec stage: %v", err)
	}

	delegate, err := NewSpecFrontmatterStore(gromitDir)
	if err != nil {
		t.Fatalf("failed to build spec store: %v", err)
	}
	store := newStoreBlockingSpecStore(delegate)

	start := make(chan struct{})
	ready := make(chan struct{}, 2)
	results := make(chan error, 2)

	run := func(m *Manager) {
		ready <- struct{}{}
		<-start
		results <- m.Advance(ctx, specID, StageAcceptanceTests)
	}

	go run(NewManager(store))
	go run(NewManager(store))

	for i := 0; i < 2; i++ {
		<-ready
	}
	close(start)

	select {
	case <-store.storeCalled:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("StoreStage never started")
	}

	select {
	case <-store.storeCalled:
		t.Fatal("second StoreStage entered before first completed")
	case <-time.After(10 * time.Millisecond):
	}

	close(store.allowStore)

	errs := []error{<-results, <-results}
	successCount, invalidCount := countAdvanceResults(errs)
	if successCount != 1 || invalidCount != 1 {
		t.Fatalf("expected one success and one ErrInvalidTransition, got success=%d invalid=%d", successCount, invalidCount)
	}

	stage, err := delegate.Stage(ctx, specID)
	if err != nil {
		t.Fatalf("failed to read stage: %v", err)
	}
	if stage != StageAcceptanceTests {
		t.Fatalf("expected stage %s, got %s", StageAcceptanceTests, stage)
	}
}

func waitForStageCalls(t *testing.T, ch <-chan struct{}, limit int, timeout time.Duration) int {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	count := 0
	for count < limit {
		select {
		case <-ch:
			count++
		case <-timer.C:
			return count
		}
	}
	return count
}

func countAdvanceResults(errs []error) (int, int) {
	successCount := 0
	invalidCount := 0
	for _, err := range errs {
		switch {
		case err == nil:
			successCount++
		case errors.Is(err, ErrInvalidTransition):
			invalidCount++
		}
	}
	return successCount, invalidCount
}

func writeSpecStage(t *testing.T, specsDir, specID string, stage Stage) error {
	t.Helper()
	specPath := filepath.Join(specsDir, specID+".md")
	content, err := frontmatter.Serialize(map[string]interface{}{"stage": string(stage)}, "# spec")
	if err != nil {
		return err
	}
	return os.WriteFile(specPath, []byte(content), 0o644)
}

type instrumentedSpecStore struct {
	delegate    SpecStore
	stageCalled chan struct{}
	allowStore  chan struct{}
}

func newInstrumentedSpecStore(delegate SpecStore) *instrumentedSpecStore {
	return &instrumentedSpecStore{
		delegate:    delegate,
		stageCalled: make(chan struct{}, 2),
		allowStore:  make(chan struct{}),
	}
}

func (s *instrumentedSpecStore) Stage(ctx context.Context, specID string) (Stage, error) {
	s.stageCalled <- struct{}{}
	return s.delegate.Stage(ctx, specID)
}

func (s *instrumentedSpecStore) StoreStage(ctx context.Context, specID string, stage Stage) error {
	<-s.allowStore
	return s.delegate.StoreStage(ctx, specID, stage)
}

type storeBlockingSpecStore struct {
	delegate    SpecStore
	storeCalled chan struct{}
	allowStore  chan struct{}
}

func newStoreBlockingSpecStore(delegate SpecStore) *storeBlockingSpecStore {
	return &storeBlockingSpecStore{
		delegate:    delegate,
		storeCalled: make(chan struct{}, 2),
		allowStore:  make(chan struct{}),
	}
}

func (s *storeBlockingSpecStore) Stage(ctx context.Context, specID string) (Stage, error) {
	return s.delegate.Stage(ctx, specID)
}

func (s *storeBlockingSpecStore) StoreStage(ctx context.Context, specID string, stage Stage) error {
	select {
	case s.storeCalled <- struct{}{}:
	default:
	}
	<-s.allowStore
	return s.delegate.StoreStage(ctx, specID, stage)
}
