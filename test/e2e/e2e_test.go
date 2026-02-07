//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var (
	// gromitBinary is the path to the built gromit binary
	gromitBinary string

	// realGitPath is the path to the real git binary (not the fake)
	realGitPath string

	// fakesDir is the absolute path to the test/fakes directory
	fakesDir string

	// fixturesDir is the absolute path to the test/fixtures directory
	fixturesDir string

	// scaffoldDir is a template directory created by `gromit init` once
	scaffoldDir string
)

// TestMain builds the gromit binary, scaffolds template directory with gromit init
func TestMain(m *testing.M) {
	// Build the gromit binary
	tmpDir, err := os.MkdirTemp("", "gromit-e2e-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	gromitBinary = filepath.Join(tmpDir, "gromit")
	buildCmd := exec.Command("go", "build", "-o", gromitBinary, "../../cmd/gromit")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build gromit binary: %v\n", err)
		os.Exit(1)
	}

	// Resolve real git path
	realGitPath = findRealGit()
	if realGitPath == "" {
		fmt.Fprintf(os.Stderr, "Failed to find real git binary\n")
		os.Exit(1)
	}

	// Resolve absolute paths to fakes and fixtures directories
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
		os.Exit(1)
	}
	fakesDir = filepath.Join(wd, "..", "fakes")
	fixturesDir = filepath.Join(wd, "..", "fixtures")

	// Create scaffold directory by running gromit init
	scaffoldDir = filepath.Join(tmpDir, "scaffold")
	if err := os.MkdirAll(scaffoldDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create scaffold dir: %v\n", err)
		os.Exit(1)
	}

	// Initialize git repo in scaffold directory
	gitInit := exec.Command(realGitPath, "init")
	gitInit.Dir = scaffoldDir
	if err := gitInit.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize git repo in scaffold dir: %v\n", err)
		os.Exit(1)
	}

	// Configure git user
	gitConfig := exec.Command(realGitPath, "config", "user.email", "test@example.com")
	gitConfig.Dir = scaffoldDir
	if err := gitConfig.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to configure git user.email: %v\n", err)
		os.Exit(1)
	}

	gitConfig = exec.Command(realGitPath, "config", "user.name", "Test User")
	gitConfig.Dir = scaffoldDir
	if err := gitConfig.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to configure git user.name: %v\n", err)
		os.Exit(1)
	}

	// Run gromit init in scaffold directory
	initCmd := exec.Command(gromitBinary, "init")
	initCmd.Dir = scaffoldDir
	initCmd.Stdout = os.Stdout
	initCmd.Stderr = os.Stderr
	if err := initCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run gromit init in scaffold dir: %v\n", err)
		os.Exit(1)
	}

	// Run the tests
	exitCode := m.Run()
	os.Exit(exitCode)
}

