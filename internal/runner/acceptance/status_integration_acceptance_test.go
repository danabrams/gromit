//go:build acceptance

package acceptance_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner"
)

// TestOrchestratorHelper_StatusIntegrationIdleWithHistory tests full status output
// when idle with a completed run history, backlog, and state files.
func TestOrchestratorHelper_StatusIntegrationIdleWithHistory(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("Failed to create gromit dir: %v", err)
	}

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Specs: filepath.Join(gromitDir, "specs"),
			Plans: filepath.Join(gromitDir, "plans"),
		},
	}

	// Create backlog.jsonl
	if err := os.WriteFile(filepath.Join(gromitDir, "backlog.jsonl"), []byte(`{"id":"idea-1","text":"Idea one"}`), 0644); err != nil {
		t.Fatalf("Failed to write backlog: %v", err)
	}

	// Create a completed status file (running: false) from 3 hours ago
	sw, _ := runner.NewStatusWriter(gromitDir)
	sw.SetStartTime(time.Now().Add(-3 * time.Hour))
	if err := sw.WriteFinal(25); err != nil {
		t.Fatalf("Failed to write final status: %v", err)
	}

	// Create state.json with never-run retro
	if err := os.WriteFile(filepath.Join(gromitDir, "state.json"), []byte(`{"iterations_since_review": 10}`), 0644); err != nil {
		t.Fatalf("Failed to write state.json: %v", err)
	}

	var buf strings.Builder
	if err := runner.PrintStatus(gromitDir, cfg, &buf, nil); err != nil {
		t.Fatalf("PrintStatus() failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "Pipeline:") {
		t.Errorf("Expected Pipeline section, got: %s", output)
	}
	if !strings.Contains(output, "Run: not running") {
		t.Errorf("Expected 'Run: not running', got: %s", output)
	}
	if !strings.Contains(output, "Last run:") {
		t.Errorf("Expected 'Last run:' info, got: %s", output)
	}
	if !strings.Contains(output, "25 iterations completed") {
		t.Errorf("Expected iteration count, got: %s", output)
	}
	if !strings.Contains(output, "ago") {
		t.Errorf("Expected relative time, got: %s", output)
	}
	if !strings.Contains(output, "Health:") {
		t.Errorf("Expected Health section, got: %s", output)
	}
	if !strings.Contains(output, "Last retro:  never") {
		t.Errorf("Expected 'Last retro: never', got: %s", output)
	}
	if !strings.Contains(output, "Last review: 10 iterations ago") {
		t.Errorf("Expected last review count, got: %s", output)
	}
	if !strings.Contains(output, "Next action:") {
		t.Errorf("Expected recommendation section, got: %s", output)
	}
}
