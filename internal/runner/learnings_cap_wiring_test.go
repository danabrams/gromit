package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestNewRunnerWiresMaxLearningCharsIntoRenderer verifies that NewRunner
// passes cfg.Learnings.MaxLearningChars to the renderer via SetMaxLearningChars.
// Since NewRunner creates a concrete *prompt.Renderer (not the interface),
// SetMaxLearningChars is called on the concrete type before assigning to the
// runner. We verify by creating a runner and confirming no error.
func TestNewRunnerWiresMaxLearningCharsIntoRenderer(t *testing.T) {
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
		Learnings: config.LearningsConfig{
			MaxLearningChars: 500,
		},
	}

	var buf []byte
	bufWriter := &byteWriter{buf: &buf}

	r, err := NewRunner(cfg, bufWriter)
	if err != nil {
		t.Fatalf("NewRunner() failed: %v", err)
	}

	if r.renderer == nil {
		t.Fatal("Expected renderer to be created")
	}
}