// findRealGit searches for the real git binary in common locations.
func findRealGit() string {
	// First, try to find git in the current PATH
	gitPath, err := exec.LookPath("git")
	if err == nil && gitPath != "" {
		return gitPath
	}

	// Fall back to common locations
	commonPaths := []string{
		"/usr/bin/git",
		"/usr/local/bin/git",
		"/opt/homebrew/bin/git",
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// e2eEnv holds the E2E test environment configuration
type e2eEnv struct {
	// Dir is the test directory (temporary, cleaned up after test)
	Dir string

	// CallLog is the path to the file where fake CLIs record invocations
	CallLog string

	// BDStateFile is the path to the fake bd state file
	BDStateFile string

	// PATH is the modified PATH with fakes prepended
	PATH string

	// Env is the full environment to pass to gromit
	Env []string
}

// setupE2E creates a fresh test environment for E2E tests by:
// - Creating a temp directory
// - Copying the scaffolded gromit project structure
// - Setting up fake CLIs in PATH
// - Creating call log and bd state files
func setupE2E(t *testing.T) *e2eEnv {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "gromit-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	// Copy scaffold to temp directory
	if err := copyScaffold(scaffoldDir, tmpDir); err != nil {
		t.Fatalf("Failed to copy scaffold: %v", err)
	}

	// Create call log file
	callLog := filepath.Join(tmpDir, "call_log.txt")

	// Create bd state file
	bdStateFile := filepath.Join(tmpDir, ".fake_bd_state.json")

	// Build modified PATH with fakes prepended
	originalPath := os.Getenv("PATH")
	modifiedPath := fakesDir + string(os.PathListSeparator) + originalPath

	// Build environment
	env := os.Environ()
	env = replaceOrAppend(env, "PATH", modifiedPath)
	env = replaceOrAppend(env, "TEST_DIR", tmpDir)
	env = replaceOrAppend(env, "TEST_CALL_LOG", callLog)
	env = replaceOrAppend(env, "REAL_GIT", realGitPath)

	return &e2eEnv{
		Dir:         tmpDir,
		CallLog:     callLog,
		BDStateFile: bdStateFile,
		PATH:        modifiedPath,
		Env:         env,
	}
}

// copyScaffold recursively copies the scaffold directory to the destination.
// It preserves directory structure and file permissions.
func copyScaffold(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	// Create destination directory with same permissions
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	// Read source directory entries
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read source dir: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := copyScaffold(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file from src to dst, preserving permissions.
func copyFile(src, dst string) error {
	// Read source file
	srcData, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read source file: %w", err)
	}

	// Get source file info for permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source file: %w", err)
	}

	// Write destination file with same permissions
	if err := os.WriteFile(dst, srcData, srcInfo.Mode()); err != nil {
		return fmt.Errorf("write dest file: %w", err)
	}

	return nil
}

// runGromit runs the gromit binary with the given arguments in the test environment.
func runGromit(env *e2eEnv, args ...string) (stdout, stderr string, exitCode int, err error) {
	cmd := exec.Command(gromitBinary, args...)
	cmd.Dir = env.Dir
	cmd.Env = env.Env

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if runErr != nil {
		// Check if it's an ExitError (non-zero exit code)
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil // Not an error - just a non-zero exit code
		} else {
			// Some other error (e.g., command not found)
			err = runErr
			exitCode = -1
		}
	} else {
		exitCode = 0
	}

	return stdout, stderr, exitCode, err
}

// readBDState reads the fake bd state file and returns the parsed state.
func readBDState(env *e2eEnv) (map[string]interface{}, error) {
	data, err := os.ReadFile(env.BDStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// State file doesn't exist yet - return empty state
			return map[string]interface{}{
				"beads":   []interface{}{},
				"next_id": 1,
			}, nil
		}
		return nil, err
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal bd state: %w", err)
	}

	return state, nil
}

// writeBDState writes the given state to the fake bd state file.
func writeBDState(env *e2eEnv, state map[string]interface{}) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal bd state: %w", err)
	}

	if err := os.WriteFile(env.BDStateFile, data, 0644); err != nil {
		return fmt.Errorf("write bd state file: %w", err)
	}

	return nil
}

// createBead adds a bead to the fake bd state.
func createBead(env *e2eEnv, id, title, description string, priority int, labels []string) error {
	state, err := readBDState(env)
	if err != nil {
		return err
	}

	bead := map[string]interface{}{
		"id":               id,
		"title":            title,
		"description":      description,
		"priority":         priority,
		"labels":           labels,
		"parent":           "",
		"issue_type":       "task",
		"status":           "open",
		"owner":            "",
		"expected_outputs": []string{},
	}

	beads, ok := state["beads"].([]interface{})
	if !ok {
		beads = []interface{}{}
	}
	beads = append(beads, bead)
	state["beads"] = beads

	return writeBDState(env, state)
}

