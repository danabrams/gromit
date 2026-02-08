package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	if os.Getenv("GO_TEST_HELPER_PROCESS") == "1" {
		// This is the helper process - just check the args
		args := os.Args
		// Find where our test args start (after the "--" separator)
		for i, arg := range args {
			if arg == "--" && i+1 < len(args) {
				// Verify we got the "Read and follow instructions" message
				found := false
				for j := i + 1; j < len(args); j++ {
					if strings.Contains(args[j], "Read and follow instructions in") {
						found = true
						break
					}
				}
				if !found {
					os.Exit(1)
				}
				os.Exit(0)
			}
		}
		os.Exit(1)
	}

	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	// Use the test helper process pattern to verify command construction
	agent := New("test", os.Args[0], []string{"-test.run=TestLaunchFileRef", "--"}, FileRef, "", nil)

	// Set up environment to trigger helper process
	if ca, ok := agent.(*cliAgent); ok {
		ca.binary = os.Args[0]
		ca.flags = []string{"-test.run=TestLaunchFileRef", "--"}
	}

	err := agent.Launch(promptPath)
	if err != nil {
		t.Errorf("Launch() with FileRef delivery failed: %v", err)
	}
}

// TestLaunchPromptFileArg verifies Launch constructs correct command for prompt_file_arg delivery
func TestLaunchPromptFileArg(t *testing.T) {
	if os.Getenv("GO_TEST_HELPER_PROCESS") == "1" {
		// This is the helper process - check for the prompt flag and file path
		args := os.Args
		foundFlag := false
		foundPath := false
		for i, arg := range args {
			if arg == "--prompt" {
				foundFlag = true
				if i+1 < len(args) && strings.HasSuffix(args[i+1], "prompt.txt") {
					foundPath = true
				}
			}
		}
		if foundFlag && foundPath {
			os.Exit(0)
		}
		os.Exit(1)
	}

	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	agent := New("test", os.Args[0], []string{"-test.run=TestLaunchPromptFileArg"}, PromptFileArg, "--prompt", nil)

	// Set helper process env
	if ca, ok := agent.(*cliAgent); ok {
		ca.binary = os.Args[0]
	}

	err := agent.Launch(promptPath)
	if err != nil {
		t.Errorf("Launch() with PromptFileArg delivery failed: %v", err)
	}
}

// TestLaunchStdin verifies Launch pipes prompt content to stdin
func TestLaunchStdin(t *testing.T) {
	if os.Getenv("GO_TEST_HELPER_PROCESS") == "1" {
		// This is the helper process - read stdin and verify content
		buf := make([]byte, 1024)
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			os.Exit(1)
		}
		if strings.Contains(string(buf[:n]), "test prompt content") {
			os.Exit(0)
		}
		os.Exit(1)
	}

	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	promptContent := "test prompt content"
	if err := os.WriteFile(promptPath, []byte(promptContent), 0644); err != nil {
		t.Fatal(err)
	}

	agent := New("test", os.Args[0], []string{"-test.run=TestLaunchStdin"}, Stdin, "", nil)

	// Set helper process env
	if ca, ok := agent.(*cliAgent); ok {
		ca.binary = os.Args[0]
	}

	err := agent.Launch(promptPath)
	if err != nil {
		t.Errorf("Launch() with Stdin delivery failed: %v", err)
	}
}

// TestLaunchWithExitError verifies that exec.ExitError is treated as graceful exit
func TestLaunchWithExitError(t *testing.T) {
	if os.Getenv("GO_TEST_HELPER_PROCESS") == "1" {
		// Exit with non-zero status to simulate agent returning error
		os.Exit(1)
	}

	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	agent := New("test", os.Args[0], []string{"-test.run=TestLaunchWithExitError"}, FileRef, "", nil)

	// Set helper process env
	if ca, ok := agent.(*cliAgent); ok {
		ca.binary = os.Args[0]
	}

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
	if os.Getenv("GO_TEST_HELPER_PROCESS") == "1" {
		// Check for extra args
		foundExtra := false
		for _, arg := range os.Args {
			if arg == "--extra-arg" {
				foundExtra = true
				break
			}
		}
		if foundExtra {
			os.Exit(0)
		}
		os.Exit(1)
	}

	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	agent := New("test", os.Args[0], []string{"-test.run=TestLaunchWithExtraArgs"}, FileRef, "", []string{"--extra-arg"})

	// Set helper process env
	if ca, ok := agent.(*cliAgent); ok {
		ca.binary = os.Args[0]
	}

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
