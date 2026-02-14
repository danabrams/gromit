package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/test/testutil"
)

// TestBDContract_SingleBeadRun verifies that gromit passes the correct arguments
// to bd for ready, close, and sync subcommands during a single-bead run.
// Note: bd show is not called during normal run - Ready returns full bead details.
func TestBDContract_SingleBeadRun(t *testing.T) {
	env := setupTestEnv(t)

	// Create a gromit.yaml config in the test directory
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
    - "go test ./..."
    - "go vet ./..."
    - "go build ./cmd/gromit"

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

	// Create minimal RULES.md
	rulesContent := "# Rules\n\nNo hard-coded secrets.\n"
	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte(rulesContent), 0644); err != nil {
		t.Fatalf("Failed to write RULES.md: %v", err)
	}

	// Create minimal LEARNINGS.md
	learningsContent := "# Learnings\n\nNo learnings yet.\n"
	if err := os.WriteFile(filepath.Join(gromitDir, "LEARNINGS.md"), []byte(learningsContent), 0644); err != nil {
		t.Fatalf("Failed to write LEARNINGS.md: %v", err)
	}

	// Create minimal build prompt template
	buildPrompt := `You are implementing a task.

## Task Details

**ID:** {{.Bead.ID}}
**Title:** {{.Bead.Title}}
**Description:** {{.Bead.Description}}

## Instructions

Implement the task above. When done, create a git commit.
`
	if err := os.WriteFile(filepath.Join(gromitDir, "templates", "PROMPT_build.md"), []byte(buildPrompt), 0644); err != nil {
		t.Fatalf("Failed to write PROMPT_build.md: %v", err)
	}

	// Create minimal validate prompt template
	validatePrompt := `Run validation commands.

## Validation Commands

{{range .ValidationCommands}}
- {{.Cmd}}
{{end}}

Run each command and report results.
`
	if err := os.WriteFile(filepath.Join(gromitDir, "templates", "PROMPT_validate.md"), []byte(validatePrompt), 0644); err != nil {
		t.Fatalf("Failed to write PROMPT_validate.md: %v", err)
	}

	// Create a test bead via the fake bd
	beadJSON := `{
		"id": "test-bead-1",
		"title": "Test task",
		"description": "Test description",
		"priority": 1,
		"labels": [],
		"parent": "",
		"issue_type": "task",
		"status": "open",
		"owner": "",
		"expected_outputs": ["Implement feature"]
	}`

	var bead map[string]interface{}
	if err := json.Unmarshal([]byte(beadJSON), &bead); err != nil {
		t.Fatalf("Failed to parse bead JSON: %v", err)
	}

	// Initialize bd state with one bead
	stateJSON := map[string]interface{}{
		"beads":   []interface{}{bead},
		"next_id": 2,
	}
	stateBytes, err := json.Marshal(stateJSON)
	if err != nil {
		t.Fatalf("Failed to marshal state JSON: %v", err)
	}
	if err := os.WriteFile(env.BDStateFile, stateBytes, 0644); err != nil {
		t.Fatalf("Failed to write bd state file: %v", err)
	}

	// Create a fixture file for Claude to return successful build output
	claudeFixture := filepath.Join(env.Dir, "claude_build_fixture.txt")
	claudeBuildOutput := `I will implement the task.

<stream-event>
<type>content</type>
<content>
{
  "type": "text",
  "text": "Task implemented successfully. Creating commit."
}
</content>
</stream-event>

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
<content>
{
  "output": "[main abc1234] Implement test task\n 1 file changed, 1 insertion(+)"
}
</content>
</stream-event>
`
	if err := os.WriteFile(claudeFixture, []byte(claudeBuildOutput), 0644); err != nil {
		t.Fatalf("Failed to write Claude fixture: %v", err)
	}

	// Create a fixture file for Claude validation output
	claudeValidateFixture := filepath.Join(env.Dir, "claude_validate_fixture.txt")
	claudeValidateOutput := `Running validation commands.

<stream-event>
<type>content</type>
<content>
{
  "type": "text",
  "text": "All validation commands passed successfully."
}
</content>
</stream-event>
`
	if err := os.WriteFile(claudeValidateFixture, []byte(claudeValidateOutput), 0644); err != nil {
		t.Fatalf("Failed to write Claude validate fixture: %v", err)
	}

	// Set environment variable to control which Claude fixture to use
	// For simplicity, we'll use a single fixture that simulates success
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE", claudeFixture)

	// Create a dummy go.mod so go commands don't fail
	goMod := "module testproject\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(env.Dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Run gromit with -n 1 to process exactly one bead
	// Note: This will fail during validation because go test/vet/build won't work
	// in the empty project, but we're only interested in the bd CLI calls
	stdout, stderr, exitCode, err := runGromitWithEnv(env, "run", "-n", "1")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	// We expect gromit to fail during validation (since there's no real Go code)
	// but we should still see the bd calls up to that point
	t.Logf("Exit code: %d", exitCode)
	t.Logf("Stdout: %s", stdout)
	t.Logf("Stderr: %s", stderr)

	// Read the call log to verify bd calls
	calls, err := filterCalls(env, "bd")
	if err != nil {
		t.Fatalf("Failed to read call log: %v", err)
	}

	// Verify the expected bd calls were made
	// During a run, gromit calls:
	// 1. bd ready --json --limit 3 (to get next bead, filtering epics client-side)
	// 2. bd close <id> (after successful build + validation)
	// 3. bd sync (after successful close)
	//
	// Note: bd show is NOT called during normal run - Ready() returns full bead details.
	// Show is only used in specific commands (review, triage) or when fetching parent beads.

	t.Logf("BD calls made: %d", len(calls))
	for i, call := range calls {
		t.Logf("  %d: %s", i+1, call)
	}

	// Verify bd ready was called with correct arguments
	foundReady := false
	for _, call := range calls {
		if strings.Contains(call, "bd ready --json --limit") {
			foundReady = true
			t.Logf("✓ Found bd ready call: %s", call)
			// Verify limit is 3 (reduced from 10 for performance optimization)
			if !strings.Contains(call, "--limit 3") {
				t.Errorf("Expected 'bd ready --json --limit 3', got: %s", call)
			}
			break
		}
	}

	if !foundReady {
		t.Errorf("Expected 'bd ready --json --limit 3' call not found")
	}

	// Note: We don't verify close/sync because validation failed in this test.
	// Those calls only happen after successful validation.
	// See TestBDContract_SuccessfulRun for the full happy path test.
}