// replaceOrAppend replaces an environment variable in the env slice,
// or appends it if it doesn't exist.
func replaceOrAppend(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

// TestE2EInfrastructure verifies that the E2E test infrastructure is set up correctly.
func TestE2EInfrastructure(t *testing.T) {
	env := setupE2E(t)

	// Verify scaffold was copied
	gromitYAML := filepath.Join(env.Dir, "gromit.yaml")
	if _, err := os.Stat(gromitYAML); err != nil {
		t.Fatalf("gromit.yaml not found in test dir: %v", err)
	}

	// Verify .gromit directory exists
	gromitDir := filepath.Join(env.Dir, ".gromit")
	if _, err := os.Stat(gromitDir); err != nil {
		t.Fatalf(".gromit directory not found: %v", err)
	}

	// Verify templates directory exists
	templatesDir := filepath.Join(gromitDir, "templates")
	if _, err := os.Stat(templatesDir); err != nil {
		t.Fatalf(".gromit/templates directory not found: %v", err)
	}

	// Verify RULES.md exists
	rulesFile := filepath.Join(gromitDir, "RULES.md")
	if _, err := os.Stat(rulesFile); err != nil {
		t.Fatalf("RULES.md not found: %v", err)
	}

	// Verify LEARNINGS.md exists
	learningsFile := filepath.Join(gromitDir, "LEARNINGS.md")
	if _, err := os.Stat(learningsFile); err != nil {
		t.Fatalf("LEARNINGS.md not found: %v", err)
	}

	// Verify git repo exists
	gitDir := filepath.Join(env.Dir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		t.Fatalf(".git directory not found: %v", err)
	}

	// Test createBead helper
	if err := createBead(env, "test-1", "Test bead", "Test description", 1, []string{}); err != nil {
		t.Fatalf("createBead failed: %v", err)
	}

	// Verify bead was created in state
	state, err := readBDState(env)
	if err != nil {
		t.Fatalf("readBDState failed: %v", err)
	}

	beads, ok := state["beads"].([]interface{})
	if !ok {
		t.Fatal("beads field not found in state")
	}

	if len(beads) != 1 {
		t.Fatalf("expected 1 bead, got %d", len(beads))
	}

	bead, ok := beads[0].(map[string]interface{})
	if !ok {
		t.Fatal("bead is not a map")
	}

	if bead["id"] != "test-1" {
		t.Errorf("expected bead id test-1, got %v", bead["id"])
	}

	if bead["title"] != "Test bead" {
		t.Errorf("expected bead title 'Test bead', got %v", bead["title"])
	}
}

// TestE2E_HappyPath tests the full happy path workflow:
// - Creates 2 beads in the fake bd state
// - Runs gromit with success fixtures
// - Verifies both beads are closed with correct invocation sequences
func TestE2E_HappyPath(t *testing.T) {
	env := setupE2E(t)

	// Create 2 beads in the fake bd state
	if err := createBead(env, "test-bead-1", "First task", "Implement first feature", 1, []string{}); err != nil {
		t.Fatalf("Failed to create bead 1: %v", err)
	}
	if err := createBead(env, "test-bead-2", "Second task", "Implement second feature", 1, []string{}); err != nil {
		t.Fatalf("Failed to create bead 2: %v", err)
	}

	// Set up Claude fixtures to simulate successful build and validation
	buildFixture := filepath.Join(fixturesDir, "claude_build_success.txt")
	validateFixture := filepath.Join(fixturesDir, "claude_validate_success.txt")

	// Point CLAUDE_FIXTURE to the build fixture for build model
	// The fake claude script will use this
	env.Env = replaceOrAppend(env.Env, "CLAUDE_FIXTURE", buildFixture)
	env.Env = replaceOrAppend(env.Env, "CLAUDE_FIXTURE_HAIKU", validateFixture)

	// Run gromit with -n 2 to process exactly 2 beads
	stdout, stderr, exitCode, err := runGromit(env, "run", "-n", "2")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	t.Logf("Exit code: %d", exitCode)
	t.Logf("Stdout:\n%s", stdout)
	t.Logf("Stderr:\n%s", stderr)

	// Verify gromit succeeded
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Read call log to verify invocation sequences
	callLogData, err := os.ReadFile(env.CallLog)
	if err != nil {
		t.Fatalf("Failed to read call log: %v", err)
	}
	calls := strings.Split(strings.TrimSpace(string(callLogData)), "\n")

	t.Logf("Total calls recorded: %d", len(calls))
	for i, call := range calls {
		t.Logf("  %d: %s", i+1, call)
	}

	// Filter bd calls
	var bdCalls []string
	for _, call := range calls {
		if strings.HasPrefix(call, "bd ") {
			bdCalls = append(bdCalls, call)
		}
	}

	// Filter claude calls
	var claudeCalls []string
	for _, call := range calls {
		if strings.HasPrefix(call, "claude ") {
			claudeCalls = append(claudeCalls, call)
		}
	}

	t.Logf("BD calls: %d", len(bdCalls))
	for i, call := range bdCalls {
		t.Logf("  BD %d: %s", i+1, call)
	}

	t.Logf("Claude calls: %d", len(claudeCalls))
	for i, call := range claudeCalls {
		t.Logf("  Claude %d: %s", i+1, call)
	}

	// Verify expected bd call sequence:
	// For each bead:
	//   1. bd ready --json --limit 10 (fetch next bead)
	//   2. bd close <id> (after successful build + validation)
	//   3. bd sync (sync after close)
	//
	// Expected sequence for 2 beads (with -n 2):
	// 1. bd ready (get bead 1)
	// 2. bd close test-bead-1
	// 3. bd sync
	// 4. bd ready (get bead 2)
	// 5. bd close test-bead-2
	// 6. bd sync
	// Note: No 7th bd ready because max iterations (2) is reached

	expectedBDSequence := []string{
		"bd ready --json --limit 10",
		"bd close test-bead-1",
		"bd sync",
		"bd ready --json --limit 10",
		"bd close test-bead-2",
		"bd sync",
	}

	if len(bdCalls) < len(expectedBDSequence) {
		t.Errorf("Expected at least %d bd calls, got %d", len(expectedBDSequence), len(bdCalls))
	}

	for i, expected := range expectedBDSequence {
		if i >= len(bdCalls) {
			t.Errorf("Missing expected bd call %d: %s", i+1, expected)
			continue
		}
		if !strings.Contains(bdCalls[i], expected) {
			t.Errorf("BD call %d: expected to contain %q, got %q", i+1, expected, bdCalls[i])
		} else {
			t.Logf("✓ BD call %d matches: %s", i+1, expected)
		}
	}

	// Verify Claude was invoked for each bead
	// Looking at the actual calls, each bead has 3 Claude invocations:
	//   1. Scope estimation (haiku)
	//   2. Build invocation (with build model - sonnet for P1)
	//   3. Validation invocation (with haiku model)
	//
	// Expected: 6 Claude calls total (2 beads × 3 phases each)
	expectedClaudeCalls := 6
	if len(claudeCalls) != expectedClaudeCalls {
		t.Errorf("Expected %d Claude calls, got %d", expectedClaudeCalls, len(claudeCalls))
	} else {
		t.Logf("✓ Claude called %d times (3 per bead: scope, build, validate)", len(claudeCalls))
	}

	// Verify both beads are closed in the bd state
	state, err := readBDState(env)
	if err != nil {
		t.Fatalf("Failed to read bd state: %v", err)
	}

	beads, ok := state["beads"].([]interface{})
	if !ok {
		t.Fatal("beads field not found in state")
	}

	if len(beads) != 2 {
		t.Fatalf("Expected 2 beads in state, got %d", len(beads))
	}

	// Verify both beads have status "closed"
	for i, beadInterface := range beads {
		bead, ok := beadInterface.(map[string]interface{})
		if !ok {
			t.Fatalf("Bead %d is not a map", i)
		}

		status, ok := bead["status"].(string)
		if !ok {
			t.Fatalf("Bead %d status is not a string", i)
		}

		if status != "closed" {
			t.Errorf("Bead %d: expected status 'closed', got %q", i, status)
		} else {
			t.Logf("✓ Bead %s is closed", bead["id"])
		}
	}

	// Verify iteration logs were written
	logsDir := filepath.Join(env.Dir, ".gromit", "logs")
	logEntries, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("Failed to read logs directory: %v", err)
	}

	if len(logEntries) == 0 {
		t.Error("Expected log files to be written, but logs directory is empty")
	} else {
		t.Logf("✓ Log files written: %d entries", len(logEntries))
	}
}

