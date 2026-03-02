package pipeline

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go/format"
)

func TestPipelineAndPromptGofmt(t *testing.T) {
	repo := repoRoot(t)
	requireGofmt(t, filepath.Join(repo, "internal", "pipeline"), filepath.Join(repo, "internal", "prompt"))
}

func requireGofmt(t *testing.T, roots ...string) {
	t.Helper()
	var notFormatted []string
	for _, root := range roots {
		if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			formatted, fmtErr := format.Source(data)
			if fmtErr != nil {
				return fmt.Errorf("format %s: %w", path, fmtErr)
			}
			if !bytes.Equal(data, formatted) {
				notFormatted = append(notFormatted, path)
			}
			return nil
		}); err != nil {
			t.Fatalf("failed to walk %s: %v", root, err)
		}
	}
	if len(notFormatted) > 0 {
		t.Fatalf("gofmt would change %d file(s): %s", len(notFormatted), strings.Join(notFormatted, ", "))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot determine working dir: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found while walking up from %s", dir)
		}
		dir = parent
	}
}
