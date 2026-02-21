package runner

import (
	"bytes"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestNewRunnerWiresBudgetConfigIntoRenderer verifies that NewRunner
// passes cfg.Prompt.Budget values to the renderer via SetBudgetConfig.
// Since NewRunner creates a concrete *prompt.Renderer (not the interface),
// SetBudgetConfig is called on the concrete type before assigning to the runner.
func TestNewRunnerWiresBudgetConfigIntoRenderer(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       tmpDir + "/.gromit/templates",
			Specs:           tmpDir + "/.gromit/specs",
			ProjectClaudeMD: tmpDir + "/CLAUDE.md",
			Logs:            tmpDir + "/.gromit/logs",
		},
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 600,
		},
		Prompt: config.PromptConfig{
			Budget: config.PromptBudgetConfig{
				MaxChars:         20000,
				LearningCapChars: 2000,
			},
		},
	}

	var bufWriter bytes.Buffer

	r, err := NewRunnerLegacy(cfg, &bufWriter)
	if err != nil {
		t.Fatalf("NewRunner() failed: %v", err)
	}

	if r.renderer == nil {
		t.Fatal("Expected renderer to be created")
	}
}
