package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEpicAcceptanceFileExists(t *testing.T) {
	t.Parallel()
	repoRoot, err := getRepoRoot()
	if err != nil {
		t.Fatalf("getRepoRoot error = %v", err)
	}
	acceptancePath := filepath.Join(repoRoot, "cmd", "gromit", "epic_acceptance_test.go")
	info, err := os.Stat(acceptancePath)
	if err != nil {
		t.Fatalf("acceptance test file %s missing: %v", acceptancePath, err)
	}
	if info.IsDir() {
		t.Fatalf("expected %s to be a file", acceptancePath)
	}
}
