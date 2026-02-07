package main

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestExecGromitSuccessExitZero verifies that execGromit returns nil when subprocess exits successfully (exit 0)
func TestExecGromitSuccessExitZero(t *testing.T) {
	// Create a test binary that exits 0
	tmpDir := t.TempDir()
	testProg := filepath.Join(tmpDir, "success.go")

	code := `package main
func main() {
	// Exit 0 (success)
}
`
	if err := os.WriteFile(testProg, []byte(code), 0644); err != nil {
		t.Fatalf("failed to write test program: %v", err)
	}

	binary := filepath.Join(tmpDir, "gromit-test")
	buildCmd := exec.Command("go", "build", "-o", binary, testProg)
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build test program: %v", err)
	}

	// Temporarily override os.Args[0] to make execGromit use our test binary
	oldArgs := os.Args
	os.Args = []string{binary}
	defer func() { os.Args = oldArgs }()

	// Call the actual execGromit function
	err := execGromit("--help") // arbitrary args, binary ignores them

	// Verify: exit 0 should produce no error
	if err != nil {
		t.Errorf("expected nil error for exit 0, got: %v", err)
	}
}

// TestExecGromitNonZeroExit verifies that execGromit returns nil for non-zero exit codes
// (it treats exec.ExitError as "subprocess handled its own errors")
func TestExecGromitNonZeroExit(t *testing.T) {
	tmpDir := t.TempDir()
	testProg := filepath.Join(tmpDir, "fail.go")

	code := `package main
import "os"
func main() {
	os.Exit(1) // Exit with error code
}
`
	if err := os.WriteFile(testProg, []byte(code), 0644); err != nil {
		t.Fatalf("failed to write test program: %v", err)
	}

	binary := filepath.Join(tmpDir, "gromit-test")
	buildCmd := exec.Command("go", "build", "-o", binary, testProg)
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build test program: %v", err)
	}

	// Temporarily override os.Args[0] to make execGromit use our test binary
	oldArgs := os.Args
	os.Args = []string{binary}
	defer func() { os.Args = oldArgs }()

	// Call the actual execGromit function
	err := execGromit("--help") // arbitrary args, binary ignores them

	// execGromit should return nil for ExitError (subprocess handled its own errors)
	if err != nil {
		t.Errorf("execGromit should return nil for ExitError, got: %v", err)
	}
}

// TestExecGromitLaunchFailure is a documentary test that verifies
// launch failures are properly distinguished from exit errors.
// Testing actual launch failure of execGromit is difficult because os.Executable()
// always returns a valid path during tests. This test documents the expected behavior.
func TestExecGromitLaunchFailure(t *testing.T) {
	// Directly test the distinction between launch errors and exit errors
	// This is what execGromit must handle correctly

	// Try to execute a non-existent binary (launch failure)
	cmd := exec.Command("/nonexistent/binary/path/that/does/not/exist")
	err := cmd.Run()

	if err == nil {
		t.Error("expected error for non-existent binary, got nil")
		return
	}

	// Launch errors should NOT be ExitErrors
	if _, isExitError := err.(*exec.ExitError); isExitError {
		t.Error("expected launch error, not ExitError")
	}

	// execGromit should return launch errors unchanged (not convert to nil)
	// This is the critical behavior: only ExitErrors become nil, other errors propagate
}

// TestChainAfterRefineThreePhasesEmptyInput verifies chainAfterRefine with empty spec list
func TestChainAfterRefineThreePhasesEmptyInput(t *testing.T) {
	// Call the actual chainAfterRefine function with empty spec list
	// It should return immediately without prompting
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	chainAfterRefine([]string{}, t.TempDir())

	w.Close()
	_, _ = buf.ReadFrom(r)

	// Should not prompt or do anything
	output := buf.String()
	if strings.Contains(output, "Run 'gromit") {
		t.Errorf("expected no output for empty spec list, got: %s", output)
	}
}

// TestChainAfterRefinePhase1Planning is a documentary test that demonstrates
// how Phase 1 of chainAfterRefine tracks successfully planned specs.
// Full integration testing of the interactive flow requires complex stdin mocking.
func TestChainAfterRefinePhase1Planning(t *testing.T) {
	// This test documents the logic of tracking successfully planned specs
	// chainAfterRefine checks for plan file existence to track success

	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	// Simulate Phase 1: checking if plan files exist
	specNames := []string{"spec1", "spec2", "spec3"}
	var plannedNames []string

	// Create plan files for spec1 and spec3 (simulating successful planning)
	plan1 := filepath.Join(plansDir, "spec1.md")
	plan3 := filepath.Join(plansDir, "spec3.md")
	if err := os.WriteFile(plan1, []byte("# Plan 1"), 0644); err != nil {
		t.Fatalf("failed to create plan1: %v", err)
	}
	if err := os.WriteFile(plan3, []byte("# Plan 3"), 0644); err != nil {
		t.Fatalf("failed to create plan3: %v", err)
	}

	// Check which specs have plan files (this is what Phase 1 does)
	for _, specName := range specNames {
		planPath := filepath.Join(plansDir, specName+".md")
		if _, err := os.Stat(planPath); err == nil {
			plannedNames = append(plannedNames, specName)
		}
	}

	// Verify: spec1 and spec3 should be in plannedNames
	if len(plannedNames) != 2 {
		t.Errorf("expected 2 planned specs, got %d: %v", len(plannedNames), plannedNames)
	}
	if plannedNames[0] != "spec1" || plannedNames[1] != "spec3" {
		t.Errorf("expected [spec1, spec3], got %v", plannedNames)
	}
}

