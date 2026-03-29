package gap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadChangedFilesFromDiff(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	diffPath := filepath.Join(tmp, "gap-analysis.diff")
	content := "internal/foo/foo.go\n\ninternal/bar/bar.go\n"
	if err := os.WriteFile(diffPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write diff file: %v", err)
	}

	changed, err := ReadChangedFiles(diffPath)
	if err != nil {
		t.Fatalf("ReadChangedFiles returned error: %v", err)
	}

	if len(changed) != 2 {
		t.Fatalf("got %d changed files, want 2", len(changed))
	}

	if changed[0] != "internal/foo/foo.go" {
		t.Fatalf("unexpected first file %q", changed[0])
	}
	if changed[1] != "internal/bar/bar.go" {
		t.Fatalf("unexpected second file %q", changed[1])
	}
}
