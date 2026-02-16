//go:build integration

package main

import (
	"bufio"
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
	confirmCalled := false
	executeCalled := false

	confirm := func(prompt string, defaultYes bool) bool {
		confirmCalled = true
		return false
	}
	execute := func(args ...string) error {
		executeCalled = true
		return nil
	}

	chainAfterRefine([]string{}, t.TempDir(), confirm, execute)

	// Should not prompt or execute anything
	if confirmCalled {
		t.Error("expected no confirm calls for empty spec list")
	}
	if executeCalled {
		t.Error("expected no execute calls for empty spec list")
	}
}

// TestChainAfterRefinePhase1Planning verifies Phase 1 tracks successfully planned specs
func TestChainAfterRefinePhase1Planning(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	specNames := []string{"spec1", "spec2", "spec3"}
	var executeCalls []string

	// Confirm always says yes
	confirm := func(prompt string, defaultYes bool) bool {
		return true
	}

	// Execute creates plan files for spec1 and spec3, fails for spec2
	execute := func(args ...string) error {
		executeCalls = append(executeCalls, strings.Join(args, " "))
		if len(args) >= 2 && args[0] == "plan" {
			specName := args[1]
			if specName == "spec1" || specName == "spec3" {
				// Create plan file to simulate success
				planPath := filepath.Join(plansDir, specName+".md")
				return os.WriteFile(planPath, []byte("# Plan"), 0644)
			}
			// spec2 fails - no plan file created
			return nil
		}
		return nil
	}

	chainAfterRefine(specNames, plansDir, confirm, execute)

	// Verify: all 3 specs were offered for planning
	if len(executeCalls) < 3 {
		t.Errorf("expected at least 3 execute calls for planning, got %d", len(executeCalls))
	}

	// Verify: plan files exist for spec1 and spec3, not spec2
	if _, err := os.Stat(filepath.Join(plansDir, "spec1.md")); err != nil {
		t.Error("expected spec1 plan file to exist")
	}
	if _, err := os.Stat(filepath.Join(plansDir, "spec3.md")); err != nil {
		t.Error("expected spec3 plan file to exist")
	}
	if _, err := os.Stat(filepath.Join(plansDir, "spec2.md")); err == nil {
		t.Error("expected spec2 plan file to not exist")
	}
}

// TestChainAfterRefinePhase2Decompose verifies Phase 2 offers decompose for planned specs
func TestChainAfterRefinePhase2Decompose(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	specNames := []string{"spec1", "spec2"}
	var decomposeOffered []string

	// Confirm always says yes
	confirm := func(prompt string, defaultYes bool) bool {
		// Track which specs were offered for decompose
		if strings.Contains(prompt, "decompose") {
			for _, name := range specNames {
				if strings.Contains(prompt, name) {
					decomposeOffered = append(decomposeOffered, name)
					break
				}
			}
		}
		return true
	}

	// Execute creates plan files in Phase 1
	execute := func(args ...string) error {
		if len(args) >= 2 && args[0] == "plan" {
			specName := args[1]
			planPath := filepath.Join(plansDir, specName+".md")
			return os.WriteFile(planPath, []byte("# Plan"), 0644)
		}
		return nil
	}

	chainAfterRefine(specNames, plansDir, confirm, execute)

	// Verify: decompose was offered for both planned specs
	if len(decomposeOffered) != 2 {
		t.Errorf("expected decompose offered for 2 specs, got %d: %v", len(decomposeOffered), decomposeOffered)
	}
	if !sliceContains(decomposeOffered, "spec1") || !sliceContains(decomposeOffered, "spec2") {
		t.Errorf("expected decompose offered for spec1 and spec2, got %v", decomposeOffered)
	}
}

