//go:build acceptance

package acceptance_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner"
)

// writeStatusFile marshals a runner.Status to status.json in gromitDir.
func writeStatusFile(gromitDir string, status runner.Status) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(gromitDir, "status.json"), data, 0644)
}

func TestRunnerStatusWithLiveRun(t *testing.T) {
	tests := []struct {
		name              string
		setupStatus       func(gromitDir string) error
		processChecker    func(pid int) bool
		expectedOutput    []string
		notExpected       []string
		expectFileDeleted bool
	}{
		{
			name: "No status file - shows pipeline status",
			setupStatus: func(gromitDir string) error {
				return nil
			},
			expectedOutput: []string{"Pipeline:", "Run: not running", "Health:", "Next action:"},
			notExpected:    []string{"Warning: stale run"},
		},
		{
			name: "Live run - shows run in progress",
			processChecker: func(pid int) bool {
				return true
			},
			setupStatus: func(gromitDir string) error {
				status := runner.Status{
					Running:   true,
					Iteration: 1,
					BeadID:    "bead-123",
					BeadTitle: "Building feature X",
					Model:     "sonnet",
					StartedAt: time.Now().Add(-2 * time.Minute),
					ElapsedS:  120,
					PID:       424242,
				}
				return writeStatusFile(gromitDir, status)
			},
			expectedOutput: []string{"Pipeline:", "Run: iteration 1", "bead-123", "Building feature X", "Model:    sonnet", "Health:"},
			notExpected:    []string{"Warning: stale run"},
		},
		{
			name: "Stale status file - warns and cleans up",
			processChecker: func(pid int) bool {
				return false
			},
			setupStatus: func(gromitDir string) error {
				status := runner.Status{
					Running:   true,
					Iteration: 2,
					BeadID:    "bead-456",
					BeadTitle: "Old bead",
					Model:     "haiku",
					StartedAt: time.Now().Add(-1 * time.Hour),
					ElapsedS:  3600,
					PID:       999999, // PID that won't exist
				}
				return writeStatusFile(gromitDir, status)
			},
			expectedOutput:    []string{"Warning: stale run detected", "Bead: bead-456 - Old bead", "Removing stale status file", "Pipeline:", "Run: not running"},
			notExpected:       []string{"Run: iteration"},
			expectFileDeleted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			gromitDir := filepath.Join(tmpDir, ".gromit")
			if err := os.MkdirAll(gromitDir, 0755); err != nil {
				t.Fatalf("Failed to create gromit dir: %v", err)
			}

			if err := tt.setupStatus(gromitDir); err != nil {
				t.Fatalf("Failed to setup status: %v", err)
			}

			mockBeads := &mockBeadClient{
				ReadyFn: func() (*bead.Bead, error) {
					return &bead.Bead{
						ID:       "test-1",
						Title:    "Test bead",
						Priority: 1,
						Labels:   []string{},
					}, nil
				},
			}

			cfg := &config.Config{}
			cfg.Paths.Specs = filepath.Join(gromitDir, "specs")
			cfg.Paths.Plans = filepath.Join(gromitDir, "plans")
			var buf strings.Builder
			deps := runner.Deps{
				Beads:    mockBeads,
				Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
				Analyzer: &mockFailureAnalyzer{},
				Renderer: &mockPromptRenderer{},
				Logger:   &mockIterationLogger{},
			}
			if tt.processChecker != nil {
				deps.ProcessChecker = tt.processChecker
			}
			r, err := runner.NewRunnerWithDeps(cfg, &buf, gromitDir, deps)
			if err != nil {
				t.Fatalf("NewRunnerWithDeps failed: %v", err)
			}

			if err = r.Status(); err != nil {
				t.Fatalf("Status() failed: %v", err)
			}

			output := buf.String()
			for _, expected := range tt.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q\nGot:\n%s", expected, output)
				}
			}
			for _, notExpected := range tt.notExpected {
				if strings.Contains(output, notExpected) {
					t.Errorf("Expected output NOT to contain %q\nGot:\n%s", notExpected, output)
				}
			}

			if tt.expectFileDeleted {
				statusPath := filepath.Join(gromitDir, "status.json")
				if _, err := os.Stat(statusPath); !os.IsNotExist(err) {
					t.Errorf("Expected status.json to be deleted, but it still exists")
				}
			}
		})
	}
}

