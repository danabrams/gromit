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

func TestDiffFiles_WithNewFiles(t *testing.T) {
	before := []string{"/path/to/spec1.md"}
	after := []string{"/path/to/spec1.md", "/path/to/spec2.md", "/path/to/spec3.md"}

	diff := DiffFiles(before, after)

	if len(diff) != 2 {
		t.Fatalf("expected 2 new files, got %d", len(diff))
	}

	expected := map[string]bool{
		"/path/to/spec2.md": false,
		"/path/to/spec3.md": false,
	}
	for _, f := range diff {
		if _, ok := expected[f]; ok {
			expected[f] = true
		}
	}

	for file, found := range expected {
		if !found {
			t.Errorf("expected new file not found: %s", file)
		}
	}
}

func TestExtractSpecTitle_SimpleHeading(t *testing.T) {
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.md")

	content := `# Pipeline Extraction

This is a spec about pipeline extraction.`

	if err := os.WriteFile(specFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	title := ExtractSpecTitle(specFile)

	expected := "Pipeline Extraction"
	if title != expected {
		t.Errorf("expected title %q, got %q", expected, title)
	}
}

func TestExtractSpecTitle_WithFrontmatter(t *testing.T) {
	dir := t.TempDir()
	specFile := filepath.Join(dir, "spec.md")

	content := `---
id: pipeline-extraction
created: 2026-02-11
---

# Pipeline Extraction

This spec has frontmatter.`

	if err := os.WriteFile(specFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	title := ExtractSpecTitle(specFile)

	expected := "Pipeline Extraction"
	if title != expected {
		t.Errorf("expected title %q, got %q", expected, title)
	}
}

func TestWriteTempPrompt_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	prompt := "This is a test prompt"

	path, cleanup, err := WriteTempPrompt(tmpDir, prompt)
	if err != nil {
		t.Fatalf("WriteTempPrompt failed: %v", err)
	}
	defer cleanup()

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("temp file was not created: %s", path)
	}

	// Verify content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read temp file: %v", err)
	}

	if string(content) != prompt {
		t.Errorf("expected content %q, got %q", prompt, string(content))
	}
}

func TestWriteTempPrompt_CleanupRemovesFile(t *testing.T) {
	tmpDir := t.TempDir()
	prompt := "This is a test prompt"

	path, cleanup, err := WriteTempPrompt(tmpDir, prompt)
	if err != nil {
		t.Fatalf("WriteTempPrompt failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("temp file was not created: %s", path)
	}

	// Call cleanup
	cleanup()

	// Verify file was removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("temp file was not removed after cleanup: %s", path)
	}
}

func TestWriteTempPrompt_ReturnsAbsolutePathForRelativeTmpDir(t *testing.T) {
	baseDir := t.TempDir()
	t.Chdir(baseDir)

	path, cleanup, err := WriteTempPrompt(filepath.Join(".gromit", "tmp"), "prompt")
	if err != nil {
		t.Fatalf("WriteTempPrompt() failed: %v", err)
	}
	defer cleanup()

	if !filepath.IsAbs(path) {
		t.Fatalf("WriteTempPrompt() path = %q, want absolute path", path)
	}
}
