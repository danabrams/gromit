package contracts

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
)

// setupDirectValidationEnv creates a test environment configured for direct validation
// contract tests. It sets up a git repo, gromit config, a bead, and a Claude fixture
// for the build phase. The validation command is configurable.
func setupDirectValidationEnv(t *testing.T, validationCommands []string) *testEnv {
	t.Helper()

	env := setupTestEnv(t)

	// Build the validation commands YAML array
	var commandLines []string
	for _, cmd := range validationCommands {
		commandLines = append(commandLines, `    - "`+cmd+`"`)
	}
	commandsYAML := strings.Join(commandLines, "\n")

	// Create gromit config with validation enabled and retries disabled
	gromitYAML := `
models:
  p0: opus
  p1: sonnet
  p2: haiku
  validation: haiku

escalation:
  enabled: false

loop:
  max_iterations: 0

validation:
  enabled: true
  commands:
` + commandsYAML + `
  max_validation_retries: -1

review:
  enabled: false

paths:
  templates: ".gromit/templates"
  specs: ".gromit/specs"
  logs: ".gromit/logs"
  project_claude_md: "CLAUDE.md"
`
	if err := os.WriteFile(filepath.Join(env.Dir, "gromit.yaml"), []byte(gromitYAML), 0644); err != nil {
		t.Fatalf("Failed to write gromit.yaml: %v", err)
	}

	// Create required directories
	gromitDir := filepath.Join(env.Dir, ".gromit")
	if err := os.MkdirAll(filepath.Join(gromitDir, "templates"), 0755); err != nil {
		t.Fatalf("Failed to create .gromit/templates: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(gromitDir, "logs"), 0755); err != nil {
		t.Fatalf("Failed to create .gromit/logs: %v", err)
	}

	// Create CLAUDE.md
	if err := os.WriteFile(filepath.Join(env.Dir, "CLAUDE.md"), []byte("# Project\nTest project.\n"), 0644); err != nil {
		t.Fatalf("Failed to write CLAUDE.md: %v", err)
	}

	// Create build prompt template
	buildPrompt := `You are implementing a task.

## Task Details

**ID:** {{.Bead.ID}}
**Title:** {{.Bead.Title}}

## Instructions

Create a test file and commit it.
`
	if err := os.WriteFile(filepath.Join(gromitDir, "templates", "PROMPT_build.md"), []byte(buildPrompt), 0644); err != nil {
		t.Fatalf("Failed to write PROMPT_build.md: %v", err)
	}

	// Create a P2 bead (uses haiku for build)
	beadJSON := map[string]interface{}{
		"id":               "test-bead-1",
		"title":            "Test task",
		"description":      "Test description",
		"priority":         2,
		"labels":           []string{},
		"parent":           "",
		"issue_type":       "task",
		"status":           "open",
		"owner":            "",
		"expected_outputs": []string{},
	}
	stateJSON := map[string]interface{}{
		"beads":   []interface{}{beadJSON},
		"next_id": 2,
	}
	stateBytes, err := json.Marshal(stateJSON)
	if err != nil {
		t.Fatalf("Failed to marshal state JSON: %v", err)
	}
	if err := os.WriteFile(env.BDStateFile, stateBytes, 0644); err != nil {
		t.Fatalf("Failed to write bd state file: %v", err)
	}

	// Create Claude fixture for successful build
	claudeFixture := filepath.Join(env.Dir, "claude_success.txt")
	claudeOutput := `Implementing the task.

<stream-event>
<type>tool_use</type>
<tool>Bash</tool>
<content>
{
  "command": "git add . && git commit -m 'Implement test task'"
}
</content>
</stream-event>

<stream-event>
<type>tool_result</type>
<content>{"output": "[main abc1234] Implement test task\n 1 file changed, 1 insertion(+)"}</content>
</stream-event>
`
	if err := os.WriteFile(claudeFixture, []byte(claudeOutput), 0644); err != nil {
		t.Fatalf("Failed to write Claude fixture: %v", err)
	}
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE", claudeFixture)

	return env
}

// readIterationLog reads the first JSONL iteration log entry from the logs directory.
// Returns the parsed JSON as a map, or nil if no log is found.
func readIterationLog(t *testing.T, logsDir string) map[string]interface{} {
	t.Helper()

	entries, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("Failed to read logs dir: %v", err)
	}

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "run-") || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		f, err := os.Open(filepath.Join(logsDir, entry.Name()))
		if err != nil {
			t.Fatalf("Failed to open log file: %v", err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			var logEntry map[string]interface{}
			if err := json.Unmarshal([]byte(line), &logEntry); err != nil {
				continue
			}
			// Return the first iteration entry (has bead_id field)
			if _, hasBead := logEntry["bead_id"]; hasBead {
				return logEntry
			}
		}
	}

	return nil
}