// getDeadPID finds a PID that is not currently alive.
func getDeadPID(t *testing.T) int {
	t.Helper()

	// Probe a high PID range to avoid shelling out for a throwaway subprocess.
	for pid := 1 << 20; pid < (1<<20)+100000; pid++ {
		if !runner.IsProcessAlive(pid) {
			return pid
		}
	}
	t.Fatal("failed to find a dead PID in probe range")
	return 0
}

func TestRunner_Status_LivePID(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("Failed to create gromit dir: %v", err)
	}

	var buf strings.Builder
	cfg := &config.Config{}
	cfg.Paths.Specs = filepath.Join(gromitDir, "specs")
	cfg.Paths.Plans = filepath.Join(gromitDir, "plans")

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{
				ID:       "test-456",
				Title:    "Next Bead",
				Priority: 1,
				Labels:   []string{},
			}, nil
		},
	}

	r, err := runner.NewRunnerWithDeps(cfg, &buf, gromitDir, runner.Deps{
		Beads:    mockBeads,
		Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Write a status file with live PID (current process)
	sw, _ := runner.NewStatusWriter(gromitDir)
	err = sw.Write(3, "running-bead-789", "Running Bead Title", "opus", true, 0, 0)
	if err != nil {
		t.Fatalf("Failed to write status file: %v", err)
	}

	// Execute
	err = r.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}

	// Verify - should show run-in-progress info in new format
	output := buf.String()
	if !strings.Contains(output, "Run: iteration 3") {
		t.Errorf("Expected 'Run: iteration 3' message for live PID, got: %s", output)
	}
	if !strings.Contains(output, "running-bead-789") {
		t.Errorf("Expected bead ID in output, got: %s", output)
	}
	if !strings.Contains(output, "Running Bead Title") {
		t.Errorf("Expected bead title in output, got: %s", output)
	}
	if !strings.Contains(output, "Model:    opus") {
		t.Errorf("Expected model in output, got: %s", output)
	}
	if strings.Contains(output, "stale run") {
		t.Errorf("Should not show stale run message for live PID, got: %s", output)
	}

	// Should show pipeline and health sections
	if !strings.Contains(output, "Pipeline:") {
		t.Errorf("Expected Pipeline section in output, got: %s", output)
	}
	if !strings.Contains(output, "Health:") {
		t.Errorf("Expected Health section in output, got: %s", output)
	}

	// Verify status file still exists (not deleted for live run)
	statusPath := filepath.Join(gromitDir, "status.json")
	if _, err := os.Stat(statusPath); err != nil {
		t.Errorf("Status file should still exist for live run: %v", err)
	}
}

func TestRunner_Status_DeadPID(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("Failed to create gromit dir: %v", err)
	}

	var buf strings.Builder
	cfg := &config.Config{}
	cfg.Paths.Specs = filepath.Join(gromitDir, "specs")
	cfg.Paths.Plans = filepath.Join(gromitDir, "plans")

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{
				ID:       "test-999",
				Title:    "Next Available Bead",
				Priority: 2,
				Labels:   []string{},
			}, nil
		},
	}

	r, err := runner.NewRunnerWithDeps(cfg, &buf, gromitDir, runner.Deps{
		Beads:    mockBeads,
		Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Get a dead PID by probing high PID range
	deadPID := getDeadPID(t)

	// Write a status file with the dead PID
	statusPath := filepath.Join(gromitDir, "status.json")
	statusData := fmt.Sprintf(`{
  "running": true,
  "iteration": 5,
  "bead_id": "crashed-bead-999",
  "bead_title": "Crashed Bead Title",
  "model": "sonnet",
  "started_at": "%s",
  "elapsed_s": 120,
  "pid": %d
}`, time.Now().Add(-2*time.Hour).Format(time.RFC3339), deadPID)
	err = os.WriteFile(statusPath, []byte(statusData), 0644)
	if err != nil {
		t.Fatalf("Failed to write status file: %v", err)
	}

	// Execute
	err = r.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}

	// Verify - should show stale run warning
	output := buf.String()
	if !strings.Contains(output, "stale run") {
		t.Errorf("Expected 'stale run' message for dead PID, got: %s", output)
	}
	if !strings.Contains(output, "crashed-bead-999") {
		t.Errorf("Expected bead ID in stale run warning, got: %s", output)
	}
	if !strings.Contains(output, "Crashed Bead Title") {
		t.Errorf("Expected bead title in stale run warning, got: %s", output)
	}
	if !strings.Contains(output, "Removing stale status file") {
		t.Errorf("Expected file removal message, got: %s", output)
	}

	// Should show pipeline status after warning
	if !strings.Contains(output, "Pipeline:") {
		t.Errorf("Expected Pipeline section after stale run warning, got: %s", output)
	}
	if !strings.Contains(output, "Run: not running") {
		t.Errorf("Expected 'Run: not running' after cleaning stale status, got: %s", output)
	}

	// Verify status file was deleted
	if _, err := os.Stat(statusPath); err == nil {
		t.Error("Status file should have been deleted for dead PID")
	} else if !os.IsNotExist(err) {
		t.Errorf("Unexpected error checking status file: %v", err)
	}
}

