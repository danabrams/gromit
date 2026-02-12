package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListMarkdownFiles_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	files, err := ListMarkdownFiles(dir)
	if err != nil {
		t.Fatalf("ListMarkdownFiles failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected no files, got %d", len(files))
	}
}

func TestListMarkdownFiles_WithMarkdownFiles(t *testing.T) {
	dir := t.TempDir()

	// Create markdown files
	mdFile1 := filepath.Join(dir, "spec1.md")
	mdFile2 := filepath.Join(dir, "spec2.md")
	txtFile := filepath.Join(dir, "readme.txt")

	if err := os.WriteFile(mdFile1, []byte("# Spec 1"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(mdFile2, []byte("# Spec 2"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(txtFile, []byte("not markdown"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	files, err := ListMarkdownFiles(dir)
	if err != nil {
		t.Fatalf("ListMarkdownFiles failed: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 markdown files, got %d", len(files))
	}

	// Check that files are returned with full paths
	expectedFiles := map[string]bool{
		mdFile1: false,
		mdFile2: false,
	}
	for _, f := range files {
		if _, ok := expectedFiles[f]; ok {
			expectedFiles[f] = true
		}
	}

	for file, found := range expectedFiles {
		if !found {
			t.Errorf("expected file not found in results: %s", file)
		}
	}
}

func TestDiffFiles_NoChanges(t *testing.T) {
	before := []string{"/path/to/spec1.md", "/path/to/spec2.md"}
	after := []string{"/path/to/spec1.md", "/path/to/spec2.md"}

	diff := DiffFiles(before, after)

	if len(diff) != 0 {
		t.Errorf("expected no differences, got %d: %v", len(diff), diff)
	}
}
