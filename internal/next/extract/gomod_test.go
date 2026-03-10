package extract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoModExtractor_Extract(t *testing.T) {
	dir := t.TempDir()
	gomod := `module github.com/example/myapp

go 1.22

require (
	github.com/spf13/cobra v1.8.0
	github.com/stretchr/testify v1.9.0
)
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644)

	ext := NewGoModExtractor()
	facts, err := ext.Extract(dir)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(facts) != 4 {
		t.Fatalf("expected 4 facts (module path, go version, 2 deps), got %d", len(facts))
	}

	// Verify one fact contains the module path
	found := false
	for _, f := range facts {
		if strings.Contains(f.Content, "github.com/example/myapp") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one fact with Content containing \"github.com/example/myapp\"")
	}

	for _, f := range facts {
		if f.Category.String() != "observed" {
			t.Errorf("fact %q has category %v, want observed", f.ID, f.Category)
		}
		if f.Source != "go-module" {
			t.Errorf("fact %q has source %q, want %q", f.ID, f.Source, "go-module")
		}
	}
}

func TestGoModExtractor_NoGoMod(t *testing.T) {
	dir := t.TempDir()
	ext := NewGoModExtractor()

	facts, err := ext.Extract(dir)
	if err != nil {
		t.Fatalf("Extract should not error on missing go.mod: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("expected 0 facts for missing go.mod, got %d", len(facts))
	}
}

func TestGoModExtractor_Name(t *testing.T) {
	ext := NewGoModExtractor()
	if ext.Name() != "go-module" {
		t.Errorf("Name = %q, want %q", ext.Name(), "go-module")
	}
}
