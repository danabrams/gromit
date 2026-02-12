package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestAgentInterface verifies the Agent interface exists with correct methods
func TestAgentInterface(t *testing.T) {
	// This test will compile only if Agent interface exists with Name() and Launch() methods
	var _ Agent = (*testAgent)(nil)
}

// testAgent is a minimal implementation for interface verification
type testAgent struct{}

func (a *testAgent) Name() string                   { return "test" }
func (a *testAgent) Launch(promptPath string) error { return nil }
func (a *testAgent) Command(promptPath string) (*exec.Cmd, error) {
	return exec.Command("echo", "test"), nil
}

// TestPromptDeliveryConstants verifies all three PromptDelivery constants exist
func TestPromptDeliveryConstants(t *testing.T) {
	tests := []struct {
		name     string
		delivery PromptDelivery
		want     string
	}{
		{
			name:     "FileRef constant exists",
			delivery: FileRef,
			want:     "file_ref",
		},
		{
			name:     "PromptFileArg constant exists",
			delivery: PromptFileArg,
			want:     "prompt_file_arg",
		},
		{
			name:     "Stdin constant exists",
			delivery: Stdin,
			want:     "stdin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.delivery) != tt.want {
				t.Errorf("PromptDelivery constant = %q, want %q", tt.delivery, tt.want)
			}
		})
	}
}

// TestNewAgent verifies the New constructor creates an Agent with correct configuration
func TestNewAgent(t *testing.T) {
	agent := New("test-agent", "test-binary", []string{"--flag1"}, FileRef, "", nil)

	if agent == nil {
		t.Fatal("New() returned nil, want non-nil Agent")
	}

	if agent.Name() != "test-agent" {
		t.Errorf("Name() = %q, want %q", agent.Name(), "test-agent")
	}
}

// TestLaunchFileRef verifies Launch constructs correct command for file_ref delivery
func TestLaunchFileRef(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	promptContent := "test prompt"
	if err := os.WriteFile(promptPath, []byte(promptContent), 0644); err != nil {
		t.Fatal(err)
	}

	agent := New("test", "echo", nil, FileRef, "", nil)

	// Should succeed (echo accepts any arguments)
	err := agent.Launch(promptPath)
	if err != nil {
		t.Errorf("Launch() with FileRef delivery failed: %v", err)
	}
}

// TestLaunchPromptFileArg verifies Launch constructs correct command for prompt_file_arg delivery
func TestLaunchPromptFileArg(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	agent := New("test", "echo", nil, PromptFileArg, "--prompt", nil)

	// Should succeed (echo accepts any arguments)
	err := agent.Launch(promptPath)
	if err != nil {
		t.Errorf("Launch() with PromptFileArg delivery failed: %v", err)
	}
}