// TestE2E_EmptyQueue tests that gromit run exits cleanly when there are no ready beads:
// - Creates empty fake bd state (no beads)
// - Runs gromit run
// - Verifies clean exit (code 0) with no claude invocations
func TestE2E_EmptyQueue(t *testing.T) {
	env := setupE2E(t)

	// Create empty bd state explicitly
	emptyState := map[string]interface{}{
		"beads":   []interface{}{},
		"next_id": 1,
	}
	if err := writeBDState(env, emptyState); err != nil {
		t.Fatalf("Failed to write empty bd state: %v", err)
	}

	// Run gromit run with no beads
	stdout, stderr, exitCode, err := runGromit(env, "run")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	t.Logf("Exit code: %d", exitCode)
	t.Logf("Stdout:\n%s", stdout)
	t.Logf("Stderr:\n%s", stderr)

	// Verify gromit succeeded with clean exit
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Read call log to verify no claude invocations occurred
	callLogData, err := os.ReadFile(env.CallLog)
	if err != nil {
		// If call log doesn't exist, that's acceptable for empty queue
		if !os.IsNotExist(err) {
			t.Fatalf("Failed to read call log: %v", err)
		}
		t.Logf("✓ No call log file created (no external tool calls)")
		return
	}

	calls := strings.Split(strings.TrimSpace(string(callLogData)), "\n")

	// Filter out empty lines
	var nonEmptyCalls []string
	for _, call := range calls {
		if strings.TrimSpace(call) != "" {
			nonEmptyCalls = append(nonEmptyCalls, call)
		}
	}

	t.Logf("Total calls recorded: %d", len(nonEmptyCalls))
	for i, call := range nonEmptyCalls {
		t.Logf("  %d: %s", i+1, call)
	}

	// Filter claude calls
	var claudeCalls []string
	for _, call := range nonEmptyCalls {
		if strings.HasPrefix(call, "claude ") {
			claudeCalls = append(claudeCalls, call)
		}
	}

	// Verify no Claude invocations occurred
	if len(claudeCalls) > 0 {
		t.Errorf("Expected 0 Claude calls for empty queue, got %d:", len(claudeCalls))
		for i, call := range claudeCalls {
			t.Errorf("  Unexpected Claude call %d: %s", i+1, call)
		}
	} else {
		t.Logf("✓ No Claude invocations (as expected for empty queue)")
	}

	// Verify bd ready was called at least once to check for work
	var bdReadyCalls []string
	for _, call := range nonEmptyCalls {
		if strings.HasPrefix(call, "bd ready") {
			bdReadyCalls = append(bdReadyCalls, call)
		}
	}

	if len(bdReadyCalls) == 0 {
		t.Errorf("Expected at least one 'bd ready' call to check for work, but found none")
	} else {
		t.Logf("✓ bd ready called %d time(s) to check for work", len(bdReadyCalls))
	}

	// Verify stdout contains appropriate message about no work
	if !strings.Contains(stdout, "No more work available") {
		t.Errorf("Expected stdout to contain 'No more work available', got:\n%s", stdout)
	} else {
		t.Logf("✓ Stdout indicates no work available")
	}
}

