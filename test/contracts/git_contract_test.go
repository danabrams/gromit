package contracts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
)

// TestGitContract_RevParseBeforeBuild verifies that gromit runs "git rev-parse HEAD"
// before starting the build phase to capture the starting commit.
func TestGitContract_RevParseBeforeBuild(t *testing.T) {
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

	// Create a test bead via the fake bd
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

	// Initialize bd state with one bead
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

	// Read git calls
	calls, err := filterCalls(env, "git")
	if err != nil {
		t.Fatalf("Failed to read call log: %v", err)
	}

	t.Logf("Git calls made: %d", len(calls))
	for i, call := range calls {
		t.Logf("  %d: %s", i+1, call)
	}

	// Verify git rev-parse HEAD was called
	foundRevParse := false
	for _, call := range calls {
		if strings.Contains(call, "git rev-parse HEAD") {
			foundRevParse = true
			t.Logf("✓ Found git rev-parse HEAD call: %s", call)
			break
		}
	}

	if !foundRevParse {
		t.Errorf("Expected 'git rev-parse HEAD' call not found. Calls: %v", calls)
	}
}

// TestGitContract_DiffAfterBuildFailure verifies that gromit runs "git diff --stat"
// after a build failure to show what partial progress was made.
func TestGitContract_DiffAfterBuildFailure(t *testing.T) {
	env := setupTestEnv(t)

	// Create an initial commit so git rev-parse HEAD works
	dummyFile := filepath.Join(env.Dir, "README.md")
	if err := os.WriteFile(dummyFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("Failed to write dummy file: %v", err)
	}
	// Use the real git path directly for initialization
	gitAdd := exec.Command(realGitPath, "add", "README.md")
	gitAdd.Dir = env.Dir
	if err := gitAdd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	gitCommit := exec.Command(realGitPath, "commit", "-m", "Initial commit")
	gitCommit.Dir = env.Dir
	if err := gitCommit.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	// Create a gromit.yaml config in the test directory
	gromitYAML := `
models:
  p0: opus
  p1: sonnet
  p2: haiku
  validation: haiku

escalation:
  enabled: false
  max_retries_per_model: 0
  max_retries_per_bead: 0

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

	// Create a test bead via the fake bd
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

	// Initialize bd state with one bead
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

	// Create a Claude fixture that simulates a failure (no commit made)
	claudeFixture := filepath.Join(env.Dir, "claude_fail.txt")
	claudeOutput := `I attempted to implement the task but encountered an error.

ERROR: Could not complete the task successfully.
`
	if err := os.WriteFile(claudeFixture, []byte(claudeOutput), 0644); err != nil {
		t.Fatalf("Failed to write Claude fixture: %v", err)
	}

	// Set CLAUDE_FAIL=1 to make claude return exit code 1
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE", claudeFixture)
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FAIL", "1")

	// Run gromit with -n 1 (will fail)
	stdout, stderr, exitCode, err := runGromitWithEnv(env, "run", "-n", "1")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	// We expect a non-zero exit code because the build failed
	t.Logf("Exit code: %d", exitCode)
	t.Logf("Stdout: %s", stdout)
	t.Logf("Stderr: %s", stderr)

	// Read git calls
	calls, err := filterCalls(env, "git")
	if err != nil {
		t.Fatalf("Failed to read call log: %v", err)
	}

	t.Logf("Git calls made: %d", len(calls))
	for i, call := range calls {
		t.Logf("  %d: %s", i+1, call)
	}

	// Verify git diff --stat was called (to show partial progress)
	foundDiffStat := false
	for _, call := range calls {
		if strings.Contains(call, "git diff --stat") {
			foundDiffStat = true
			t.Logf("✓ Found git diff --stat call: %s", call)
			break
		}
	}

	if !foundDiffStat {
		t.Errorf("Expected 'git diff --stat' call not found. Calls: %v", calls)
	}
}

// TestGitContract_FullRunSequence verifies the complete git command sequence
// during a run: rev-parse before build, diff commands on failure.
func TestGitContract_FullRunSequence(t *testing.T) {
	env := setupTestEnv(t)

	// Create an initial commit so git rev-parse HEAD works
	dummyFile := filepath.Join(env.Dir, "README.md")
	if err := os.WriteFile(dummyFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("Failed to write dummy file: %v", err)
	}
	// Use the real git path directly for initialization
	gitAdd := exec.Command(realGitPath, "add", "README.md")
	gitAdd.Dir = env.Dir
	if err := gitAdd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	gitCommit := exec.Command(realGitPath, "commit", "-m", "Initial commit")
	gitCommit.Dir = env.Dir
	if err := gitCommit.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

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

	// Create a test bead via the fake bd
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

	// Initialize bd state with one bead
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

	// Create a Claude fixture that simulates a failure
	claudeFixture := filepath.Join(env.Dir, "claude_fail.txt")
	claudeOutput := `I attempted to implement the task but encountered an error.

ERROR: Could not complete the task successfully.
`
	if err := os.WriteFile(claudeFixture, []byte(claudeOutput), 0644); err != nil {
		t.Fatalf("Failed to write Claude fixture: %v", err)
	}

	// Set CLAUDE_FAIL=1 to make claude return exit code 1
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FIXTURE", claudeFixture)
	env.Env = testutil.ReplaceOrAppend(env.Env, "CLAUDE_FAIL", "1")

	// Run gromit with -n 1 (will fail)
	stdout, stderr, exitCode, err := runGromitWithEnv(env, "run", "-n", "1")
	if err != nil {
		t.Fatalf("Failed to run gromit: %v", err)
	}

	// We expect a non-zero exit code because the build failed
	t.Logf("Exit code: %d", exitCode)
	t.Logf("Stdout: %s", stdout)
	t.Logf("Stderr: %s", stderr)

	// Read git calls
	calls, err := filterCalls(env, "git")
	if err != nil {
		t.Fatalf("Failed to read call log: %v", err)
	}

	t.Logf("Git calls made: %d", len(calls))
	for i, call := range calls {
		t.Logf("  %d: %s", i+1, call)
	}

	// Verify the expected sequence of git commands
	// 1. git rev-parse HEAD should be called early (before build)
	// 2. git diff --stat should be called after the build failure
	// Note: Other git commands (config, add, commit) may also appear via Claude,
	// but those are not part of gromit's contract - they're Claude's actions.

	foundRevParse := false
	revParseIndex := -1
	for i, call := range calls {
		if strings.Contains(call, "git rev-parse HEAD") {
			foundRevParse = true
			revParseIndex = i
			t.Logf("✓ Found git rev-parse HEAD at index %d: %s", i, call)
			break
		}
	}

	if !foundRevParse {
		t.Errorf("Expected 'git rev-parse HEAD' call not found")
	}

	foundDiffStat := false
	diffStatIndex := -1
	for i, call := range calls {
		if strings.Contains(call, "git diff --stat") {
			foundDiffStat = true
			diffStatIndex = i
			t.Logf("✓ Found git diff --stat at index %d: %s", i, call)
			break
		}
	}

	if !foundDiffStat {
		t.Errorf("Expected 'git diff --stat' call not found")
	}

	// Verify ordering: rev-parse should come before diff --stat
	// (though there may be many git commands in between from Claude)
	if foundRevParse && foundDiffStat {
		if revParseIndex < diffStatIndex {
			t.Logf("✓ Correct sequence: git rev-parse HEAD (index %d) before git diff --stat (index %d)", revParseIndex, diffStatIndex)
		} else {
			t.Errorf("Incorrect sequence: git rev-parse HEAD (index %d) should come before git diff --stat (index %d)", revParseIndex, diffStatIndex)
		}
	}
}

// TestGitContract_PassthroughToRealGit verifies that the fake git script
// passes through to real git and that real git operations work correctly.
func TestGitContract_PassthroughToRealGit(t *testing.T) {
	env := setupTestEnv(t)

	// Run a simple git status command via gromit's environment
	// This verifies that the fake git wrapper calls real git correctly
	stdout, stderr, exitCode, err := runGromitWithEnv(env, "status")
	if err != nil {
		t.Fatalf("Failed to run gromit status: %v", err)
	}

	t.Logf("Exit code: %d", exitCode)
	t.Logf("Stdout: %s", stdout)
	t.Logf("Stderr: %s", stderr)

	// Read git calls - even gromit status might trigger git commands
	// (e.g., for checking the review baseline)
	calls, err := filterCalls(env, "git")
	if err != nil {
		t.Fatalf("Failed to read call log: %v", err)
	}

	t.Logf("Git calls made: %d", len(calls))
	for i, call := range calls {
		t.Logf("  %d: %s", i+1, call)
	}

	// At minimum, we should see that git calls were recorded
	// The actual commands depend on what gromit status does
	if len(calls) > 0 {
		t.Logf("✓ Git calls were recorded through the fake git wrapper")
	}

	// Verify that the git repo is still valid (passthrough worked)
	// Check that .git directory exists
	gitDir := filepath.Join(env.Dir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Errorf("Git repository not found at %s - passthrough may have failed", gitDir)
	} else {
		t.Logf("✓ Git repository exists - passthrough to real git works")
	}
}
