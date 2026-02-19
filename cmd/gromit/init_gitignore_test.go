package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendToGitignore_AddsCodexHomeEntry(t *testing.T) {
	tempDir := t.TempDir()
	gitignorePath := filepath.Join(tempDir, ".gitignore")

	if err := os.WriteFile(gitignorePath, []byte("node_modules/\n"), 0644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	appendToGitignore(gitignorePath)

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, ".gromit/logs/") {
		t.Fatalf("expected .gromit/logs/ entry in .gitignore, got:\n%s", content)
	}
	if !strings.Contains(content, ".codex-home/") {
		t.Fatalf("expected .codex-home/ entry in .gitignore, got:\n%s", content)
	}
}
