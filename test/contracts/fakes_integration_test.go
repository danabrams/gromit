//go:build contract

package contracts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
)

// TestFakes_GitPassthrough verifies that the fake git script records calls
// and passes through to real git
func TestFakes_GitPassthrough(t *testing.T) {
	env := setupTestEnv(t)

	// Run a git command directly using the fake
	cmd := exec.Command(filepath.Join(fakesDir, "git"), "status")
	cmd.Dir = env.Dir
	cmd.Env = env.Env

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status failed: %v\nOutput: %s", err, output)
	}

	// Verify git status succeeded (should show "On branch" in output)
	outputStr := string(output)
	if !strings.Contains(outputStr, "On branch") && !strings.Contains(outputStr, "No commits yet") {
		t.Errorf("Expected git status output, got: %s", outputStr)
	}

	// Verify the call was logged
	calls, err := filterCalls(env, "git")
	if err != nil {
		t.Fatalf("filterCalls failed: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("Expected 1 git call, got %d: %v", len(calls), calls)
	}

	if calls[0] != "git status" {
		t.Errorf("Expected 'git status', got %q", calls[0])
	}
}

// TestFakes_CodexFixtureModes verifies that fake codex supports fixture-driven
// plain output and JSONL output modes while consuming stdin and logging calls.
func TestFakes_CodexFixtureModes(t *testing.T) {
	env := setupTestEnv(t)

	fixtureContent := "Codex fixture output"
	fixtureFile := filepath.Join(env.Dir, "codex_fixture.txt")
	if err := os.WriteFile(fixtureFile, []byte(fixtureContent), 0644); err != nil {
		t.Fatalf("Failed to write fixture file: %v", err)
	}

	tests := []struct {
		name                string
		args                []string
		expectedOutputCheck func(t *testing.T, output string)
	}{
		{
			name: "plain mode",
			args: []string{"run", "--model", "sonnet"},
			expectedOutputCheck: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, fixtureContent) {
					t.Fatalf("Expected fixture content %q in plain output, got: %s", fixtureContent, output)
				}
			},
		},
		{
			name: "jsonl mode",
			args: []string{"run", "--jsonl", "--model", "sonnet"},
			expectedOutputCheck: func(t *testing.T, output string) {
				t.Helper()
				if !strings.Contains(output, "\"type\":\"assistant\"") {
					t.Fatalf("Expected JSONL assistant event, got: %s", output)
				}
				if !strings.Contains(output, fixtureContent) {
					t.Fatalf("Expected JSONL output to include fixture content %q, got: %s", fixtureContent, output)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testEnv := codexTestEnvWithFixture(env.Env, fixtureFile)
			cmd := newCodexFakeCommand(env.Dir, testEnv, tt.args...)
			cmd.Stdin = strings.NewReader("acceptance prompt input\n")

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("codex failed in %s: %v\nOutput: %s", tt.name, err, output)
			}

			outputStr := string(output)
			tt.expectedOutputCheck(t, outputStr)

			calls, err := filterCalls(env, "codex")
			if err != nil {
				t.Fatalf("filterCalls failed: %v", err)
			}
			if len(calls) == 0 {
				t.Fatalf("Expected codex call to be logged, got none")
			}
			expectedCall := fmt.Sprintf("codex %s", strings.Join(tt.args, " "))
			if calls[len(calls)-1] != expectedCall {
				t.Fatalf("Expected call %q, got %q", expectedCall, calls[len(calls)-1])
			}
		})
	}
}