// TestE2E_Escalation tests that gromit correctly escalates from haiku to sonnet
// when the first build invocation fails:
// - Creates a P2 bead (which uses haiku by default)
// - Configures claude fake to fail first invocation (haiku) and succeed second (sonnet)
// - Runs gromit run
// - Verifies call log shows haiku→sonnet escalation
func TestE2E_Escalation(t *testing.T) {
	env := setupE2E(t)

	// Create a P2 bead (priority 2 → haiku model by default)
	if err := createBead(env, "test-escalate-1", "Task requiring escalation", "This task will fail on haiku and succeed on sonnet", 2, []string{}); err != nil {
		t.Fatalf("Failed to create bead: %v", err)
	}

	// Set up Claude fixtures:
	// - Haiku uses success fixtures (for scope, analysis, validation)
	// - Sonnet uses success fixture (for build after escalation)
	// - CLAUDE_FAIL_HAIKU_ONCE=1 will make the first haiku stream-json invocation
	//   fail with exit code 1 (triggering escalation), but output the fail fixture text
	buildFailFixture := filepath.Join(fixturesDir, "claude_build_fail.txt")
	buildSuccessFixture := filepath.Join(fixturesDir, "claude_build_success.txt")
	validateSuccessFixture := filepath.Join(fixturesDir, "claude_validate_success.txt")

	// The CLAUDE_FAIL_HAIKU_ONCE mode in the fake claude script will:
	// 1. On first stream-json invocation with haiku: use CLAUDE_FIXTURE_HAIKU (fail), exit 1
	// 2. On subsequent invocations: use CLAUDE_FIXTURE_HAIKU_SUCCESS (success), exit 0
	//
	// This allows the first build to fail (triggering escalation), while scope check,
	// analysis, and validation all succeed.
	env.Env = replaceOrAppend(env.Env, "CLAUDE_FIXTURE_HAIKU", buildFailFixture)
	env.Env = replaceOrAppend(env.Env, "CLAUDE_FIXTURE_HAIKU_SUCCESS", validateSuccessFixture)
	env.Env = replaceOrAppend(env.Env, "CLAUDE_FIXTURE_SONNET", buildSuccessFixture)
	env.Env = replaceOrAppend(env.Env, "CLAUDE_FIXTURE", validateSuccessFixture) // Default fallback

	// Configure haiku to fail only on first stream-json invocation (build phase)
	env.Env = replaceOrAppend(env.Env, "CLAUDE_FAIL_HAIKU_ONCE", "1")

	// Run gromit with -n 1 to process exactly 1 bead
	stdout, stderr, exitCode, err := runGromit(env, "run", "-n", "1")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	t.Logf("Exit code: %d", exitCode)
	t.Logf("Stdout:\n%s", stdout)
	t.Logf("Stderr:\n%s", stderr)

	// Verify gromit succeeded (escalation worked)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 after escalation, got %d", exitCode)
	}

	// Read call log to verify escalation sequence
	callLogData, err := os.ReadFile(env.CallLog)
	if err != nil {
		t.Fatalf("Failed to read call log: %v", err)
	}
	calls := strings.Split(strings.TrimSpace(string(callLogData)), "\n")

	t.Logf("Total calls recorded: %d", len(calls))
	for i, call := range calls {
		t.Logf("  %d: %s", i+1, call)
	}

	// Filter claude calls
	var claudeCalls []string
	for _, call := range calls {
		if strings.HasPrefix(call, "claude ") {
			claudeCalls = append(claudeCalls, call)
		}
	}

	t.Logf("Claude calls: %d", len(claudeCalls))
	for i, call := range claudeCalls {
		t.Logf("  Claude %d: %s", i+1, call)
	}

	// Expected Claude call sequence:
	// 1. Scope check with haiku (succeeds)
	// 2. Build with haiku (fails, triggers escalation)
	// 3. Build with sonnet (succeeds)
	// 4. Validation with haiku (succeeds)
	//
	// So we expect at least 4 Claude calls, with haiku failing and sonnet succeeding

	if len(claudeCalls) < 4 {
		t.Errorf("Expected at least 4 Claude calls (scope, failed haiku build, sonnet build, validate), got %d", len(claudeCalls))
	}

	// Verify that we have calls to both haiku and sonnet
	var haikuCalls []string
	var sonnetCalls []string
	for _, call := range claudeCalls {
		if strings.Contains(call, "--model haiku") {
			haikuCalls = append(haikuCalls, call)
		}
		if strings.Contains(call, "--model sonnet") {
			sonnetCalls = append(sonnetCalls, call)
		}
	}

	t.Logf("Haiku calls: %d", len(haikuCalls))
	for i, call := range haikuCalls {
		t.Logf("  Haiku %d: %s", i+1, call)
	}

	t.Logf("Sonnet calls: %d", len(sonnetCalls))
	for i, call := range sonnetCalls {
		t.Logf("  Sonnet %d: %s", i+1, call)
	}

	// Verify escalation happened: at least one haiku call and at least one sonnet call
	if len(haikuCalls) == 0 {
		t.Error("Expected at least one call to haiku model (initial attempt)")
	} else {
		t.Logf("✓ Haiku was invoked initially")
	}

	if len(sonnetCalls) == 0 {
		t.Error("Expected at least one call to sonnet model (after escalation)")
	} else {
		t.Logf("✓ Sonnet was invoked after escalation")
	}

	// Verify the bead was closed (escalation succeeded)
	state, err := readBDState(env)
	if err != nil {
		t.Fatalf("Failed to read bd state: %v", err)
	}

	beads, ok := state["beads"].([]interface{})
	if !ok {
		t.Fatal("beads field not found in state")
	}

	if len(beads) != 1 {
		t.Fatalf("Expected 1 bead in state, got %d", len(beads))
	}

	bead, ok := beads[0].(map[string]interface{})
	if !ok {
		t.Fatal("Bead is not a map")
	}

	status, ok := bead["status"].(string)
	if !ok {
		t.Fatal("Bead status is not a string")
	}

	if status != "closed" {
		t.Errorf("Expected bead status 'closed' after successful escalation, got %q", status)
	} else {
		t.Logf("✓ Bead was closed after successful escalation")
	}
}

