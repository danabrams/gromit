package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// TestAddCommand_UsesPipelineAdd verifies the CLI delegates idea creation to pipeline.Add.
func TestAddCommand_UsesPipelineAdd(t *testing.T) {
	originalHandler := addHandler
	defer func() { addHandler = originalHandler }()

	called := false
	var captured pipeline.AddInput
	addHandler = func(ctx context.Context, cfg *config.Config, gromitDir string, input pipeline.AddInput) (*pipeline.AddResult, error) {
		called = true
		captured = input
		return &pipeline.AddResult{
			Idea: &pipeline.Idea{
				Text: "Test idea",
				Type: "feature",
			},
			Type: "feature",
		}, nil
	}

	stdout, stderr, exitCode := runGromitWithStdin(t, "\n", "add", "Test idea")
	if exitCode != 0 {
		t.Fatalf("gromit add exited with code %d\nstdout: %s\nstderr: %s", exitCode, stdout, stderr)
	}

	if !called {
		t.Fatal("pipeline Add handler was not invoked")
	}
	if captured.Text != "Test idea" {
		t.Fatalf("AddInput.Text = %q, want %q", captured.Text, "Test idea")
	}
	if captured.Context != "" {
		t.Fatalf("AddInput.Context = %q, want empty", captured.Context)
	}

	if !strings.Contains(stdout, "Added to backlog (feature)") {
		t.Fatalf("stdout missing confirmation message: %s", stdout)
	}
	if !strings.Contains(stdout, "Test idea") {
		t.Fatalf("stdout missing idea text: %s", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr output, got %q", stderr)
	}
}

// TestAddCommand_MultiWordContext verifies that multi-word context strings are captured in full
func TestAddCommand_MultiWordContext(t *testing.T) {

	// Create a temporary directory for the test

	tmpDir := t.TempDir()

	// Create .gromit directory
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create .gromit directory: %v", err)
	}

	// Create minimal gromit.yaml pointing to our temp .gromit directory
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := "paths:\n  gromit_dir: " + gromitDir + "\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Change to temp directory so config is found
	t.Chdir(tmpDir)

	tests := []struct {
		name            string
		ideaText        string
		contextInput    string
		expectedContext string
	}{
		{
			name:            "multi-word context",
			ideaText:        "Add user authentication",
			contextInput:    "this should work with the new auth system\n",
			expectedContext: "this should work with the new auth system",
		},
		{
			name:            "single-word context",
			ideaText:        "Fix bug",
			contextInput:    "urgent\n",
			expectedContext: "urgent",
		},
		{
			name:            "empty context",
			ideaText:        "Refactor code",
			contextInput:    "\n",
			expectedContext: "",
		},
		{
			name:            "context with leading/trailing spaces",
			ideaText:        "Add feature",
			contextInput:    "  needs discussion with team  \n",
			expectedContext: "needs discussion with team",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare stdin: context input (no category prompt since ideas auto-categorize as "feature")
			stdin := tt.contextInput

			// Run the add command
			stdout, stderr, exitCode := runGromitWithStdin(t, stdin, "add", tt.ideaText)

			if exitCode != 0 {
				t.Errorf("gromit add exited with code %d\nstdout: %s\nstderr: %s", exitCode, stdout, stderr)
			}

			// Verify the idea was added to the backlog
			backlogPath := filepath.Join(gromitDir, "backlog.jsonl")
			data, err := os.ReadFile(backlogPath)
			if err != nil {
				t.Fatalf("failed to read backlog file: %v", err)
			}

			// Parse the last line (most recent idea)
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) == 0 {
				t.Fatal("backlog file is empty")
			}
			lastLine := lines[len(lines)-1]

			var idea backlog.Idea
			if err := json.Unmarshal([]byte(lastLine), &idea); err != nil {
				t.Fatalf("failed to unmarshal idea: %v", err)
			}

			// Verify the context field
			if idea.Context != tt.expectedContext {
				t.Errorf("context = %q, want %q", idea.Context, tt.expectedContext)
			}

			// Verify the text field as well
			if idea.Text != tt.ideaText {
				t.Errorf("text = %q, want %q", idea.Text, tt.ideaText)
			}

			// Clean up the backlog file for the next test
			if err := os.Remove(backlogPath); err != nil {
				t.Fatalf("failed to remove backlog file: %v", err)
			}
		})
	}
}

// TestAddCommand_CategoryChoice verifies the category prompt still works correctly
func TestAddCommand_CategoryChoice(t *testing.T) {

	// Create a temporary directory for the test

	tmpDir := t.TempDir()

	// Create .gromit directory
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create .gromit directory: %v", err)
	}

	// Create minimal gromit.yaml
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	configContent := "paths:\n  gromit_dir: " + gromitDir + "\n"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Change to temp directory
	t.Chdir(tmpDir)

	// Use an idea that won't auto-categorize (ambiguous)
	ideaText := "Do something"
	categoryChoice := "2\n" // Choose "bug"
	contextInput := "test context\n"
	stdin := categoryChoice + contextInput

	// Run the add command
	stdout, stderr, exitCode := runGromitWithStdin(t, stdin, "add", ideaText)

	if exitCode != 0 {
		t.Errorf("gromit add exited with code %d\nstdout: %s\nstderr: %s", exitCode, stdout, stderr)
	}

	// Verify the idea was added with correct type
	backlogPath := filepath.Join(gromitDir, "backlog.jsonl")
	data, err := os.ReadFile(backlogPath)
	if err != nil {
		t.Fatalf("failed to read backlog file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		t.Fatal("backlog file is empty")
	}
	lastLine := lines[len(lines)-1]

	var idea backlog.Idea
	if err := json.Unmarshal([]byte(lastLine), &idea); err != nil {
		t.Fatalf("failed to unmarshal idea: %v", err)
	}

	// Verify the type was set correctly by the category choice
	if idea.Type != "bug" {
		t.Errorf("type = %q, want %q", idea.Type, "bug")
	}

	// Verify the context was also captured
	if idea.Context != "test context" {
		t.Errorf("context = %q, want %q", idea.Context, "test context")
	}
}
