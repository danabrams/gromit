package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTestLogsDir_ExistsAtRepoRoot verifies that test-logs/ exists at the repo
// root and is safe to create idempotently via os.MkdirAll.
func TestTestLogsDir_ExistsAtRepoRoot(t *testing.T) {
	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}

	testLogsDir := filepath.Join(root, "test-logs")

	t.Run("directory exists", func(t *testing.T) {
		info, err := os.Stat(testLogsDir)
		if err != nil {
			t.Fatalf("test-logs/ does not exist at repo root %q: %v", root, err)
		}
		if !info.IsDir() {
			t.Errorf("test-logs exists but is not a directory")
		}
	})

	t.Run("MkdirAll is safe when directory already exists", func(t *testing.T) {
		if err := os.MkdirAll(testLogsDir, 0o755); err != nil {
			t.Errorf("os.MkdirAll failed on existing directory: %v", err)
		}
	})

	t.Run("directory is writable", func(t *testing.T) {
		tmp, err := os.CreateTemp(testLogsDir, "write-check-*.tmp")
		if err != nil {
			t.Fatalf("test-logs/ is not writable: %v", err)
		}
		tmp.Close()
		os.Remove(tmp.Name())
	})
}