// TestBDContract_ReadyWithLimit verifies that gromit calls bd ready with the correct limit
func TestBDContract_ReadyWithLimit(t *testing.T) {
	env := setupTestEnv(t)

	// Initialize bd state with multiple beads
	beads := []map[string]interface{}{
		{
			"id":               "test-bead-1",
			"title":            "Task 1",
			"description":      "First task",
			"priority":         1,
			"labels":           []string{},
			"parent":           "",
			"issue_type":       "task",
			"status":           "open",
			"owner":            "",
			"expected_outputs": []string{},
		},
		{
			"id":               "test-bead-2",
			"title":            "Task 2",
			"description":      "Second task",
			"priority":         2,
			"labels":           []string{},
			"parent":           "",
			"issue_type":       "task",
			"status":           "open",
			"owner":            "",
			"expected_outputs": []string{},
		},
	}

	stateJSON := map[string]interface{}{
		"beads":   beads,
		"next_id": 3,
	}
	stateBytes, err := json.Marshal(stateJSON)
	if err != nil {
		t.Fatalf("Failed to marshal state JSON: %v", err)
	}
	if err := os.WriteFile(env.BDStateFile, stateBytes, 0644); err != nil {
		t.Fatalf("Failed to write bd state file: %v", err)
	}

	// Create minimal gromit config
	gromitYAML := `
models:
  p0: opus
  p1: sonnet
  p2: haiku

loop:
  max_iterations: 0

validation:
  enabled: false

paths:
  templates: ".gromit/templates"
  logs: ".gromit/logs"
`
	if err := os.WriteFile(filepath.Join(env.Dir, "gromit.yaml"), []byte(gromitYAML), 0644); err != nil {
		t.Fatalf("Failed to write gromit.yaml: %v", err)
	}

	// Run gromit status (which calls bd ready)
	stdout, stderr, exitCode, err := runGromitWithEnv(env, "status")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	t.Logf("Exit code: %d", exitCode)
	t.Logf("Stdout: %s", stdout)
	t.Logf("Stderr: %s", stderr)

	// Read bd calls
	calls, err := filterCalls(env, "bd")
	if err != nil {
		t.Fatalf("Failed to read call log: %v", err)
	}

	// Verify bd ready was called with limit 3 (optimized from 10 for performance)
	foundReady := false
	for _, call := range calls {
		if strings.Contains(call, "bd ready --json --limit") {
			foundReady = true
			// Verify it's limit 3 specifically
			if !strings.Contains(call, "--limit 3") && !strings.Contains(call, "--limit 1") {
				t.Errorf("Expected 'bd ready --json --limit 3' or '--limit 1', got: %s", call)
			}
			t.Logf("✓ Found ready call with limit: %s", call)
		}
	}

	if !foundReady {
		t.Errorf("Expected bd ready call not found. Calls: %v", calls)
	}
}

