package main

import (
	"os"
	"path/filepath"
	"testing"
)

func prependFakeTools(t *testing.T, testDir string) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	fakesDir := filepath.Join(wd, "..", "..", "test", "fakes")
	path := os.Getenv("PATH")
	t.Setenv("PATH", fakesDir+string(os.PathListSeparator)+path)
	t.Setenv("TEST_DIR", testDir)
}