// TestFakes_CodexErrorAndDelayModes verifies that fake codex can simulate
// non-zero failures and optional delay behavior.
func TestFakes_CodexErrorAndDelayModes(t *testing.T) {
	env := setupTestEnv(t)

	fixtureFile := filepath.Join(env.Dir, "codex_fixture.txt")
	if err := os.WriteFile(fixtureFile, []byte("Codex fixture output"), 0644); err != nil {
		t.Fatalf("Failed to write fixture file: %v", err)
	}

	t.Run("non-zero failure", func(t *testing.T) {
		testEnv := codexTestEnvWithFixture(env.Env, fixtureFile)
		testEnv = testutil.ReplaceOrAppend(testEnv, codexFailureExitCodeEnvVar, "23")

		cmd := newCodexFakeCommand(env.Dir, testEnv, "run", "--model", "sonnet")
		cmd.Stdin = strings.NewReader("prompt\n")

		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("Expected codex to fail with CODEX_FAIL=23, output: %s", output)
		}
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("Expected *exec.ExitError, got %T", err)
		}
		if exitErr.ExitCode() != 23 {
			t.Fatalf("Expected exit code 23, got %d (output: %s)", exitErr.ExitCode(), output)
		}
	})

	t.Run("delay", func(t *testing.T) {
		testEnv := codexTestEnvWithFixture(env.Env, fixtureFile)
		testEnv = testutil.ReplaceOrAppend(testEnv, codexDelayEnvVar, "0.2")

		cmd := newCodexFakeCommand(env.Dir, testEnv, "run", "--model", "sonnet")
		cmd.Stdin = strings.NewReader("prompt\n")

		start := testNowUnixMilli()
		output, err := cmd.CombinedOutput()
		elapsed := testNowUnixMilli() - start

		if err != nil {
			t.Fatalf("Expected delayed codex command to succeed, got %v (output: %s)", err, output)
		}
		if elapsed < 150 {
			t.Fatalf("Expected CODEX_DELAY to add latency; elapsed=%dms output=%s", elapsed, output)
		}
	})
}

// TestFakes_CodexRequiresFixture verifies that fake codex fails with a clear
// error when CODEX_FIXTURE is not configured.
func TestFakes_CodexRequiresFixture(t *testing.T) {
	env := setupTestEnv(t)

	cmd := newCodexFakeCommand(env.Dir, env.Env, "run", "--model", "sonnet")
	cmd.Stdin = strings.NewReader("prompt\n")

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("Expected codex to fail when CODEX_FIXTURE is unset, output: %s", output)
	}
	if !strings.Contains(string(output), codexFixtureRequiredErrorToken) {
		t.Fatalf("Expected error output to mention CODEX_FIXTURE, got: %s", output)
	}
}

// TestFakes_BDStateful verifies that the fake bd maintains state
func TestFakes_BDStateful(t *testing.T) {
	env := setupTestEnv(t)

	// Create a bead
	cmd := exec.Command(filepath.Join(fakesDir, "bd"), "create", "Test task", "--priority", "1", "--json")
	cmd.Dir = env.Dir
	cmd.Env = env.Env

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bd create failed: %v\nOutput: %s", err, output)
	}

	// bd ready should return the created bead
	cmd = exec.Command(filepath.Join(fakesDir, "bd"), "ready", "--json", "--limit", "1")
	cmd.Dir = env.Dir
	cmd.Env = env.Env

	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bd ready failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "test-bead-1") {
		t.Errorf("Expected bead test-bead-1 in output, got: %s", outputStr)
	}

	// Close the bead
	cmd = exec.Command(filepath.Join(fakesDir, "bd"), "close", "test-bead-1")
	cmd.Dir = env.Dir
	cmd.Env = env.Env

	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bd close failed: %v\nOutput: %s", err, output)
	}

	// bd ready should now return empty
	cmd = exec.Command(filepath.Join(fakesDir, "bd"), "ready", "--json", "--limit", "1")
	cmd.Dir = env.Dir
	cmd.Env = env.Env

	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bd ready failed: %v\nOutput: %s", err, output)
	}

	outputStr = string(output)
	if outputStr != "[]\n" {
		t.Errorf("Expected empty array after closing bead, got: %s", outputStr)
	}

	// Verify all calls were logged
	calls, err := filterCalls(env, "bd")
	if err != nil {
		t.Fatalf("filterCalls failed: %v", err)
	}

	expectedCalls := []string{
		"bd create Test task --priority 1 --json",
		"bd ready --json --limit 1",
		"bd close test-bead-1",
		"bd ready --json --limit 1",
	}

	if len(calls) != len(expectedCalls) {
		t.Errorf("Expected %d bd calls, got %d: %v", len(expectedCalls), len(calls), calls)
	}
}

