package contracts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	env.Env = replaceOrAppend(env.Env, "CLAUDE_FIXTURE", fixtureFile)

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
	env.Env = replaceOrAppend(env.Env, "BD_FAIL", "1")

	cmd := exec.Command(filepath.Join(fakesDir, "bd"), "ready", "--json", "--limit", "1")
	cmd.Dir = env.Dir
	cmd.Env = env.Env

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected bd to fail with BD_FAIL=1, but it succeeded. Output: %s", output)
	}

	// Test GIT_FAIL error mode
	env.Env = replaceOrAppend(env.Env, "BD_FAIL", "0")
	env.Env = replaceOrAppend(env.Env, "GIT_FAIL", "1")

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

	env.Env = replaceOrAppend(env.Env, "GIT_FAIL", "0")
	env.Env = replaceOrAppend(env.Env, "CLAUDE_FAIL", "1")
	env.Env = replaceOrAppend(env.Env, "CLAUDE_FIXTURE", fixtureFile)

	cmd = exec.Command(filepath.Join(fakesDir, "claude"), "-p", "test")
	cmd.Dir = env.Dir
	cmd.Env = env.Env
	cmd.Stdin = strings.NewReader("test\n")

	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected claude to fail with CLAUDE_FAIL=1, but it succeeded. Output: %s", output)
	}
}