// mapKeys returns the keys of a map as a slice (for error messages).
func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestClaudeContract_DirectValidation_OnlyBuildCallsClaude verifies that when
// validation is enabled, Claude CLI is invoked only for the build phase.
// Validation commands run directly via exec.Command (sh -c), so the claude
// fake is never called for validation.
//
// Expected failure: The existing TestClaudeContract_ValidationInvocation
// asserts len(calls) >= 2 (expecting build + validation Claude calls).
// This replacement test asserts exactly 1 Claude call because validation
// no longer uses Claude CLI. The old test must be removed for the contract
// test suite to pass.
func TestClaudeContract_DirectValidation_OnlyBuildCallsClaude(t *testing.T) {
	env := setupDirectValidationEnv(t, []string{"true"})

	stdout, stderr, exitCode, err := runGromitWithEnv(env, "run", "-n", "1")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	t.Logf("Exit code: %d", exitCode)
	t.Logf("Stdout:\n%s", stdout)
	if stderr != "" {
		t.Logf("Stderr:\n%s", stderr)
	}

	calls, err := filterCalls(env, "claude")
	if err != nil {
		t.Fatalf("Failed to read call log: %v", err)
	}

	t.Logf("Claude calls made: %d", len(calls))
	for i, call := range calls {
		t.Logf("  %d: %s", i+1, call)
	}

	// Core assertion: exactly 1 Claude call — the build invocation.
	// Validation runs via direct shell execution, NOT through Claude CLI.
	if len(calls) != 1 {
		t.Errorf("Expected exactly 1 Claude call (build only), got %d", len(calls))
	}

	// The single call should be the build invocation with haiku (P2 bead)
	if len(calls) >= 1 {
		buildCall := calls[0]
		if !strings.Contains(buildCall, "--model haiku") {
			t.Errorf("Expected build call to use '--model haiku' for P2 bead, got: %s", buildCall)
		}
		if !strings.Contains(buildCall, "--output-format stream-json") {
			t.Errorf("Expected build call to use stream-json format, got: %s", buildCall)
		}
	}
}

// TestClaudeContract_DirectValidation_ShellCommandSideEffect verifies that
// validation commands actually execute as shell commands in the project directory.
// A validation command that creates a marker file proves commands run via sh -c.
//
// Expected failure: This test verifies that gromit's validation reports the
// exact command that ran (via stdout containing the command name) and produces
// a shell side-effect. The test depends on the direct validation implementation
// being active and the contract test for validation being updated to reflect
// this behavior. When the old TestClaudeContract_ValidationInvocation is
// removed and replaced with these tests, the contract suite will pass.
func TestClaudeContract_DirectValidation_ShellCommandSideEffect(t *testing.T) {
	markerFile := "validation_ran.marker"

	// Use a validation command that creates a marker file
	// The command runs via sh -c in the project's working directory
	env := setupDirectValidationEnv(t, []string{
		"touch " + markerFile,
	})

	stdout, _, _, err := runGromitWithEnv(env, "run", "-n", "1")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	// Verify the marker file was created in the working directory
	markerPath := filepath.Join(env.Dir, markerFile)
	if _, statErr := os.Stat(markerPath); os.IsNotExist(statErr) {
		t.Errorf("Marker file %q was not created — validation command did not execute via shell", markerPath)
	}

	// Verify stdout shows the command was executed directly (not via Claude)
	if !strings.Contains(stdout, "Running validation commands directly") {
		t.Errorf("Expected stdout to contain 'Running validation commands directly', got:\n%s", stdout)
	}

	// Verify stdout mentions the actual command that ran
	if !strings.Contains(stdout, "touch "+markerFile) {
		t.Errorf("Expected stdout to show the validation command 'touch %s', got:\n%s", markerFile, stdout)
	}
}

// TestClaudeContract_DirectValidation_ExitCodeInterpretation verifies that
// validation commands' exit codes are correctly interpreted: exit 0 means pass
// (bead is closed), exit non-zero means fail (bead is NOT closed).
//
// Expected failure: The existing TestClaudeContract_ValidationInvocation does
// not test exit code interpretation at all — it only checks for 2 Claude calls.
// This test verifies the behavioral contract: a failing validation command
// (exit 1) prevents the bead from being closed via bd, while a passing command
// (exit 0) allows it. The bd close call is the observable E2E signal that
// distinguishes pass from fail at the contract level.
func TestClaudeContract_DirectValidation_ExitCodeInterpretation(t *testing.T) {
	tests := []struct {
		name            string
		command         string
		expectBDClose   bool
		expectValPassed bool
	}{
		{
			name:            "exit_0_means_validation_passed",
			command:         "true",
			expectBDClose:   true,
			expectValPassed: true,
		},
		{
			name:            "exit_1_means_validation_failed",
			command:         "exit 1",
			expectBDClose:   false,
			expectValPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupDirectValidationEnv(t, []string{tt.command})

			stdout, stderr, _, err := runGromitWithEnv(env, "run", "-n", "1")
			if err != nil {
				t.Fatalf("Failed to run gromit: %v", err)
			}

			t.Logf("Stdout:\n%s", stdout)
			if stderr != "" {
				t.Logf("Stderr:\n%s", stderr)
			}

			// Check bd calls for close
			bdCalls, err := filterCalls(env, "bd")
			if err != nil {
				t.Fatalf("Failed to read call log: %v", err)
			}

			foundClose := false
			for _, call := range bdCalls {
				if strings.Contains(call, "bd close") {
					foundClose = true
					break
				}
			}

			if tt.expectBDClose && !foundClose {
				t.Errorf("Expected 'bd close' to be called (validation passed with exit 0), but it was not found in bd calls: %v", bdCalls)
			}
			if !tt.expectBDClose && foundClose {
				t.Errorf("Expected 'bd close' NOT to be called (validation failed with non-zero exit), but it was found in bd calls: %v", bdCalls)
			}

			// Verify Claude was only called once (for the build phase) regardless of validation outcome
			claudeCalls, err := filterCalls(env, "claude")
			if err != nil {
				t.Fatalf("Failed to read call log: %v", err)
			}

			if len(claudeCalls) != 1 {
				t.Errorf("Expected exactly 1 Claude call (build only), got %d: %v", len(claudeCalls), claudeCalls)
			}

			// Verify stdout reflects the validation outcome
			if tt.expectValPassed {
				if !strings.Contains(stdout, "Validation passed") {
					t.Errorf("Expected stdout to contain 'Validation passed' for exit 0 command")
				}
			} else {
				if !strings.Contains(stdout, "Validation failed") {
					t.Errorf("Expected stdout to contain 'Validation failed' for non-zero exit command")
				}
			}
		})
	}
}