// TestFakes_ClaudeFixture verifies that the fake claude returns fixture content
func TestFakes_ClaudeFixture(t *testing.T) {
	env := setupTestEnv(t)

	// Set CLAUDE_FIXTURE to a test fixture
	fixtureContent := "Test output from Claude"
	fixtureFile := filepath.Join(env.Dir, "test_fixture.txt")
	if err := os.WriteFile(fixtureFile, []byte(fixtureContent), 0644); err != nil {
		t.Fatalf("Failed to write fixture file: %v", err)
	}

	// Add CLAUDE_FIXTURE to environment
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE", fixtureFile)

	// Run fake claude (need to provide stdin)
	cmd := exec.Command(filepath.Join(fakesDir, "claude"), "-p", "test-prompt", "--model", "sonnet")
	cmd.Dir = env.Dir
	cmd.Env = env.Env
	cmd.Stdin = strings.NewReader("test prompt content\n")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("claude failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, fixtureContent) {
		t.Errorf("Expected fixture content %q in output, got: %s", fixtureContent, outputStr)
	}

	// Verify the call was logged
	calls, err := filterCalls(env, "claude")
	if err != nil {
		t.Fatalf("filterCalls failed: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("Expected 1 claude call, got %d: %v", len(calls), calls)
	}

	expectedCall := "claude -p test-prompt --model sonnet"
	if calls[0] != expectedCall {
		t.Errorf("Expected %q, got %q", expectedCall, calls[0])
	}
}

// TestFakes_ErrorModes verifies that fake CLIs can simulate failures
func TestFakes_ErrorModes(t *testing.T) {
	env := setupTestEnv(t)

	// Test BD_FAIL error mode
	env.Env = testutil.ReplaceOrAppend(env.Env, "BD_FAIL", "1")

	cmd := exec.Command(filepath.Join(fakesDir, "bd"), "ready", "--json", "--limit", "1")
	cmd.Dir = env.Dir
	cmd.Env = env.Env

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected bd to fail with BD_FAIL=1, but it succeeded. Output: %s", output)
	}

	// Test GIT_FAIL error mode
	env.Env = testutil.ReplaceOrAppend(env.Env, "BD_FAIL", "0")
	env.Env = testutil.ReplaceOrAppend(env.Env, "GIT_FAIL", "1")

	cmd = exec.Command(filepath.Join(fakesDir, "git"), "status")
	cmd.Dir = env.Dir
	cmd.Env = env.Env

	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected git to fail with GIT_FAIL=1, but it succeeded. Output: %s", output)
	}

	// Test CLAUDE_FAIL error mode
	fixtureFile := filepath.Join(env.Dir, "test_fixture.txt")
	if err := os.WriteFile(fixtureFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write fixture file: %v", err)
	}

	env.Env = testutil.ReplaceOrAppend(env.Env, "GIT_FAIL", "0")
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FAIL", "1")
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE", fixtureFile)

	cmd = exec.Command(filepath.Join(fakesDir, "claude"), "-p", "test")
	cmd.Dir = env.Dir
	cmd.Env = env.Env
	cmd.Stdin = strings.NewReader("test\n")

	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected claude to fail with CLAUDE_FAIL=1, but it succeeded. Output: %s", output)
	}
}

