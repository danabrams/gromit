package contracts

import (
	"os"
	"path/filepath"
	"testing"
)

// TestHelpers_setupTestEnv verifies that setupTestEnv creates a valid test environment
func TestHelpers_setupTestEnv(t *testing.T) {
	env := setupTestEnv(t)

	// Verify temp directory exists
	if _, err := os.Stat(env.Dir); os.IsNotExist(err) {
		t.Errorf("Test directory does not exist: %s", env.Dir)
	}

	// Verify it's a git repository
	gitDir := filepath.Join(env.Dir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error("Test directory is not a git repository")
	}

	// Verify call log path is set
	if env.CallLog == "" {
		t.Error("CallLog path is empty")
	}

	// Verify bd state file path is set
	if env.BDStateFile == "" {
		t.Error("BDStateFile path is empty")
	}

	// Verify PATH includes fakes directory
	if env.PATH == "" {
		t.Error("PATH is empty")
	}

	// Verify environment variables are set
	foundTestDir := false
	foundCallLog := false
	foundRealGit := false

	for _, envVar := range env.Env {
		if envVar == "TEST_DIR="+env.Dir {
			foundTestDir = true
		}
		if envVar == "TEST_CALL_LOG="+env.CallLog {
			foundCallLog = true
		}
		if envVar == "REAL_GIT="+realGitPath {
			foundRealGit = true
		}
	}

	if !foundTestDir {
		t.Error("TEST_DIR not found in environment")
	}
	if !foundCallLog {
		t.Error("TEST_CALL_LOG not found in environment")
	}
	if !foundRealGit {
		t.Error("REAL_GIT not found in environment")
	}
}

// TestHelpers_readCallLog verifies reading from an empty and populated call log
func TestHelpers_readCallLog(t *testing.T) {
	env := setupTestEnv(t)

	// Initially, call log should be empty
	calls, err := readCallLog(env)
	if err != nil {
		t.Fatalf("readCallLog failed: %v", err)
	}
	if len(calls) != 0 {
		t.Errorf("Expected empty call log, got %d calls", len(calls))
	}

	// Write some test data to the call log
	testCalls := []string{
		"bd ready --json --limit 1",
		"claude -p prompt --model sonnet",
		"git add .",
		"git commit -m test",
	}

	f, err := os.Create(env.CallLog)
	if err != nil {
		t.Fatalf("Failed to create call log: %v", err)
	}
	for _, call := range testCalls {
		f.WriteString(call + "\n")
	}
	f.Close()

	// Read back the calls
	calls, err = readCallLog(env)
	if err != nil {
		t.Fatalf("readCallLog failed: %v", err)
	}

	if len(calls) != len(testCalls) {
		t.Errorf("Expected %d calls, got %d", len(testCalls), len(calls))
	}

	for i, expected := range testCalls {
		if i >= len(calls) {
			break
		}
		if calls[i] != expected {
			t.Errorf("Call %d: expected %q, got %q", i, expected, calls[i])
		}
	}
}

// TestHelpers_filterCalls verifies filtering calls by prefix
func TestHelpers_filterCalls(t *testing.T) {
	env := setupTestEnv(t)

	// Write mixed calls to the log
	testCalls := []string{
		"bd ready --json --limit 1",
		"claude -p prompt --model sonnet",
		"git add .",
		"bd show test-bead-1 --json",
		"git commit -m test",
		"bd close test-bead-1",
	}

	f, err := os.Create(env.CallLog)
	if err != nil {
		t.Fatalf("Failed to create call log: %v", err)
	}
	for _, call := range testCalls {
		f.WriteString(call + "\n")
	}
	f.Close()

	// Filter bd calls
	bdCalls, err := filterCalls(env, "bd")
	if err != nil {
		t.Fatalf("filterCalls failed: %v", err)
	}

	expectedBD := []string{
		"bd ready --json --limit 1",
		"bd show test-bead-1 --json",
		"bd close test-bead-1",
	}

	if len(bdCalls) != len(expectedBD) {
		t.Errorf("Expected %d bd calls, got %d", len(expectedBD), len(bdCalls))
	}

	for i, expected := range expectedBD {
		if i >= len(bdCalls) {
			break
		}
		if bdCalls[i] != expected {
			t.Errorf("BD call %d: expected %q, got %q", i, expected, bdCalls[i])
		}
	}

	// Filter git calls
	gitCalls, err := filterCalls(env, "git")
	if err != nil {
		t.Fatalf("filterCalls failed: %v", err)
	}

	expectedGit := []string{
		"git add .",
		"git commit -m test",
	}

	if len(gitCalls) != len(expectedGit) {
		t.Errorf("Expected %d git calls, got %d", len(expectedGit), len(gitCalls))
	}

	for i, expected := range expectedGit {
		if i >= len(gitCalls) {
			break
		}
		if gitCalls[i] != expected {
			t.Errorf("Git call %d: expected %q, got %q", i, expected, gitCalls[i])
		}
	}

	// Filter claude calls
	claudeCalls, err := filterCalls(env, "claude")
	if err != nil {
		t.Fatalf("filterCalls failed: %v", err)
	}

	expectedClaude := []string{
		"claude -p prompt --model sonnet",
	}

	if len(claudeCalls) != len(expectedClaude) {
		t.Errorf("Expected %d claude calls, got %d", len(expectedClaude), len(claudeCalls))
	}
}

// TestHelpers_runGromitWithEnv verifies running gromit with fake CLIs
func TestHelpers_runGromitWithEnv(t *testing.T) {
	env := setupTestEnv(t)

	// Run gromit with --help which should always succeed
	// This tests that we can execute gromit and capture output
	stdout, stderr, exitCode, err := runGromitWithEnv(env, "--help")

	if err != nil {
		t.Fatalf("runGromitWithEnv failed: %v", err)
	}

	// --help should succeed with exit code 0
	if exitCode != 0 {
		t.Logf("stdout: %s", stdout)
		t.Logf("stderr: %s", stderr)
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Should have some output
	if stdout == "" && stderr == "" {
		t.Error("Expected some output from gromit --help")
	}
}