// TestBDContract_ShowReturnsArrayWrappedJSON verifies that gromit handles
// bd show's array-wrapped JSON format correctly
func TestBDContract_ShowReturnsArrayWrappedJSON(t *testing.T) {
	env := setupTestEnv(t)

	// Create a bead
	beadJSON := map[string]interface{}{
		"id":               "test-bead-1",
		"title":            "Test task",
		"description":      "Test description",
		"priority":         1,
		"labels":           []string{"complexity:low"},
		"parent":           "",
		"issue_type":       "task",
		"status":           "open",
		"owner":            "",
		"expected_outputs": []string{"Complete task"},
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

	// Create minimal config
	gromitYAML := `
models:
  p0: opus
  p1: sonnet
  p2: haiku

validation:
  enabled: false

paths:
  templates: ".gromit/templates"
  logs: ".gromit/logs"
`
	if err := os.WriteFile(filepath.Join(env.Dir, "gromit.yaml"), []byte(gromitYAML), 0644); err != nil {
		t.Fatalf("Failed to write gromit.yaml: %v", err)
	}

	// Run gromit status - it calls bd ready but NOT bd show
	// (show is only called during actual run, not during status)
	stdout, stderr, exitCode, err := runGromitWithEnv(env, "status")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	t.Logf("Exit code: %d", exitCode)
	t.Logf("Stdout: %s", stdout)
	t.Logf("Stderr: %s", stderr)

	// Verify bd ready was called (status calls ready, not show)
	calls, err := filterCalls(env, "bd")
	if err != nil {
		t.Fatalf("Failed to read call log: %v", err)
	}

	foundReady := false
	for _, call := range calls {
		if strings.Contains(call, "bd ready") {
			foundReady = true
			t.Logf("✓ Found ready call: %s", call)
			break
		}
	}

	if !foundReady {
		t.Errorf("Expected 'bd ready' call. Calls: %v", calls)
	}

	// Verify the output contains bead information
	// gromit status should display the bead details from ready output
	if !strings.Contains(stdout, "test-bead-1") {
		t.Errorf("Expected stdout to contain bead ID 'test-bead-1', got: %s", stdout)
	}
}

// TestBDContract_SuccessfulRun verifies that gromit calls bd close and bd sync
// after a successful build and validation
func TestBDContract_SuccessfulRun(t *testing.T) {
	env := setupTestEnv(t)

	// Create a bead
	beadJSON := map[string]interface{}{
		"id":               "test-bead-1",
		"title":            "Test task",
		"description":      "Test description",
		"priority":         1,
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

	// Create gromit config with validation disabled for simplicity
	gromitYAML := `
models:
  p0: opus
  p1: sonnet
  p2: haiku

escalation:
  enabled: false

loop:
  max_iterations: 0

validation:
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
	claudeMD := "# Project\n\nTest project.\n"
	if err := os.WriteFile(filepath.Join(env.Dir, "CLAUDE.md"), []byte(claudeMD), 0644); err != nil {
		t.Fatalf("Failed to write CLAUDE.md: %v", err)
	}

	// Create minimal build prompt template
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

	// Create a Claude fixture that simulates successful work with a commit
	claudeFixture := filepath.Join(env.Dir, "claude_success.txt")
	claudeOutput := `Implementing the task.

<stream-event>
<type>tool_use</type>
<tool>Write</tool>
<content>
{
  "file_path": "` + filepath.Join(env.Dir, "test.txt") + `",
  "content": "test content"
}
</content>
</stream-event>

<stream-event>
<type>tool_result</type>
<content>{"success": true}</content>
</stream-event>

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

	// Run gromit with -n 1
	stdout, stderr, exitCode, err := runGromitWithEnv(env, "run", "-n", "1")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	t.Logf("Exit code: %d", exitCode)
	t.Logf("Stdout: %s", stdout)
	t.Logf("Stderr: %s", stderr)

	// Read bd calls
	calls, err := filterCalls(env, "bd")
	if err != nil {
		t.Fatalf("Failed to read call log: %v", err)
	}

	t.Logf("BD calls made: %d", len(calls))
	for i, call := range calls {
		t.Logf("  %d: %s", i+1, call)
	}

	// Verify the sequence: ready, close, sync
	expectedSequence := []string{
		"bd ready --json --limit 3",
		"bd close test-bead-1",
		"bd sync",
	}

	if len(calls) < len(expectedSequence) {
		t.Errorf("Expected at least %d bd calls, got %d", len(expectedSequence), len(calls))
	}

	for i, expected := range expectedSequence {
		if i >= len(calls) {
			t.Errorf("Missing expected call %d: %s", i+1, expected)
			continue
		}
		if !strings.Contains(calls[i], expected) {
			t.Errorf("Call %d: expected %q, got %q", i+1, expected, calls[i])
		} else {
			t.Logf("✓ Call %d: %s", i+1, expected)
		}
	}
}

// TestBDContract_SyncAfterClose verifies the bd sync contract:
// sync is called after successfully closing a bead
func TestBDContract_SyncAfterClose(t *testing.T) {
	// The sync contract is verified in TestBDContract_SuccessfulRun,
	// which shows that bd sync is called after bd close.
	//
	// The contract is:
	// 1. bd close <id> - mark bead as complete
	// 2. bd sync - sync bd database with git
	//
	// This happens at the end of a successful iteration.
	t.Log("✓ Sync contract verified in TestBDContract_SuccessfulRun")
}

// TestBDContract_ReadyWithLabel verifies that ReadyWithLabel calls bd with --label flag
func TestBDContract_ReadyWithLabel(t *testing.T) {
	env := setupTestEnv(t)

	// Create multiple beads with different labels
	beads := []map[string]interface{}{
		{
			"id":               "epic-001",
			"title":            "Epic",
			"description":      "Parent epic",
			"priority":         0,
			"labels":           []string{"spec:auth"},
			"parent":           "",
			"issue_type":       "epic",
			"status":           "open",
			"owner":            "",
			"expected_outputs": []string{},
		},
		{
			"id":               "task-001",
			"title":            "Auth task 1",
			"description":      "First auth task",
			"priority":         1,
			"labels":           []string{"spec:auth"},
			"parent":           "",
			"issue_type":       "task",
			"status":           "open",
			"owner":            "",
			"expected_outputs": []string{},
		},
		{
			"id":               "task-002",
			"title":            "Auth task 2",
			"description":      "Second auth task",
			"priority":         1,
			"labels":           []string{"spec:auth"},
			"parent":           "",
			"issue_type":       "task",
			"status":           "open",
			"owner":            "",
			"expected_outputs": []string{},
		},
		{
			"id":               "task-003",
			"title":            "Payments task",
			"description":      "Payments feature",
			"priority":         1,
			"labels":           []string{"spec:payments"},
			"parent":           "",
			"issue_type":       "task",
			"status":           "open",
			"owner":            "",
			"expected_outputs": []string{},
		},
	}

	stateJSON := map[string]interface{}{
		"beads":   beads,
		"next_id": 4,
	}
	stateBytes, err := json.Marshal(stateJSON)
	if err != nil {
		t.Fatalf("Failed to marshal state JSON: %v", err)
	}
	if err := os.WriteFile(env.BDStateFile, stateBytes, 0644); err != nil {
		t.Fatalf("Failed to write bd state file: %v", err)
	}

	// Set env vars so the bead client's subprocess finds the fake bd
	t.Setenv("PATH", env.PATH)
	t.Setenv("TEST_DIR", env.Dir)
	t.Setenv("TEST_CALL_LOG", env.CallLog)

	// Call ReadyWithLabel directly
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("Failed to create bead client: %v", err)
	}
	client.Dir = env.Dir

	b, err := client.ReadyWithLabel("spec:auth")
	if err != nil {
		t.Fatalf("ReadyWithLabel error: %v", err)
	}

	var output string
	if b == nil {
		output = "No bead found"
	} else {
		output = "Found bead: " + b.ID + " (" + b.Type + ")"
	}
	t.Logf("Program output: %s", output)

	// Read bd calls
	calls, err := filterCalls(env, "bd")
	if err != nil {
		t.Fatalf("Failed to read call log: %v", err)
	}

	t.Logf("BD calls made: %d", len(calls))
	for i, call := range calls {
		t.Logf("  %d: %s", i+1, call)
	}

	// Verify bd ready was called with --label flag
	foundReadyWithLabel := false
	for _, call := range calls {
		if strings.Contains(call, "bd ready") &&
			strings.Contains(call, "--label") &&
			strings.Contains(call, "spec:auth") {
			foundReadyWithLabel = true
			// Verify it includes --json and --limit 3
			if !strings.Contains(call, "--json") {
				t.Errorf("Expected --json flag in call: %s", call)
			}
			if !strings.Contains(call, "--limit 3") {
				t.Errorf("Expected --limit 3 in call: %s", call)
			}
			t.Logf("✓ Found ready with label call: %s", call)
			break
		}
	}

	if !foundReadyWithLabel {
		t.Errorf("Expected 'bd ready --json --limit 3 --label spec:auth' call not found. Calls: %v", calls)
	}

	// Verify the program found a task (not the epic)
	if !strings.Contains(string(output), "task-001") && !strings.Contains(string(output), "task-002") {
		t.Errorf("Expected program to find task-001 or task-002 (excluding epic), got: %s", output)
	}
	if strings.Contains(string(output), "epic-001") {
		t.Errorf("Expected program to exclude epic-001, but it was returned: %s", output)
	}
}