// TestFakes_ClaudeWriteFile verifies that the fake claude creates files via CLAUDE_WRITE_FILE
func TestFakes_ClaudeWriteFile(t *testing.T) {
	env := setupTestEnv(t)

	// Set up fixture file
	fixtureContent := "Test output from Claude"
	fixtureFile := filepath.Join(env.Dir, "test_fixture.txt")
	if err := os.WriteFile(fixtureFile, []byte(fixtureContent), 0644); err != nil {
		t.Fatalf("Failed to write fixture file: %v", err)
	}

	// Set up file to be created
	targetFile := filepath.Join(env.Dir, "nested", "dir", "created_file.txt")
	fileContent := "This is the file content"

	// Add environment variables
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE", fixtureFile)
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_WRITE_FILE", targetFile)
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_WRITE_CONTENT", fileContent)

	// Run fake claude
	cmd := exec.Command(filepath.Join(fakesDir, "claude"), "-p", "test-prompt")
	cmd.Dir = env.Dir
	cmd.Env = env.Env
	cmd.Stdin = strings.NewReader("test prompt content\n")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("claude failed: %v\nOutput: %s", err, output)
	}

	// Verify fixture output
	outputStr := string(output)
	if !strings.Contains(outputStr, fixtureContent) {
		t.Errorf("Expected fixture content %q in output, got: %s", fixtureContent, outputStr)
	}

	// Verify file was created
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Errorf("Expected file %s to be created, but it doesn't exist", targetFile)
	}

	// Verify file content
	createdContent, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read created file: %v", err)
	}

	// Note: echo adds a newline, so we need to account for that
	expectedContent := fileContent + "\n"
	if string(createdContent) != expectedContent {
		t.Errorf("Expected file content %q, got %q", expectedContent, string(createdContent))
	}

	// Verify parent directories were created
	parentDir := filepath.Dir(targetFile)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		t.Errorf("Expected parent directory %s to be created, but it doesn't exist", parentDir)
	}
}

// TestFakes_ClaudeWriteFile_MissingTestDir verifies validation when TEST_DIR is not set
func TestFakes_ClaudeWriteFile_MissingTestDir(t *testing.T) {
	env := setupTestEnv(t)

	// Set up fixture file
	fixtureContent := "Test output"
	fixtureFile := filepath.Join(env.Dir, "test_fixture.txt")
	if err := os.WriteFile(fixtureFile, []byte(fixtureContent), 0644); err != nil {
		t.Fatalf("Failed to write fixture file: %v", err)
	}

	targetFile := filepath.Join(env.Dir, "created_file.txt")

	// Set CLAUDE_WRITE_FILE without TEST_DIR
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE", fixtureFile)
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_WRITE_FILE", targetFile)
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_WRITE_CONTENT", "content")
	// Remove TEST_DIR from environment
	env.Env = testutil.RemoveEnvVar(env.Env, "TEST_DIR")

	// Run fake claude - should fail
	cmd := exec.Command(filepath.Join(fakesDir, "claude"), "-p", "test")
	cmd.Dir = env.Dir
	cmd.Env = env.Env
	cmd.Stdin = strings.NewReader("test\n")

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected claude to fail without TEST_DIR, but it succeeded. Output: %s", output)
	}

	// Verify error message mentions TEST_DIR
	outputStr := string(output)
	if !strings.Contains(outputStr, "TEST_DIR") {
		t.Errorf("Expected error message to mention TEST_DIR, got: %s", outputStr)
	}
}

// TestFakes_ClaudeWriteFile_RelativePath verifies validation when path is relative
func TestFakes_ClaudeWriteFile_RelativePath(t *testing.T) {
	env := setupTestEnv(t)

	// Set up fixture file
	fixtureContent := "Test output"
	fixtureFile := filepath.Join(env.Dir, "test_fixture.txt")
	if err := os.WriteFile(fixtureFile, []byte(fixtureContent), 0644); err != nil {
		t.Fatalf("Failed to write fixture file: %v", err)
	}

	// Use a relative path
	relativePath := "relative/path/file.txt"

	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE", fixtureFile)
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_WRITE_FILE", relativePath)
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_WRITE_CONTENT", "content")

	// Run fake claude - should fail
	cmd := exec.Command(filepath.Join(fakesDir, "claude"), "-p", "test")
	cmd.Dir = env.Dir
	cmd.Env = env.Env
	cmd.Stdin = strings.NewReader("test\n")

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected claude to fail with relative path, but it succeeded. Output: %s", output)
	}

	// Verify error message mentions absolute path
	outputStr := string(output)
	if !strings.Contains(outputStr, "absolute path") {
		t.Errorf("Expected error message to mention 'absolute path', got: %s", outputStr)
	}
}

