package specflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	store := newInstrumentedSpecStore(specsDir)
	proxy := NewManager(store)
	mirror := NewManager(store)

	start := make(chan struct{})
	ready := make(chan struct{}, 2)
	results := make(chan error, 2)

	run := func(m *Manager) {
		ready <- struct{}{}
		<-start
		results <- m.Advance(ctx, specID, StageAcceptanceTests)
	}

	go run(proxy)
	go run(mirror)

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

	stage, err := readStageFromFile(filepath.Join(specsDir, specID+".md"))
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

func readStageFromFile(path string) (Stage, error) {
	fm, _, err := frontmatter.ReadFile(path)
	if err != nil {
		return "", err
	}
	raw, ok := fm["stage"]
	if !ok {
		return "", ErrStageNotFound
	}
	value, ok := raw.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("invalid stage value: %v", raw)
	}
	return Stage(value), nil
}

func writeStageToFile(path string, stage Stage) error {
	updates := map[string]interface{}{"stage": string(stage)}
	return frontmatter.UpdateFile(path, updates)
}

type instrumentedSpecStore struct {
	specsDir    string
	stageCalled chan struct{}
	allowStore  chan struct{}
}

func newInstrumentedSpecStore(specsDir string) *instrumentedSpecStore {
	return &instrumentedSpecStore{
		specsDir:    specsDir,
		stageCalled: make(chan struct{}, 2),
		allowStore:  make(chan struct{}),
	}
}

func (s *instrumentedSpecStore) Stage(ctx context.Context, specID string) (Stage, error) {
	s.stageCalled <- struct{}{}
	return readStageFromFile(filepath.Join(s.specsDir, specID+".md"))
}

func (s *instrumentedSpecStore) StoreStage(ctx context.Context, specID string, stage Stage) error {
	<-s.allowStore
	return writeStageToFile(filepath.Join(s.specsDir, specID+".md"), stage)
}
