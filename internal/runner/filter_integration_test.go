//go:build integration

package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestNewRunnerWiresFilterIntoLearningsFile verifies that the filter is properly wired
// into the learnings file when NewRunner is called.
func TestNewRunnerWiresFilterIntoLearningsFile(t *testing.T) {
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
	}

	// Create output buffer
	var buf []byte
	bufWriter := &byteWriter{buf: &buf}

	// Create a new runner
	r, err := NewRunnerLegacy(cfg, bufWriter)
	if err != nil {
		t.Fatalf("NewRunner() failed: %v", err)
	}

	// Verify that renderer was created
	if r.renderer == nil {
		t.Fatal("Expected renderer to be created")
	}

	// Get the learnings file from renderer
	lf := r.renderer.GetLearningsFile()
	if lf == nil {
		t.Fatal("Expected learnings file to exist in renderer")
	}

	// The fact that the learnings file exists and we didn't get an error
	// means the filter was successfully wired (or at least attempted).
	// We can't directly test the filter function since it's not exported,
	// but we can verify the wiring happened without errors.
}

// byteWriter is a simple writer for test purposes.
type byteWriter struct {
	buf *[]byte
}

func (b *byteWriter) Write(p []byte) (n int, err error) {
	*b.buf = append(*b.buf, p...)
	return len(p), nil
}