// TestFakes_ClaudeWriteFile_PathTraversal verifies validation blocks path traversal
func TestFakes_ClaudeWriteFile_PathTraversal(t *testing.T) {
	env := setupTestEnv(t)

	// Set up fixture file
	fixtureContent := "Test output"
	fixtureFile := filepath.Join(env.Dir, "test_fixture.txt")
	if err := os.WriteFile(fixtureFile, []byte(fixtureContent), 0644); err != nil {
		t.Fatalf("Failed to write fixture file: %v", err)
	}

	// Try to escape TEST_DIR using ..
	escapePath := filepath.Join(env.Dir, "..", "..", "etc", "passwd")

	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE", fixtureFile)
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_WRITE_FILE", escapePath)
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_WRITE_CONTENT", "malicious content")

	// Run fake claude - should fail
	cmd := exec.Command(filepath.Join(fakesDir, "claude"), "-p", "test")
	cmd.Dir = env.Dir
	cmd.Env = env.Env
	cmd.Stdin = strings.NewReader("test\n")

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected claude to fail with path traversal, but it succeeded. Output: %s", output)
	}

	// Verify error message mentions path traversal or outside TEST_DIR
	outputStr := string(output)
	if !strings.Contains(outputStr, "outside TEST_DIR") && !strings.Contains(outputStr, "path traversal") {
		t.Errorf("Expected error message to mention path containment, got: %s", outputStr)
	}
}

// TestFakes_ClaudeWriteFile_BackslashContent verifies content with literal backslash-n
// is written without escape interpretation. Uses raw string literal for test input.
func TestFakes_ClaudeWriteFile_BackslashContent(t *testing.T) {
	env := setupTestEnv(t)

	// Set up fixture file
	fixtureContent := "Test output"
	fixtureFile := filepath.Join(env.Dir, "test_fixture.txt")
	if err := os.WriteFile(fixtureFile, []byte(fixtureContent), 0644); err != nil {
		t.Fatalf("Failed to write fixture file: %v", err)
	}

	targetFile := filepath.Join(env.Dir, "backslash_test.txt")
	// Raw string literal containing literal backslash-n sequence
	content := `hello\nworld`

	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE", fixtureFile)
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_WRITE_FILE", targetFile)
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_WRITE_CONTENT", content)

	// Run fake claude
	cmd := exec.Command(filepath.Join(fakesDir, "claude"), "-p", "test")
	cmd.Dir = env.Dir
	cmd.Env = env.Env
	cmd.Stdin = strings.NewReader("test\n")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("claude failed: %v\nOutput: %s", err, output)
	}

	// Verify file content contains literal backslash-n, not interpreted as newline
	createdContent, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read created file: %v", err)
	}

	// printf '%s\n' adds a trailing newline
	expectedContent := content + "\n"
	if string(createdContent) != expectedContent {
		t.Errorf("Content not written literally:\n  expected: %q\n  got:      %q", expectedContent, string(createdContent))
	}

	// Explicitly verify the backslash-n is literal, not a newline character
	if strings.Contains(string(createdContent), "hello\nworld") {
		t.Errorf("Content was incorrectly interpreted: backslash-n became actual newline")
	}
}

// TestFakes_ClaudeWriteFile_ContentFidelity verifies content is written exactly as provided
func TestFakes_ClaudeWriteFile_ContentFidelity(t *testing.T) {
	env := setupTestEnv(t)

	// Set up fixture file
	fixtureContent := "Test output"
	fixtureFile := filepath.Join(env.Dir, "test_fixture.txt")
	if err := os.WriteFile(fixtureFile, []byte(fixtureContent), 0644); err != nil {
		t.Fatalf("Failed to write fixture file: %v", err)
	}

	// Test content with backslashes and special characters that echo might interpret
	testCases := []struct {
		name    string
		content string
	}{
		{"backslash-n", "hello\\nworld"},
		{"backslash-t", "hello\\tworld"},
		{"leading-hyphen", "-e test"},
		{"multiple-backslashes", "path\\to\\file"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			targetFile := filepath.Join(env.Dir, tc.name+".txt")

			env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE", fixtureFile)
			env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_WRITE_FILE", targetFile)
			env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_WRITE_CONTENT", tc.content)

			// Run fake claude
			cmd := exec.Command(filepath.Join(fakesDir, "claude"), "-p", "test")
			cmd.Dir = env.Dir
			cmd.Env = env.Env
			cmd.Stdin = strings.NewReader("test\n")

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("claude failed: %v\nOutput: %s", err, output)
			}

			// Verify file content is literal (not interpreted)
			createdContent, err := os.ReadFile(targetFile)
			if err != nil {
				t.Fatalf("Failed to read created file: %v", err)
			}

			// printf '%s\n' adds a trailing newline
			expectedContent := tc.content + "\n"
			if string(createdContent) != expectedContent {
				t.Errorf("Content not written literally:\n  expected: %q\n  got:      %q", expectedContent, string(createdContent))
			}
		})
	}
}