// TestLaunchStdin verifies Launch pipes prompt content to stdin
func TestLaunchStdin(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	promptContent := "test prompt content"
	if err := os.WriteFile(promptPath, []byte(promptContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Use 'cat' to read stdin - it echoes stdin and succeeds
	agent := New("test", "cat", nil, Stdin, "", nil)

	// Should succeed (cat reads stdin without error)
	err := agent.Launch(promptPath)
	if err != nil {
		t.Errorf("Launch() with Stdin delivery failed: %v", err)
	}
}

// TestLaunchWithExitError verifies that exec.ExitError is treated as graceful exit
func TestLaunchWithExitError(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Use 'false' command which always exits with 1
	agent := New("test", "false", nil, FileRef, "", nil)

	// exec.ExitError should be treated as non-error (graceful exit)
	err := agent.Launch(promptPath)
	if err != nil {
		// Check if it's an ExitError - if so, it should have been swallowed
		if _, ok := err.(*exec.ExitError); ok {
			t.Errorf("Launch() returned exec.ExitError, should treat as graceful exit: %v", err)
		}
	}
}

// TestLaunchWithExtraArgs verifies extraArgs are passed to the command
func TestLaunchWithExtraArgs(t *testing.T) {
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	agent := New("test", "echo", nil, FileRef, "", []string{"--extra-arg"})

	// Should succeed (echo accepts any arguments)
	err := agent.Launch(promptPath)
	if err != nil {
		t.Errorf("Launch() with extra args failed: %v", err)
	}
}

// TestCliAgentFields verifies cliAgent struct has all required fields
func TestCliAgentFields(t *testing.T) {
	// Create an agent and verify we can access it as cliAgent
	agent := New("test", "binary", []string{"flag"}, FileRef, "--prompt", []string{"extra"})

	ca, ok := agent.(*cliAgent)
	if !ok {
		t.Fatal("New() should return *cliAgent")
	}

	// Verify fields exist and are set correctly
	if ca.name != "test" {
		t.Errorf("name = %q, want %q", ca.name, "test")
	}

	if ca.binary != "binary" {
		t.Errorf("binary = %q, want %q", ca.binary, "binary")
	}

	if len(ca.flags) != 1 || ca.flags[0] != "flag" {
		t.Errorf("flags = %v, want [flag]", ca.flags)
	}

	if ca.promptDelivery != FileRef {
		t.Errorf("promptDelivery = %q, want %q", ca.promptDelivery, FileRef)
	}

	if ca.promptFlag != "--prompt" {
		t.Errorf("promptFlag = %q, want %q", ca.promptFlag, "--prompt")
	}

	if len(ca.extraArgs) != 1 || ca.extraArgs[0] != "extra" {
		t.Errorf("extraArgs = %v, want [extra]", ca.extraArgs)
	}
}

// TestLaunchNonexistentPromptFile verifies Launch returns error for missing prompt file
func TestLaunchNonexistentPromptFile(t *testing.T) {
	agent := New("test", "nonexistent-binary", nil, FileRef, "", nil)

	err := agent.Launch("/nonexistent/path/to/prompt.txt")
	if err == nil {
		t.Error("Launch() with nonexistent prompt file should return error, got nil")
	}
}

// TestCommandFileRef verifies Command builds exec.Cmd with FileRef delivery arguments
func TestCommandFileRef(t *testing.T) {
	// Expected failure: Command() method does not exist on Agent interface yet
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	agent := New("test", "echo", []string{"--flag1"}, FileRef, "", []string{"--extra"})

	cmd, err := agent.Command(promptPath)
	if err != nil {
		t.Fatalf("Command() error = %v", err)
	}

	if cmd == nil {
		t.Fatal("Command() returned nil cmd")
	}

	// Verify command is not yet started
	if cmd.Process != nil {
		t.Error("Command() should return a command that has not been started yet")
	}

	// Verify the command args contain the file reference message
	expectedMsg := fmt.Sprintf(fileRefMessageFormat, promptPath)
	found := false
	for _, arg := range cmd.Args {
		if arg == expectedMsg {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Command() args = %v, expected to contain file reference message %q", cmd.Args, expectedMsg)
	}

	// Verify flags are included
	foundFlag := false
	for _, arg := range cmd.Args {
		if arg == "--flag1" {
			foundFlag = true
			break
		}
	}
	if !foundFlag {
		t.Errorf("Command() args = %v, expected to contain flag --flag1", cmd.Args)
	}

	// Verify extra args are included
	foundExtra := false
	for _, arg := range cmd.Args {
		if arg == "--extra" {
			foundExtra = true
			break
		}
	}
	if !foundExtra {
		t.Errorf("Command() args = %v, expected to contain extra arg --extra", cmd.Args)
	}
}

// TestCommandPromptFileArg verifies Command builds exec.Cmd with PromptFileArg delivery
func TestCommandPromptFileArg(t *testing.T) {
	// Expected failure: Command() method does not exist on Agent interface yet
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	agent := New("test", "cat", []string{"--verbose"}, PromptFileArg, "--prompt-file", []string{"--output=json"})

	cmd, err := agent.Command(promptPath)
	if err != nil {
		t.Fatalf("Command() error = %v", err)
	}

	if cmd == nil {
		t.Fatal("Command() returned nil cmd")
	}

	// Verify command is not yet started
	if cmd.Process != nil {
		t.Error("Command() should return a command that has not been started yet")
	}

	// Verify the prompt flag and path are in the args
	foundPromptFlag := false
	foundPromptPath := false
	for i, arg := range cmd.Args {
		if arg == "--prompt-file" {
			foundPromptFlag = true
			// Check if next arg is the prompt path
			if i+1 < len(cmd.Args) && cmd.Args[i+1] == promptPath {
				foundPromptPath = true
			}
		}
	}
	if !foundPromptFlag {
		t.Errorf("Command() args = %v, expected to contain --prompt-file flag", cmd.Args)
	}
	if !foundPromptPath {
		t.Errorf("Command() args = %v, expected --prompt-file to be followed by %q", cmd.Args, promptPath)
	}

	// Verify flags and extra args are included
	foundFlag := false
	foundExtra := false
	for _, arg := range cmd.Args {
		if arg == "--verbose" {
			foundFlag = true
		}
		if arg == "--output=json" {
			foundExtra = true
		}
	}
	if !foundFlag {
		t.Errorf("Command() args = %v, expected to contain --verbose", cmd.Args)
	}
	if !foundExtra {
		t.Errorf("Command() args = %v, expected to contain --output=json", cmd.Args)
	}
}

// TestCommandStdin verifies Command builds exec.Cmd for Stdin delivery WITHOUT setting up pipe
func TestCommandStdin(t *testing.T) {
	// Expected failure: Command() method does not exist on Agent interface yet
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test prompt content"), 0644); err != nil {
		t.Fatal(err)
	}

	agent := New("test", "cat", []string{"-n"}, Stdin, "", nil)

	cmd, err := agent.Command(promptPath)
	if err != nil {
		t.Fatalf("Command() error = %v", err)
	}

	if cmd == nil {
		t.Fatal("Command() returned nil cmd")
	}

	// Verify command is not yet started
	if cmd.Process != nil {
		t.Error("Command() should return a command that has not been started yet")
	}

	// CRITICAL: Verify that stdin pipe is NOT set up for Stdin delivery mode
	// The task description explicitly states: "For Stdin delivery, do NOT set up the pipe"
	// The caller is responsible for setting up stdin when using Command()
	if cmd.Stdin != nil {
		t.Error("Command() with Stdin delivery should NOT set up the stdin pipe - caller is responsible for stdin setup")
	}

	// Verify args only contain flags, not prompt content or file references
	for _, arg := range cmd.Args {
		if arg == promptPath {
			t.Errorf("Command() args should not contain prompt path for Stdin delivery, got %v", cmd.Args)
		}
		if arg == fmt.Sprintf(fileRefMessageFormat, promptPath) {
			t.Errorf("Command() args should not contain file reference for Stdin delivery, got %v", cmd.Args)
		}
	}

	// Verify -n flag is present
	foundFlag := false
	for _, arg := range cmd.Args {
		if arg == "-n" {
			foundFlag = true
			break
		}
	}
	if !foundFlag {
		t.Errorf("Command() args = %v, expected to contain -n flag", cmd.Args)
	}
}

// TestCommandNonexistentPromptFile verifies Command returns error for missing prompt file
func TestCommandNonexistentPromptFile(t *testing.T) {
	// Expected failure: Command() method does not exist on Agent interface yet
	agent := New("test", "echo", nil, FileRef, "", nil)

	cmd, err := agent.Command("/nonexistent/path/to/prompt.txt")
	if err == nil {
		t.Error("Command() with nonexistent prompt file should return error, got nil")
	}
	if cmd != nil {
		t.Errorf("Command() with nonexistent prompt file should return nil cmd, got %v", cmd)
	}
}

// TestCommandReturnsConfiguredCmd verifies Command builds a properly configured exec.Cmd
func TestCommandReturnsConfiguredCmd(t *testing.T) {
	// Expected failure: Command() method does not exist on Agent interface yet
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	agent := New("test", "echo", []string{"arg1", "arg2"}, FileRef, "", []string{"arg3"})

	cmd, err := agent.Command(promptPath)
	if err != nil {
		t.Fatalf("Command() error = %v", err)
	}

	// Verify the command can be started by the caller
	// This proves Command() returns a valid, unstarted *exec.Cmd
	if cmd.Path == "" {
		t.Error("Command() returned cmd with empty Path")
	}

	// Verify Args are set correctly (binary name should be first arg)
	if len(cmd.Args) == 0 {
		t.Fatal("Command() returned cmd with no Args")
	}
	if cmd.Args[0] != "echo" {
		t.Errorf("Command() Args[0] = %q, expected %q", cmd.Args[0], "echo")
	}

	// Verify this is a fresh command (not started, no output set yet)
	if cmd.ProcessState != nil {
		t.Error("Command() should return a fresh command with no ProcessState")
	}
}

// TestCommandWithAllThreePromptDeliveryModes verifies Command handles all delivery modes correctly
func TestCommandWithAllThreePromptDeliveryModes(t *testing.T) {
	// Expected failure: Command() method does not exist on Agent interface yet
	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		delivery    PromptDelivery
		promptFlag  string
		verifyArgs  func(t *testing.T, args []string, promptPath string)
		verifyStdin func(t *testing.T, stdin interface{})
	}{
		{
			name:       "FileRef delivery includes file reference message",
			delivery:   FileRef,
			promptFlag: "",
			verifyArgs: func(t *testing.T, args []string, promptPath string) {
				expectedMsg := fmt.Sprintf(fileRefMessageFormat, promptPath)
				found := false
				for _, arg := range args {
					if arg == expectedMsg {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Args should contain file reference message %q, got %v", expectedMsg, args)
				}
			},
			verifyStdin: func(t *testing.T, stdin interface{}) {
				// FileRef does not set up stdin
				if stdin != nil {
					t.Error("FileRef delivery should not set stdin")
				}
			},
		},
		{
			name:       "PromptFileArg delivery includes prompt flag and path",
			delivery:   PromptFileArg,
			promptFlag: "--prompt",
			verifyArgs: func(t *testing.T, args []string, promptPath string) {
				foundFlag := false
				foundPath := false
				for i, arg := range args {
					if arg == "--prompt" {
						foundFlag = true
						if i+1 < len(args) && args[i+1] == promptPath {
							foundPath = true
						}
					}
				}
				if !foundFlag || !foundPath {
					t.Errorf("Args should contain --prompt followed by path, got %v", args)
				}
			},
			verifyStdin: func(t *testing.T, stdin interface{}) {
				// PromptFileArg does not set up stdin
				if stdin != nil {
					t.Error("PromptFileArg delivery should not set stdin")
				}
			},
		},
		{
			name:       "Stdin delivery does NOT set up stdin pipe",
			delivery:   Stdin,
			promptFlag: "",
			verifyArgs: func(t *testing.T, args []string, promptPath string) {
				// Stdin delivery should not add prompt to args
				for _, arg := range args {
					if arg == promptPath {
						t.Errorf("Stdin delivery should not include prompt path in args, got %v", args)
					}
				}
			},
			verifyStdin: func(t *testing.T, stdin interface{}) {
				// CRITICAL: Command() with Stdin delivery should NOT set up pipe
				// This is the key behavioral difference from Launch()
				if stdin != nil {
					t.Error("Command() with Stdin delivery should NOT set up stdin pipe - caller is responsible")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := New("test", "echo", nil, tt.delivery, tt.promptFlag, nil)

			cmd, err := agent.Command(promptPath)
			if err != nil {
				t.Fatalf("Command() error = %v", err)
			}
			if cmd == nil {
				t.Fatal("Command() returned nil")
			}

			// Verify command not started
			if cmd.Process != nil {
				t.Error("Command() should return unstarted command")
			}

			// Run delivery-specific arg verification
			tt.verifyArgs(t, cmd.Args, promptPath)

			// Run delivery-specific stdin verification
			tt.verifyStdin(t, cmd.Stdin)
		})
	}
}