// TestE2E_ValidationFailure tests that gromit correctly handles validation failures
// by keeping the bead open when build succeeds but validation fails:
// - Creates a single bead
// - Configures build to succeed and validation to fail
// - Runs gromit run
// - Verifies bead status remains "open" (not closed)
// - Verifies bd close was not called
func TestE2E_ValidationFailure(t *testing.T) {
	env := setupE2E(t)

	// Create a single bead
	if err := createBead(env, "test-validation-fail", "Task with validation failure", "Build succeeds but validation fails", 1, []string{}); err != nil {
		t.Fatalf("Failed to create bead: %v", err)
	}

	// Set up Claude fixtures:
	// - Build succeeds
	// - Validation fails
	buildSuccessFixture := filepath.Join(fixturesDir, "claude_build_success.txt")
	validateFailFixture := filepath.Join(fixturesDir, "claude_validate_fail.txt")

	env.Env = replaceOrAppend(env.Env, "CLAUDE_FIXTURE", buildSuccessFixture)
	env.Env = replaceOrAppend(env.Env, "CLAUDE_FIXTURE_HAIKU", validateFailFixture)

	// Run gromit with -n 1 to process exactly 1 bead
	stdout, stderr, exitCode, err := runGromit(env, "run", "-n", "1")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	t.Logf("Exit code: %d", exitCode)
	t.Logf("Stdout:\n%s", stdout)
	t.Logf("Stderr:\n%s", stderr)

	// Verify gromit reports the validation failure in output
	if !strings.Contains(stdout, "Validation failed") && !strings.Contains(stdout, "VALIDATION_FAILED") {
		t.Errorf("Expected stdout to mention validation failure, got:\n%s", stdout)
	} else {
		t.Logf("✓ Validation failure reported in output")
	}

	// Read call log to verify bd close was not called
	callLogData, err := os.ReadFile(env.CallLog)
	if err != nil {
		if !os.IsNotExist(err) {
			t.Fatalf("Failed to read call log: %v", err)
		}
		// If call log doesn't exist, that's acceptable
		t.Logf("✓ No call log file created")
	} else {
		calls := strings.Split(strings.TrimSpace(string(callLogData)), "\n")

		t.Logf("Total calls recorded: %d", len(calls))
		for i, call := range calls {
			t.Logf("  %d: %s", i+1, call)
		}

		// Filter bd calls
		var bdCloseCalls []string
		for _, call := range calls {
			if strings.HasPrefix(call, "bd close") {
				bdCloseCalls = append(bdCloseCalls, call)
			}
		}

		// Verify bd close was NOT called
		if len(bdCloseCalls) > 0 {
			t.Errorf("Expected 0 'bd close' calls after validation failure, got %d:", len(bdCloseCalls))
			for i, call := range bdCloseCalls {
				t.Errorf("  Unexpected bd close call %d: %s", i+1, call)
			}
		} else {
			t.Logf("✓ No 'bd close' calls (bead stays open)")
		}
	}

	// Verify bead status is still "open" (not closed)
	state, err := readBDState(env)
	if err != nil {
		t.Fatalf("Failed to read bd state: %v", err)
	}

	beads, ok := state["beads"].([]interface{})
	if !ok {
		t.Fatal("beads field not found in state")
	}

	if len(beads) != 1 {
		t.Fatalf("Expected 1 bead in state, got %d", len(beads))
	}

	bead, ok := beads[0].(map[string]interface{})
	if !ok {
		t.Fatal("Bead is not a map")
	}

	status, ok := bead["status"].(string)
	if !ok {
		t.Fatalf("Bead status is not a string")
	}

	if status != "open" {
		t.Errorf("Expected bead status 'open' after validation failure, got %q", status)
	} else {
		t.Logf("✓ Bead status remains 'open' after validation failure")
	}
}