// TestFakes_ClaudeFailOnceRequiresTestDir verifies that CLAUDE_FAIL_<MODEL>_ONCE requires TEST_DIR
func TestFakes_ClaudeFailOnceRequiresTestDir(t *testing.T) {
	env := setupTestEnv(t)

	// Set up fixture file
	fixtureContent := "Test output"
	fixtureFile := filepath.Join(env.Dir, "test_fixture.txt")
	if err := os.WriteFile(fixtureFile, []byte(fixtureContent), 0644); err != nil {
		t.Fatalf("Failed to write fixture file: %v", err)
	}

	// Set CLAUDE_FAIL_HAIKU_ONCE without TEST_DIR
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE_HAIKU", fixtureFile)
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FAIL_HAIKU_ONCE", "1")
	// Remove TEST_DIR from environment
	env.Env = testutil.RemoveEnvVar(env.Env, "TEST_DIR")

	// Run fake claude with stream-json (to trigger the once-mode logic)
	cmd := exec.Command(filepath.Join(fakesDir, "claude"), "stream-json", "-p", "test", "--model", "haiku")
	cmd.Dir = env.Dir
	cmd.Env = env.Env
	cmd.Stdin = strings.NewReader("test\n")

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected claude to fail without TEST_DIR when using CLAUDE_FAIL_HAIKU_ONCE, but it succeeded. Output: %s", output)
	}

	// Verify error message mentions TEST_DIR
	outputStr := string(output)
	if !strings.Contains(outputStr, "TEST_DIR") {
		t.Errorf("Expected error message to mention TEST_DIR, got: %s", outputStr)
	}
}

// TestFakes_ClaudeFailOnceStateFileCleanup verifies that state files don't persist between tests
func TestFakes_ClaudeFailOnceStateFileCleanup(t *testing.T) {
	env := setupTestEnv(t)

	// Set up fixture file
	fixtureContent := "Test output"
	fixtureFile := filepath.Join(env.Dir, "test_fixture.txt")
	if err := os.WriteFile(fixtureFile, []byte(fixtureContent), 0644); err != nil {
		t.Fatalf("Failed to write fixture file: %v", err)
	}

	// Set CLAUDE_FAIL_HAIKU_ONCE with TEST_DIR
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE_HAIKU", fixtureFile)
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FAIL_HAIKU_ONCE", "1")

	stateFile := filepath.Join(env.Dir, ".claude_fail_haiku_once_state")

	// Before running: state file should not exist
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Errorf("Expected state file to not exist before test, but it does")
	}

	// Run fake claude with stream-json (first invocation should create state file)
	cmd := exec.Command(filepath.Join(fakesDir, "claude"), "stream-json", "-p", "test", "--model", "haiku")
	cmd.Dir = env.Dir
	cmd.Env = env.Env
	cmd.Stdin = strings.NewReader("test\n")

	_, _ = cmd.CombinedOutput()

	// After running: state file should exist
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Errorf("Expected state file to exist after first invocation, but it doesn't")
	}

	// Verify that cleanup happens in test teardown (this will be checked after test returns)
	// The test framework will call os.RemoveAll(tmpDir) which removes the entire directory
	// including the state file, and then cleanupClaudeFailOnceStateFiles is also called
}