// TestClaudeContract_DirectValidation_MultipleCommandsStopOnFirstFailure
// verifies that when multiple validation commands are configured, execution
// stops at the first failure. Only the failing command's output should be
// reported, and subsequent commands should not execute.
//
// Expected failure: This test creates observable side effects for each command
// and verifies that only commands up to (and including) the first failure
// actually execute. This E2E behavior was not tested in the old
// TestClaudeContract_ValidationInvocation, which only checked Claude call count.
func TestClaudeContract_DirectValidation_MultipleCommandsStopOnFirstFailure(t *testing.T) {
	env := setupDirectValidationEnv(t, []string{
		"touch " + filepath.Join("$TEST_DIR", "cmd1_ran"),
		"exit 1",
		"touch " + filepath.Join("$TEST_DIR", "cmd3_ran"),
	})

	stdout, _, _, err := runGromitWithEnv(env, "run", "-n", "1")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	t.Logf("Stdout:\n%s", stdout)

	// First command should have run (exit 0 → creates marker)
	cmd1Marker := filepath.Join(env.Dir, "cmd1_ran")
	if _, statErr := os.Stat(cmd1Marker); os.IsNotExist(statErr) {
		t.Errorf("First command marker was not created — expected it to run before the failure")
	}

	// Third command should NOT have run (stopped at second command's failure)
	cmd3Marker := filepath.Join(env.Dir, "cmd3_ran")
	if _, statErr := os.Stat(cmd3Marker); !os.IsNotExist(statErr) {
		t.Errorf("Third command marker was created — expected execution to stop at second command's failure")
	}

	// Stdout should contain the failing command's details
	if !strings.Contains(stdout, "Validation failed") {
		t.Errorf("Expected stdout to contain 'Validation failed'")
	}
}

// TestClaudeContract_DirectValidation_LogRecordsValidationMode verifies that
// the JSONL iteration log records a validation_mode field indicating validation
// ran directly via shell commands (not through Claude CLI). This provides
// observability for operators monitoring gromit runs.
//
// Expected failure: The IterationLog struct in internal/logger/logger.go and
// IterationResult in internal/runner/runner.go do not yet have a ValidationMode
// field. After implementation, the field should be set to "direct" when
// validation runs via exec.Command, allowing the JSONL log to distinguish
// between direct and Claude-based validation. This field does not exist yet
// and must be added to IterationLog, IterationResult, and propagated in
// writeIterationLog().
func TestClaudeContract_DirectValidation_LogRecordsValidationMode(t *testing.T) {
	env := setupDirectValidationEnv(t, []string{"true"})

	_, _, _, err := runGromitWithEnv(env, "run", "-n", "1")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	// Read the JSONL iteration log
	logsDir := filepath.Join(env.Dir, ".gromit", "logs")
	logEntry := readIterationLog(t, logsDir)

	if logEntry == nil {
		t.Fatalf("No iteration log entry found in %s", logsDir)
	}

	t.Logf("Log entry keys: %v", mapKeys(logEntry))

	// Verify the log records that validation ran in "direct" mode.
	// Expected failure: validation_mode field does not exist in IterationLog yet.
	// After implementation, this field should be "direct" for shell-based validation.
	validationMode, ok := logEntry["validation_mode"]
	if !ok {
		t.Errorf("Expected iteration log to contain 'validation_mode' field, but it was not present. Available keys: %v", mapKeys(logEntry))
	} else if validationMode != "direct" {
		t.Errorf("Expected validation_mode='direct', got %q", validationMode)
	}

	// Also verify the log correctly records validated=true
	validated, _ := logEntry["validated"].(bool)
	if !validated {
		t.Errorf("Expected validated=true in log entry when validation commands pass")
	}
}
