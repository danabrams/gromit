package pipeline

import (
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
