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
	origConfigPath := configPath
	configPath = resolveProjectPath("t", "gromit.yaml")
	defer func() { configPath = origConfigPath }()

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

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "gromit.yaml"), []byte(""), 0644)
	os.MkdirAll(filepath.Join(tmpDir, ".gromit"), 0755)
	t.Chdir(tmpDir)

	stdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		t.Fatalf("failed to write to stdin pipe: %v", err)
	}
	w.Close()
	os.Stdin = r
	defer func() {
		os.Stdin = stdin
	}()

	output := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"add", "Test idea"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("gromit add failed: %v", err)
		}
	})

	if !called {
		t.Fatal("pipeline Add handler was not invoked")
	}

	if captured.Text != "Test idea" {
		t.Fatalf("AddInput.Text = %q, want %q", captured.Text, "Test idea")
	}
	if captured.Context != "" {
		t.Fatalf("AddInput.Context = %q, want empty", captured.Context)
	}

	if !strings.Contains(output, "Added to backlog (feature)") {
		t.Fatalf("stdout missing confirmation message: %s", output)
	}
	if !strings.Contains(output, "Test idea") {
		t.Fatalf("stdout missing idea text: %s", output)
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
	origConfigPath := configPath
	configPath = resolveProjectPath("t", "gromit.yaml")
	defer func() { configPath = origConfigPath }()

	origHandler := addHandler
	defer func() { addHandler = origHandler }()

	var gotType, gotContext string
	addHandler = func(ctx context.Context, cfg *config.Config, gromitDir string, input pipeline.AddInput) (*pipeline.AddResult, error) {
		gotType = input.Type
		gotContext = input.Context
		return &pipeline.AddResult{
			Idea: &pipeline.Idea{
				Text: input.Text,
				Type: input.Type,
			},
			Type: input.Type,
		}, nil
	}

	stdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	// Stdin order: category choice first (scanner reads "2"), then context (scanner reads "test context").
	if _, err := w.Write([]byte("2\ntest context\n")); err != nil {
		t.Fatalf("failed to write stdin: %v", err)
	}
	_ = w.Close()
	os.Stdin = r
	defer func() { os.Stdin = stdin }()

	stdout := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"add", "Do something"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("gromit add failed: %v", err)
		}
	})

	if gotType != "bug" {
		t.Fatalf("type = %q, want %q", gotType, "bug")
	}
	if gotContext != "test context" {
		t.Fatalf("context = %q, want %q", gotContext, "test context")
	}
	if !strings.Contains(stdout, "Added to backlog (bug)") {
		t.Fatalf("stdout missing bug confirmation: %s", stdout)
	}
}
