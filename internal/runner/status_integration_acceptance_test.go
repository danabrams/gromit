//go:build acceptance

package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

func TestRunner_Status_Integration_IdleWithHistory(t *testing.T) {
	tmpDir := t.TempDir()
	var buf strings.Builder
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Specs: filepath.Join(tmpDir, "specs"),
			Plans: filepath.Join(tmpDir, "plans"),
		},
	}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{
				ID:       "next-bead-789",
				Title:    "Next Work Item",
				Priority: 2,
				Labels:   []string{},
			}, nil
		},
	}

	r, err := NewRunnerWithDeps(cfg, &buf, tmpDir, Deps{
		Beads:    mockBeads,
		Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Create backlog.jsonl
	backlogPath := filepath.Join(tmpDir, "backlog.jsonl")
	err = os.WriteFile(backlogPath, []byte(`{"id":"idea-1","text":"Idea one"}`), 0644)
	if err != nil {
		t.Fatalf("Failed to write backlog: %v", err)
	}

	// Create a completed status file (running: false) from 3 hours ago
	sw, _ := NewStatusWriter(tmpDir)
	sw.startTime = time.Now().Add(-3 * time.Hour)
	err = sw.WriteFinal(25)
	if err != nil {
		t.Fatalf("Failed to write final status: %v", err)
	}

	// Create state.json with never-run retro
	stateContent := `{
		"iterations_since_review": 10
	}`
	err = os.WriteFile(filepath.Join(tmpDir, "state.json"), []byte(stateContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write state.json: %v", err)
	}

	// Execute
	err = r.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}

	output := buf.String()

	// Verify Pipeline section
	if !strings.Contains(output, "Pipeline:") {
		t.Errorf("Expected Pipeline section, got: %s", output)
	}

	// Verify Run section shows idle with last run info
	if !strings.Contains(output, "Run: not running") {
		t.Errorf("Expected 'Run: not running', got: %s", output)
	}
	if !strings.Contains(output, "Last run:") {
		t.Errorf("Expected 'Last run:' info, got: %s", output)
	}
	if !strings.Contains(output, "25 iterations completed") {
		t.Errorf("Expected iteration count in last run info, got: %s", output)
	}
	if !strings.Contains(output, "ago") {
		t.Errorf("Expected relative time in last run info, got: %s", output)
	}

	// Verify Health section
	if !strings.Contains(output, "Health:") {
		t.Errorf("Expected Health section, got: %s", output)
	}
	if !strings.Contains(output, "Last retro:  never") {
		t.Errorf("Expected 'Last retro: never', got: %s", output)
	}
	if !strings.Contains(output, "Last review: 10 iterations ago") {
		t.Errorf("Expected last review count, got: %s", output)
	}

	// Verify recommendation section exists
	if !strings.Contains(output, "Next action:") {
		t.Errorf("Expected recommendation section, got: %s", output)
	}
}
