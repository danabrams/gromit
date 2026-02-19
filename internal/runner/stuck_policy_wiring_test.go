package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestNewRunnerWiresStuckPolicy(t *testing.T) {
	tmpDir := setupNewRunnerDirs(t)
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates: tmpDir + "/templates",
			Specs:     tmpDir + "/specs",
			Logs:      tmpDir + "/logs",
		},
		Claude: config.ClaudeConfig{Binary: "claude"},
	}

	r, err := NewRunner(cfg, nil)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if r.stuckPolicy == nil {
		t.Fatal("expected NewRunner to wire stuckPolicy")
	}
}