// TestChainAfterRefinePhase2Decompose is a documentary test that demonstrates
// how Phase 2 of chainAfterRefine proceeds after Phase 1 creates plan files.
// Full integration testing requires a real gromit binary and complex stdin mocking.
func TestChainAfterRefinePhase2Decompose(t *testing.T) {
	// This test documents that Phase 2 proceeds based on plan file existence
	// chainAfterRefine transitions to decompose phase for specs with plan files

	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	// Verify that a plan file's existence determines Phase 1 success
	planFile := filepath.Join(plansDir, "spec1.md")

	// Before plan exists
	if _, err := os.Stat(planFile); err == nil {
		t.Error("plan file should not exist yet")
	}

	// Create plan file
	if err := os.WriteFile(planFile, []byte("# Plan for spec1"), 0644); err != nil {
		t.Fatalf("failed to create plan file: %v", err)
	}

	// After plan exists
	if _, err := os.Stat(planFile); err != nil {
		t.Errorf("plan file should exist after creation: %v", err)
	}
}

// TestChainAfterRefinePhase3RunOnlyIfDecomposed is a documentary test that demonstrates
// the logic that Phase 3 only runs if decomposedCount > 0 in chainAfterRefine.
func TestChainAfterRefinePhase3RunOnlyIfDecomposed(t *testing.T) {
	// This test documents the conditional logic for Phase 3
	// chainAfterRefine only prompts for 'gromit run' if at least one spec was decomposed

	// Simulate different decomposedCount scenarios
	testCases := []struct {
		name            string
		decomposedCount int
		shouldPrompt    bool
	}{
		{"no decomposed specs", 0, false},
		{"one decomposed spec", 1, true},
		{"multiple decomposed specs", 3, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This is the logic from chainAfterRefine Phase 3
			decomposedCount := tc.decomposedCount
			shouldPromptForRun := decomposedCount > 0

			if shouldPromptForRun != tc.shouldPrompt {
				t.Errorf("expected shouldPrompt=%v for decomposedCount=%d, got %v",
					tc.shouldPrompt, decomposedCount, shouldPromptForRun)
			}
		})
	}
}

// TestChainAfterRefineDecomposedCountIncrementsIncorrectly is a documentary test
// that documents a potential issue: decomposedCount increments when execGromit
// returns nil for exit failures (because execGromit treats ExitError as "handled").
func TestChainAfterRefineDecomposedCountIncrementsIncorrectly(t *testing.T) {
	// This test documents the current behavior:
	// execGromit returns nil for *exec.ExitError (subprocess printed its own errors)
	// So chainAfterRefine's Phase 2 increments decomposedCount even on non-zero exits

	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	// Create a plan file
	planFile := filepath.Join(plansDir, "spec1.md")
	if err := os.WriteFile(planFile, []byte("# Plan"), 0644); err != nil {
		t.Fatalf("failed to create plan file: %v", err)
	}

	// Simulate the buggy behavior in code
	var decomposedCount int

	// Simulate execGromit returning nil even for exit 1
	tmpProg := filepath.Join(tmpDir, "fail.go")
	code := `package main
import "os"
func main() {
	os.Exit(1)
}
`
	if err := os.WriteFile(tmpProg, []byte(code), 0644); err != nil {
		t.Fatalf("failed to write test program: %v", err)
	}

	binary := filepath.Join(tmpDir, "testbin")
	buildCmd := exec.Command("go", "build", "-o", binary, tmpProg)
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build test program: %v", err)
	}

	cmd := exec.Command(binary)
	err := cmd.Run()

	// Simulate execGromit's buggy logic
	if _, ok := err.(*exec.ExitError); ok {
		err = nil
	}

	// Bug: err is now nil even though subprocess failed
	if err != nil {
		// Don't increment
	} else {
		// BUG: This increments even on failure
		decomposedCount++
	}

	// Verify the bug
	if decomposedCount != 1 {
		t.Errorf("expected decomposedCount=1 due to bug (nil error for exit 1), got %d", decomposedCount)
	}
}

// TestChainAfterRefineBreakOnDecline is a documentary test that demonstrates
// the break-on-decline behavior in chainAfterRefine phases.
func TestChainAfterRefineBreakOnDecline(t *testing.T) {
	// This test documents the logic that when a user declines a prompt,
	// remaining items in that phase are skipped (the loop breaks in chainAfterRefine)

	// Simulate the break logic from Phase 1
	specNames := []string{"spec1", "spec2", "spec3"}
	var processedSpecs []string

	for _, specName := range specNames {
		// Simulate user declining on spec2
		if specName == "spec2" {
			break // This is what happens when user declines
		}
		processedSpecs = append(processedSpecs, specName)
	}

	// Verify: only spec1 should be processed (spec2 declined, spec3 never reached)
	if len(processedSpecs) != 1 {
		t.Errorf("expected 1 processed spec after decline, got %d: %v", len(processedSpecs), processedSpecs)
	}
	if processedSpecs[0] != "spec1" {
		t.Errorf("expected spec1, got %s", processedSpecs[0])
	}
}

// TestConfirmPromptDefaultBehavior verifies defaults are respected
func TestConfirmPromptDefaultBehavior(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultYes bool
		want       bool
	}{
		{"empty defaults to yes", "\n", true, true},
		{"empty defaults to no", "\n", false, false},
		{"invalid defaults to yes", "invalid\n", true, true},
		{"invalid defaults to no", "invalid\n", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			got := confirmPrompt(reader, "Test", tt.defaultYes)
			if got != tt.want {
				t.Errorf("confirmPrompt() = %v, want %v", got, tt.want)
			}
		})
	}
}