func sliceContains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// TestChainAfterRefinePhase3RunOnlyIfDecomposed verifies Phase 3 only prompts for run when specs decomposed
func TestChainAfterRefinePhase3RunOnlyIfDecomposed(t *testing.T) {
	testCases := []struct {
		name              string
		createDecomposed  bool
		expectRunPrompted bool
	}{
		{"no decomposed specs", false, false},
		{"one decomposed spec", true, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			plansDir := filepath.Join(tmpDir, "plans")
			if err := os.MkdirAll(plansDir, 0755); err != nil {
				t.Fatalf("failed to create plans dir: %v", err)
			}

			specNames := []string{"spec1"}
			runPrompted := false

			confirm := func(prompt string, defaultYes bool) bool {
				if strings.Contains(prompt, "gromit run") {
					runPrompted = true
				}
				return true
			}

			execute := func(args ...string) error {
				if len(args) >= 2 && args[0] == "plan" {
					// Create plan file
					specName := args[1]
					planPath := filepath.Join(plansDir, specName+".md")
					return os.WriteFile(planPath, []byte("# Plan"), 0644)
				}
				if len(args) >= 2 && args[0] == "decompose" && tc.createDecomposed {
					// Mark plan as decomposed
					specName := args[1]
					planPath := filepath.Join(plansDir, specName+".md")
					content := "---\ndecomposed: true\n---\n# Plan"
					return os.WriteFile(planPath, []byte(content), 0644)
				}
				return nil
			}

			chainAfterRefine(specNames, plansDir, confirm, execute)

			if runPrompted != tc.expectRunPrompted {
				t.Errorf("expected runPrompted=%v, got %v", tc.expectRunPrompted, runPrompted)
			}
		})
	}
}

// TestChainAfterRefineDecomposedCountWithExecuteReturningNil verifies behavior when execute returns nil for failures
func TestChainAfterRefineDecomposedCountWithExecuteReturningNil(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	specNames := []string{"spec1"}
	runPrompted := false

	confirm := func(prompt string, defaultYes bool) bool {
		if strings.Contains(prompt, "gromit run") {
			runPrompted = true
		}
		return true
	}

	// Execute creates plan file, but decompose fails (returns nil without updating frontmatter)
	execute := func(args ...string) error {
		if len(args) >= 2 && args[0] == "plan" {
			specName := args[1]
			planPath := filepath.Join(plansDir, specName+".md")
			return os.WriteFile(planPath, []byte("# Plan"), 0644)
		}
		if len(args) >= 2 && args[0] == "decompose" {
			// Return nil but don't mark as decomposed (simulating exit failure but nil error)
			return nil
		}
		return nil
	}

	chainAfterRefine(specNames, plansDir, confirm, execute)

	// Verify: run should NOT be prompted because decompose didn't actually succeed
	// (frontmatter not updated with decomposed: true)
	if runPrompted {
		t.Error("expected run NOT to be prompted when decompose fails to set decomposed flag")
	}
}

// TestChainAfterRefineBreakOnDecline verifies that declining skips remaining items in phase
func TestChainAfterRefineBreakOnDecline(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("failed to create plans dir: %v", err)
	}

	specNames := []string{"spec1", "spec2", "spec3"}
	var promptedSpecs []string
	var executedSpecs []string

	// Decline on spec2
	confirm := func(prompt string, defaultYes bool) bool {
		for _, name := range specNames {
			if strings.Contains(prompt, name) {
				promptedSpecs = append(promptedSpecs, name)
				// Decline on spec2
				if name == "spec2" {
					return false
				}
				return true
			}
		}
		return true
	}

	execute := func(args ...string) error {
		if len(args) >= 2 && args[0] == "plan" {
			specName := args[1]
			executedSpecs = append(executedSpecs, specName)
		}
		return nil
	}

	chainAfterRefine(specNames, plansDir, confirm, execute)

	// Verify: spec1 and spec2 were prompted, but only spec1 was executed
	if len(promptedSpecs) != 2 {
		t.Errorf("expected 2 specs prompted (spec1, spec2), got %d: %v", len(promptedSpecs), promptedSpecs)
	}
	if len(executedSpecs) != 1 || executedSpecs[0] != "spec1" {
		t.Errorf("expected only spec1 executed, got %v", executedSpecs)
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
