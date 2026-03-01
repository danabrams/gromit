package specflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/frontmatter"
)

func TestSpecFrontmatterStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	workDir := t.TempDir()
	gromitDir := filepath.Join(workDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	specName := "round-trip"
	specFile := filepath.Join(specsDir, specName+".md")
	original := `---
id: round-trip
foo: bar
---
# Title
`
	if err := os.WriteFile(specFile, []byte(original), 0o644); err != nil {
		t.Fatalf("failed to write spec fixture: %v", err)
	}

	store, err := NewSpecFrontmatterStore(gromitDir)
	if err != nil {
		t.Fatalf("failed to build store: %v", err)
	}

	if err := store.StoreStage(ctx, specName, StageImplementation); err != nil {
		t.Fatalf("store stage: %v", err)
	}

	stage, err := store.Stage(ctx, specName)
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	if stage != StageImplementation {
		t.Fatalf("unexpected stage after store: %s", stage)
	}

	fm, _, err := frontmatter.ReadFile(specFile)
	if err != nil {
		t.Fatalf("read spec frontmatter: %v", err)
	}
	if fm["id"] != specName {
		t.Fatalf("id field was modified, want %s got %v", specName, fm["id"])
	}
	if fm["foo"] != "bar" {
		t.Fatalf("custom field was modified, want bar got %v", fm["foo"])
	}
	if fm["stage"] != string(StageImplementation) {
		t.Fatalf("stage not written correctly, got %v", fm["stage"])
	}
}

func TestSpecFrontmatterStoreMalformedFrontmatter(t *testing.T) {
	ctx := context.Background()
	workDir := t.TempDir()
	gromitDir := filepath.Join(workDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	specName := "malformed"
	specFile := filepath.Join(specsDir, specName+".md")
	manifest := "---\nid: malformed\n"
	if err := os.WriteFile(specFile, []byte(manifest), 0o644); err != nil {
		t.Fatalf("failed to write malformed spec: %v", err)
	}

	store, err := NewSpecFrontmatterStore(gromitDir)
	if err != nil {
		t.Fatalf("failed to build store: %v", err)
	}

	if _, err := store.Stage(ctx, specName); err == nil || !errors.Is(err, ErrMalformedSpecFrontmatter) {
		t.Fatalf("expected malformed frontmatter error, got %v", err)
	}

	if err := store.StoreStage(ctx, specName, StageReview); err == nil || !errors.Is(err, ErrMalformedSpecFrontmatter) {
		t.Fatalf("expected malformed frontmatter error on store, got %v", err)
	}
}
