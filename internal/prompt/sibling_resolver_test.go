package prompt

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestBuildContext_UsesSiblingTouchedPackagesResolver(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	specsDir := filepath.Join(tmpDir, ".gromit", "specs")
	gromitDir := filepath.Join(tmpDir, ".gromit")

	r, err := NewRenderer(templatesDir, specsDir, filepath.Join(tmpDir, "CLAUDE.md"), gromitDir)
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}

	called := false
	r.SetSiblingTouchedPackagesResolver(func(current *bead.Bead, parent *bead.Bead) ([]string, error) {
		called = true
		if current == nil {
			t.Fatal("resolver current bead should not be nil")
		}
		return []string{"internal/logger", "internal/prompt"}, nil
	})

	ctx, err := r.BuildContext(&bead.Bead{
		ID:              "test-1",
		Title:           "test",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}, nil, 1, "sonnet")
	if err != nil {
		t.Fatalf("BuildContext() error = %v", err)
	}
	if !called {
		t.Fatal("expected sibling resolver to be called")
	}
	if len(ctx.SiblingTouchedPackages) != 2 {
		t.Fatalf("SiblingTouchedPackages length = %d, want 2", len(ctx.SiblingTouchedPackages))
	}
	if ctx.SiblingTouchedPackages[0] != "internal/logger" || ctx.SiblingTouchedPackages[1] != "internal/prompt" {
		t.Fatalf("SiblingTouchedPackages = %v, want [internal/logger internal/prompt]", ctx.SiblingTouchedPackages)
	}
}

func TestBuildContext_SiblingResolverErrorDoesNotFail(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".gromit", "templates")
	specsDir := filepath.Join(tmpDir, ".gromit", "specs")
	gromitDir := filepath.Join(tmpDir, ".gromit")

	r, err := NewRenderer(templatesDir, specsDir, filepath.Join(tmpDir, "CLAUDE.md"), gromitDir)
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}

	r.SetSiblingTouchedPackagesResolver(func(current *bead.Bead, parent *bead.Bead) ([]string, error) {
		return nil, fmt.Errorf("boom")
	})

	ctx, err := r.BuildContext(&bead.Bead{
		ID:              "test-2",
		Title:           "test",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}, nil, 1, "sonnet")
	if err != nil {
		t.Fatalf("BuildContext() should degrade on resolver error, got %v", err)
	}
	if ctx == nil {
		t.Fatal("BuildContext() returned nil context")
	}
	if ctx.SiblingTouchedPackages == nil {
		t.Fatal("SiblingTouchedPackages should be normalized to empty slice")
	}
	if len(ctx.SiblingTouchedPackages) != 0 {
		t.Fatalf("SiblingTouchedPackages = %v, want empty", ctx.SiblingTouchedPackages)
	}
}