// TestE2E_TimeBudget tests that gromit respects the time budget and stops gracefully
// when the budget is exceeded:
// - Creates 5 beads
// - Configures claude fake with 5s delay per invocation
// - Runs gromit with 1-minute time budget
// - Verifies fewer than 5 beads were processed
func TestE2E_TimeBudget(t *testing.T) {
	env := setupE2E(t)

	// Create 5 beads
	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("test-budget-%d", i)
		title := fmt.Sprintf("Task %d", i)
		description := fmt.Sprintf("Task %d for time budget test", i)
		if err := createBead(env, id, title, description, 1, []string{}); err != nil {
			t.Fatalf("Failed to create bead %d: %v", i, err)
		}
	}

	// Set up Claude fixtures for success
	buildFixture := filepath.Join(fixturesDir, "claude_build_success.txt")
	validateFixture := filepath.Join(fixturesDir, "claude_validate_success.txt")

	env.Env = replaceOrAppend(env.Env, "CLAUDE_FIXTURE", buildFixture)
	env.Env = replaceOrAppend(env.Env, "CLAUDE_FIXTURE_HAIKU", validateFixture)

	// Configure claude fake with 2-second delay per invocation
	env.Env = replaceOrAppend(env.Env, "CLAUDE_DELAY", "2")

	// Run gromit with 1-minute time budget
	// With 2s delay per Claude call and 3 calls per bead (scope, build, validate),
	// each bead takes ~6 seconds. With 60-second budget, we should process 10 beads max.
	// But we only have 5 beads, so we expect fewer than 5 to complete if time checking works.
	// Actually, let's use a larger delay to ensure we can't complete all 5 in 1 minute.
	env.Env = replaceOrAppend(env.Env, "CLAUDE_DELAY", "5")

	// With 5s delay per Claude call and 3 calls per bead, each bead takes ~15 seconds.
	// In 60 seconds, we should process 4 beads max.
	stdout, stderr, exitCode, err := runGromit(env, "run", "--time-budget", "1")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	t.Logf("Exit code: %d", exitCode)
	t.Logf("Stdout:\n%s", stdout)
	t.Logf("Stderr:\n%s", stderr)

	// When time budget is exceeded, gromit may exit with code 1 due to context cancellation
	// This is acceptable - the important thing is that it stopped processing
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1, got %d", exitCode)
	} else {
		t.Logf("✓ Gromit stopped with exit code %d", exitCode)
	}

	// Read bd state to count how many beads were closed
	state, err := readBDState(env)
	if err != nil {
		t.Fatalf("Failed to read bd state: %v", err)
	}

	beads, ok := state["beads"].([]interface{})
	if !ok {
		t.Fatal("beads field not found in state")
	}

	if len(beads) != 5 {
		t.Fatalf("Expected 5 beads in state, got %d", len(beads))
	}

	// Count closed beads
	closedCount := 0
	for i, beadInterface := range beads {
		bead, ok := beadInterface.(map[string]interface{})
		if !ok {
			t.Fatalf("Bead %d is not a map", i)
		}

		status, ok := bead["status"].(string)
		if !ok {
			t.Fatalf("Bead %d status is not a string", i)
		}

		if status == "closed" {
			closedCount++
			t.Logf("✓ Bead %s is closed", bead["id"])
		}
	}

	t.Logf("Processed %d out of 5 beads within time budget", closedCount)

	// Verify that fewer than 5 beads were processed
	// With 5s delay per Claude invocation and ~3 invocations per bead (scope, build, validate),
	// that's roughly 15 seconds per bead. In 60 seconds, we should process 4 beads max.
	if closedCount >= 5 {
		t.Errorf("Expected fewer than 5 beads to be processed within 1-minute budget, but %d were closed", closedCount)
	} else {
		t.Logf("✓ Time budget respected: only %d/5 beads processed", closedCount)
	}

	// Verify that at least one bead was processed (time budget wasn't too restrictive)
	if closedCount == 0 {
		t.Error("Expected at least 1 bead to be processed, but none were closed")
	}

	// Verify output mentions time budget or context cancellation
	if strings.Contains(stdout, "Time budget") || strings.Contains(stdout, "time budget") {
		t.Logf("✓ Output mentions time budget")
	} else if strings.Contains(stdout, "Context cancelled") || strings.Contains(stderr, "context deadline exceeded") {
		t.Logf("✓ Output indicates context cancellation due to time budget")
	} else {
		t.Logf("Warning: Expected output to mention time budget or context cancellation")
	}
}
