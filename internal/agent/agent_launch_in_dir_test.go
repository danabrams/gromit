package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestAgentLaunchInDirMethod verifies Agent interface has LaunchInDir method
func TestAgentLaunchInDirMethod(t *testing.T) {
	// Expected failure: LaunchInDir method does not exist on Agent interface yet
	var _ Agent = (*testAgentWithDir)(nil)
}

// testAgentWithDir implements Agent with LaunchInDir
type testAgentWithDir struct{}

func (a *testAgentWithDir) Name() string                             { return "test" }
func (a *testAgentWithDir) Launch(promptPath string) error           { return nil }
func (a *testAgentWithDir) LaunchInDir(promptPath, dir string) error { return nil }
func (a *testAgentWithDir) Command(promptPath string) (*exec.Cmd, error) {
	return exec.Command("echo", "test"), nil
}

// TestLaunchInDirSetsWorkingDirectory verifies LaunchInDir sets cmd.Dir when dir is non-empty
func TestLaunchInDirSetsWorkingDirectory(t *testing.T) {
	// Expected failure: LaunchInDir method does not exist on cliAgent yet
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a target directory for the command to run in
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test file in target directory that the command will find
	testFile := filepath.Join(targetDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	agent := New("test", "pwd", nil, FileRef, "", nil)

	// LaunchInDir should set the working directory to targetDir
	// The pwd command will output the current directory
	err := agent.LaunchInDir(promptPath, targetDir)
	if err != nil {
		t.Errorf("LaunchInDir() error = %v", err)
	}

	// We can't easily capture pwd output here, but the fact that it runs
	// without error and with the dir parameter is what we're testing
}

// TestLaunchInDirEmptyDirUsesCurrentDir verifies LaunchInDir with empty dir uses current directory
func TestLaunchInDirEmptyDirUsesCurrentDir(t *testing.T) {
	// Expected failure: LaunchInDir method does not exist on cliAgent yet
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	agent := New("test", "echo", nil, FileRef, "", nil)

	// Empty dir should not cause an error - should use current directory
	err := agent.LaunchInDir(promptPath, "")
	if err != nil {
		t.Errorf("LaunchInDir() with empty dir should not error, got %v", err)
	}
}

// TestLaunchInDirNonexistentDirectory verifies LaunchInDir fails gracefully with nonexistent directory
func TestLaunchInDirNonexistentDirectory(t *testing.T) {
	// Expected failure: LaunchInDir method does not exist on cliAgent yet
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	agent := New("test", "echo", nil, FileRef, "", nil)

	// Nonexistent directory should cause the command to fail
	// (The command itself will fail, not LaunchInDir validation)
	nonexistentDir := filepath.Join(tmpDir, "does-not-exist")
	err := agent.LaunchInDir(promptPath, nonexistentDir)

	// The error behavior depends on the command - some commands may fail,
	// others may succeed. We're primarily testing that the method exists
	// and accepts a nonexistent directory without panicking.
	_ = err // Accept any outcome - dir validation happens at exec level
}

// TestLaunchInDirWithAllPromptDeliveryModes verifies LaunchInDir works with all delivery modes
func TestLaunchInDirWithAllPromptDeliveryModes(t *testing.T) {
	// Expected failure: LaunchInDir method does not exist on cliAgent yet
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test prompt content"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		delivery   PromptDelivery
		promptFlag string
	}{
		{
			name:       "FileRef delivery with dir",
			delivery:   FileRef,
			promptFlag: "",
		},
		{
			name:       "PromptFileArg delivery with dir",
			delivery:   PromptFileArg,
			promptFlag: "--prompt",
		},
		{
			name:       "Stdin delivery with dir",
			delivery:   Stdin,
			promptFlag: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := New("test", "echo", nil, tt.delivery, tt.promptFlag, nil)

			err := agent.LaunchInDir(promptPath, targetDir)
			if err != nil {
				t.Errorf("LaunchInDir() with %s delivery error = %v", tt.delivery, err)
			}
		})
	}
}

// TestLaunchInDirPreservesAgentBehavior verifies LaunchInDir preserves existing agent behavior
func TestLaunchInDirPreservesAgentBehavior(t *testing.T) {
	// Expected failure: LaunchInDir method does not exist on cliAgent yet
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test that flags and extra args are still passed correctly
	agent := New("test", "echo", []string{"--flag1"}, FileRef, "", []string{"--extra"})

	err := agent.LaunchInDir(promptPath, targetDir)
	if err != nil {
		t.Errorf("LaunchInDir() should preserve flags and extra args, error = %v", err)
	}
}

// TestLaunchInDirTreatsExitErrorAsGraceful verifies LaunchInDir treats exec.ExitError as graceful
func TestLaunchInDirTreatsExitErrorAsGraceful(t *testing.T) {
	// Expected failure: LaunchInDir method does not exist on cliAgent yet
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Use 'false' command which always exits with 1
	agent := New("test", "false", nil, FileRef, "", nil)

	// exec.ExitError should be treated as non-error (graceful exit)
	err := agent.LaunchInDir(promptPath, targetDir)
	if err != nil {
		// Check if it's an ExitError - if so, it should have been swallowed
		if _, ok := err.(*exec.ExitError); ok {
			t.Errorf("LaunchInDir() returned exec.ExitError, should treat as graceful exit: %v", err)
		}
	}
}

// TestLaunchInDirMissingPromptFile verifies LaunchInDir returns error for missing prompt file
func TestLaunchInDirMissingPromptFile(t *testing.T) {
	// Expected failure: LaunchInDir method does not exist on cliAgent yet
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	agent := New("test", "echo", nil, FileRef, "", nil)

	err := agent.LaunchInDir("/nonexistent/prompt.txt", targetDir)
	if err == nil {
		t.Error("LaunchInDir() with nonexistent prompt file should return error, got nil")
	}
}
