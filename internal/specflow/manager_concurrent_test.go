package specflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

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

	specPath := filepath.Join(specsDir, specID+".md")
	frontMatter, err := frontmatter.Serialize(map[string]interface{}{"stage": string(StagePlanning)}, "# concurrent spec")
	if err != nil {
		t.Fatalf("failed to serialize frontmatter: %v", err)
	}
	if err := os.WriteFile(specPath, []byte(frontMatter), 0o644); err != nil {
		t.Fatalf("failed to write spec file: %v", err)
	}

	store, err := NewSpecFrontmatterStore(gromitDir)
	if err != nil {
		t.Fatalf("failed to create spec store: %v", err)
	}

	firstMgr := NewManager(store)
	secondMgr := NewManager(store)

	start := make(chan struct{})
	ready := make(chan struct{}, 2)
	firstErr := make(chan error, 1)
	secondErr := make(chan error, 1)

	go func() {
		ready <- struct{}{}
		<-start
		firstErr <- firstMgr.Advance(ctx, specID, StageAcceptanceTests)
	}()

	go func() {
		ready <- struct{}{}
		<-start
		secondErr <- secondMgr.Advance(ctx, specID, StageAcceptanceTests)
	}()

	for i := 0; i < 2; i++ {
		<-ready
	}
	close(start)

	errs := []error{<-firstErr, <-secondErr}
	successCount := 0
	invalidCount := 0
	for _, err := range errs {
		switch {
		case err == nil:
			successCount++
		case errors.Is(err, ErrInvalidTransition):
			invalidCount++
		default:
			t.Fatalf("unexpected error from advance: %v", err)
		}
	}
	if successCount != 1 || invalidCount != 1 {
		t.Fatalf("expected one successful advance and one ErrInvalidTransition, got success=%d invalid=%d", successCount, invalidCount)
	}
}
