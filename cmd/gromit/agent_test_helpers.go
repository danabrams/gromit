package main

import (
	"os"
	"path/filepath"
	"testing"
)

func setupAgentConfig(t *testing.T, configYAML string) (string, string) {
	t.Helper()

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("creating specs dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	backlogPath := filepath.Join(gromitDir, "backlog.jsonl")
	if err := os.WriteFile(backlogPath, []byte(""), 0o644); err != nil {
		t.Fatalf("writing backlog file: %v", err)
	}

	return tmpDir, configPath
}