func TestRunner_Status_Integration_ActiveRun(t *testing.T) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("Failed to create gromit dir: %v", err)
	}

	var buf strings.Builder
	cfg := &config.Config{
		Paths: config.PathsConfig{
			Specs: filepath.Join(gromitDir, "specs"),
			Plans: filepath.Join(gromitDir, "plans"),
		},
	}

	mockBeads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return &bead.Bead{
				ID:       "next-bead-123",
				Title:    "Next Available Bead",
				Priority: 1,
				Labels:   []string{},
			}, nil
		},
	}

	r, err := runner.NewRunnerWithDeps(cfg, &buf, gromitDir, runner.Deps{
		Beads:    mockBeads,
		Router:   newMockRouterFromClaudeClient(&mockClaudeClient{}),
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	// Create backlog.jsonl with some items
	backlogPath := filepath.Join(gromitDir, "backlog.jsonl")
	backlogContent := `{"id":"idea-1","text":"Add rate limiting"}
{"id":"idea-2","text":"Support webhooks"}`
	err = os.WriteFile(backlogPath, []byte(backlogContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write backlog: %v", err)
	}

	// Create a running status file with limits
	sw, _ := runner.NewStatusWriter(gromitDir)
	err = sw.Write(12, "active-bead-456", "Build user profiles", "sonnet", true, 50, 30)
	if err != nil {
		t.Fatalf("Failed to write status: %v", err)
	}

	// Create state.json with review data
	stateContent := `{
		"iterations_since_review": 5
	}`
	err = os.WriteFile(filepath.Join(gromitDir, "state.json"), []byte(stateContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write state.json: %v", err)
	}
	// Create interactive-state.json with last retro data
	interactiveContent := fmt.Sprintf(`{
		"last_retro": "%s"
	}`, time.Now().Add(-2*time.Hour).Format(time.RFC3339))
	err = os.WriteFile(filepath.Join(gromitDir, "interactive-state.json"), []byte(interactiveContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write interactive-state.json: %v", err)
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
	if !strings.Contains(output, "2 unrefined idea") {
		t.Errorf("Expected backlog count in output, got: %s", output)
	}

	// Verify Run section shows active run with limits
	if !strings.Contains(output, "Run: iteration 12/50") {
		t.Errorf("Expected 'Run: iteration 12/50', got: %s", output)
	}
	if !strings.Contains(output, "of 30m elapsed") {
		t.Errorf("Expected time budget in output, got: %s", output)
	}
	if !strings.Contains(output, "active-bead-456") {
		t.Errorf("Expected current bead ID in output, got: %s", output)
	}
	if !strings.Contains(output, "Build user profiles") {
		t.Errorf("Expected current bead title in output, got: %s", output)
	}
	if !strings.Contains(output, "Model:    sonnet") {
		t.Errorf("Expected model in output, got: %s", output)
	}

	// Verify SPC section exists
	if !strings.Contains(output, "SPC: (no data)") {
		t.Errorf("Expected SPC section, got: %s", output)
	}

	// Verify Health section
	if !strings.Contains(output, "Health:") {
		t.Errorf("Expected Health section, got: %s", output)
	}
	if !strings.Contains(output, "Last retro:") {
		t.Errorf("Expected last retro in output, got: %s", output)
	}
	if !strings.Contains(output, "Last review: 5 iterations ago") {
		t.Errorf("Expected last review in output, got: %s", output)
	}

	// Verify recommendation section exists
	if !strings.Contains(output, "Next action:") {
		t.Errorf("Expected recommendation section, got: %s", output)
	}
}
