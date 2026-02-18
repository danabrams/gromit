package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestGetSpecBaseCommit_UsesProvidedClient(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	specName := "client-spec"
	specPath := filepath.Join(specsDir, specName+".md")
	content := `---
id: client-spec
created: 2026-02-11
---

# Client Spec
`
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	called := false
	mockFn := func(args ...string) (string, error) {
		called = true
		return "[]", nil
	}

	mockClient := &bead.Client{RunFn: mockFn}

	_, err := getSpecBaseCommit(mockClient, specName, specsDir)
	if err == nil {
		t.Fatal("getSpecBaseCommit should return error when no beads are found")
	}
	if !called {
		t.Fatal("expected getSpecBaseCommit to use provided bead client")
	}
}
