package extract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"main.go":               "package main",
		"internal/auth/auth.go": "package auth",
		"go.mod":                "module example.com/test",
		"README.md":             "# Test",
	}
	for path, content := range files {
		full := filepath.Join(dir, path)
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(content), 0o644)
	}
	return dir
}

func TestFileTreeExtractor_Extract(t *testing.T) {
	repo := setupFixtureRepo(t)
	ext := NewFileTreeExtractor()

	facts, err := ext.Extract(repo)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(facts) != 4 {
		t.Fatalf("expected 4 facts (one per file), got %d", len(facts))
	}

	// At least one fact should reference main.go
	found := false
	for _, f := range facts {
		if strings.Contains(f.Content, "main.go") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one fact with Content containing \"main.go\"")
	}

	// All facts should be observed
	for _, f := range facts {
		if f.Category.String() != "observed" {
			t.Errorf("fact %q has category %v, want observed", f.ID, f.Category)
		}
	}
}

func TestFileTreeExtractor_Name(t *testing.T) {
	ext := NewFileTreeExtractor()
	if ext.Name() != "file-tree" {
		t.Errorf("Name = %q, want %q", ext.Name(), "file-tree")
	}
}
