package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestGetEpicBaseCommit_UsesProvidedClient(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	specContent := `---
id: epic-spec
epic: client-epic
created: 2026-02-22
---

# Epic Spec
`
	specPath := filepath.Join(specsDir, "epic-spec.md")
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	called := false
	mockFn := func(args ...string) (string, error) {
		called = true
		return "[]", nil
	}
	mockClient := &bead.Client{RunFn: mockFn}

	_, err := getEpicBaseCommit(mockClient, "client-epic", specsDir, tempDir)
	if err == nil {
		t.Fatal("getEpicBaseCommit should return error when no beads are found")
	}
	if !called {
		t.Fatal("expected getEpicBaseCommit to use provided bead client")
	}
}
