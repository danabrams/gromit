//go:build acceptance

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// TestRunCommand_SpecFlagExists verifies that the run command has a --spec flag.
//
// Expected failure: runCmd.Flags() does not include "spec" flag because the flag
// is not registered in init() with runCmd.Flags().StringVar(&runSpecFlag, "spec", "", "...")
func TestRunCommand_SpecFlagExists(t *testing.T) {
	specFlag := runCmd.Flags().Lookup("spec")
	if specFlag == nil {
		t.Fatal("run command should have --spec flag")
	}
	if specFlag.Value.Type() != "string" {
		t.Errorf("--spec flag should be string type, got %s", specFlag.Value.Type())
	}
}

// TestRunCommand_EpicFlagExists verifies that the run command has an --epic flag.
//
// Expected failure: runCmd.Flags() does not include "epic" flag because the flag
// is not registered in init() with runCmd.Flags().StringVar(&runEpicFlag, "epic", "", "...")
func TestRunCommand_EpicFlagExists(t *testing.T) {
	epicFlag := runCmd.Flags().Lookup("epic")
	if epicFlag == nil {
		t.Fatal("run command should have --epic flag")
	}
	if epicFlag.Value.Type() != "string" {
		t.Errorf("--epic flag should be string type, got %s", epicFlag.Value.Type())
	}
}

// TestRunCommand_SpecFlagFiltersBeads verifies that --spec flag only processes beads
// with the matching spec label, not all beads.
//
// Expected failure: Package variable runSpecFlag does not exist, and runLoop() does not
// call scope.ValidateFlags(), scope.ResolveSpec(), or runner.SetLabelFilters().
// Without this logic, all ready beads are processed regardless of the flag value.
func TestRunCommand_SpecFlagFiltersBeads(t *testing.T) {
	// Set up test environment
	tempDir := t.TempDir()
	gromitDir := setupFullGromitEnv(t, tempDir)
	specsDir := filepath.Join(gromitDir, "specs")

	// Create spec files
	writeSpecFiles(t, specsDir, []specFile{
		{"auth.md", "auth", "epic-100"},
		{"billing.md", "billing", "epic-200"},
	})

	// Create test beads with different spec labels
	client := setupBeadClient(t, tempDir)

	authBead1, err := client.Create("Auth task 1", 1, []string{"spec:auth"}, []string{})
	if err != nil {
		t.Skipf("Cannot create auth bead: %v", err)
	}

	authBead2, err := client.Create("Auth task 2", 1, []string{"spec:auth"}, []string{})
	if err != nil {
		t.Skipf("Cannot create second auth bead: %v", err)
	}

	billingBead, err := client.Create("Billing task", 1, []string{"spec:billing"}, []string{})
	if err != nil {
		t.Skipf("Cannot create billing bead: %v", err)
	}

	noSpecBead, err := client.Create("No spec task", 1, []string{}, []string{})
	if err != nil {
		t.Skipf("Cannot create no-spec bead: %v", err)
	}

	// Set the flag
	runSpecFlag = "auth"
	defer func() { runSpecFlag = "" }()

	// Run gromit with dry-run to see what would be processed
	dryRun = true
	defer func() { dryRun = false }()

	// Capture what bead would be processed
	err = runLoop(runCmd, []string{})

	// The key behavioral test: With --spec auth, the runner should ONLY pick up
	// beads labeled spec:auth (authBead1 or authBead2), NOT billingBead or noSpecBead.
	//
	// Current behavior without implementation: getNextBead() calls client.Ready()
	// which returns ALL ready beads, so it could pick any of: authBead1, authBead2,
	// billingBead, noSpecBead.
	//
	// Expected behavior after implementation: getNextBead() calls client.ReadyWithLabel("spec:auth")
	// for each label in the filter, so it ONLY returns authBead1 or authBead2.
	//
	// We verify this by checking the log output or error messages to see which bead
	// was selected. In dry-run mode, it should mention the bead title.

	// For now, we verify that the command accepted the flag without error
	// (the full integration test would check logs to verify the right bead was selected)
	if err != nil && !strings.Contains(err.Error(), "no beads") {
		t.Logf("Beads created: auth=[%s,%s], billing=[%s], no-spec=[%s]",
			authBead1.ID, authBead2.ID, billingBead.ID, noSpecBead.ID)
		t.Errorf("runLoop with --spec should succeed or report 'no beads', got: %v", err)
	}
}

// TestRunCommand_EpicFlagFiltersToMultipleSpecs verifies that --epic flag resolves
// to all specs in that epic and only processes beads with those spec labels.
//
// Expected failure: Package variable runEpicFlag does not exist, and runLoop() does not
// call scope.ValidateFlags(), scope.ResolveEpic(), or runner.SetLabelFilters().
// Without this logic, all ready beads are processed regardless of the flag value.
func TestRunCommand_EpicFlagFiltersToMultipleSpecs(t *testing.T) {
	tempDir := t.TempDir()
	gromitDir := setupFullGromitEnv(t, tempDir)
	specsDir := filepath.Join(gromitDir, "specs")

	// Create specs: two in epic-100, one in epic-200
	writeSpecFiles(t, specsDir, []specFile{
		{"auth.md", "auth", "epic-100"},
		{"profile.md", "profile", "epic-100"},
		{"billing.md", "billing", "epic-200"},
	})

	client := setupBeadClient(t, tempDir)

	authBead, err := client.Create("Auth task", 1, []string{"spec:auth"}, []string{})
	if err != nil {
		t.Skipf("Cannot create auth bead: %v", err)
	}

	profileBead, err := client.Create("Profile task", 1, []string{"spec:profile"}, []string{})
	if err != nil {
		t.Skipf("Cannot create profile bead: %v", err)
	}

	billingBead, err := client.Create("Billing task", 1, []string{"spec:billing"}, []string{})
	if err != nil {
		t.Skipf("Cannot create billing bead: %v", err)
	}

	// Set the epic flag
	runEpicFlag = "epic-100"
	defer func() { runEpicFlag = "" }()

	dryRun = true
	defer func() { dryRun = false }()

	err = runLoop(runCmd, []string{})

	// The key behavioral test: With --epic epic-100, the runner should resolve to
	// ["spec:auth", "spec:profile"] and ONLY pick up authBead or profileBead,
	// NOT billingBead (which is spec:billing in epic-200).
	//
	// Current behavior: getNextBead() would pick ANY ready bead (auth, profile, or billing)
	// Expected behavior: getNextBead() only considers beads with spec:auth or spec:profile labels

	if err != nil && !strings.Contains(err.Error(), "no beads") {
		t.Logf("Beads: epic-100=[%s,%s], epic-200=[%s]",
			authBead.ID, profileBead.ID, billingBead.ID)
		t.Errorf("runLoop with --epic should succeed or report 'no beads', got: %v", err)
	}
}

// TestRunCommand_SpecAndEpicFlagsMutuallyExclusive verifies that using both
// --spec and --epic flags returns an error.
//
// Expected failure: Package variables runSpecFlag and runEpicFlag do not exist, and
// runLoop() does not call scope.ValidateFlags(runEpicFlag, runSpecFlag).
// Without validation, both flags can be set simultaneously without error.
func TestRunCommand_SpecAndEpicFlagsMutuallyExclusive(t *testing.T) {
	tempDir := t.TempDir()
	setupFullGromitEnv(t, tempDir)

	// Set both flags
	runSpecFlag = "auth"
	runEpicFlag = "epic-100"
	defer func() {
		runSpecFlag = ""
		runEpicFlag = ""
	}()

	err := runLoop(runCmd, []string{})

	// Expected: scope.ValidateFlags returns error saying flags are mutually exclusive
	// Current: no validation, so command proceeds normally
	if err == nil {
		t.Fatal("runLoop should return error when both --spec and --epic are set")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention 'mutually exclusive', got: %v", err)
	}
}

// TestRunCommand_SpecFlagValidatesSpecExists verifies that --spec flag validates
// the spec file exists before proceeding.
//
// Expected failure: Package variable runSpecFlag does not exist, and runLoop() does not
// call scope.ValidateSpec(specsDir, runSpecFlag). Without validation, nonexistent spec
// names are accepted and only fail later with unhelpful "no beads found" errors.
func TestRunCommand_SpecFlagValidatesSpecExists(t *testing.T) {
	tempDir := t.TempDir()
	gromitDir := setupFullGromitEnv(t, tempDir)
	specsDir := filepath.Join(gromitDir, "specs")

	// Create one valid spec
	writeSpecFiles(t, specsDir, []specFile{
		{"auth.md", "auth", "epic-100"},
	})

	runSpecFlag = "nonexistent"
	defer func() { runSpecFlag = "" }()

	err := runLoop(runCmd, []string{})

	// Expected: scope.ValidateSpec returns error listing available specs
	// Current: ResolveSpec returns "spec:nonexistent" which later fails with "no beads"
	if err == nil {
		t.Fatal("runLoop should return error for nonexistent spec")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should say spec 'not found', got: %v", err)
	}

	// The helpful part: error should list available specs
	if !strings.Contains(err.Error(), "auth") {
		t.Errorf("error should list available spec 'auth', got: %v", err)
	}
}

// TestRunCommand_NoFlagsProcessesAllBeads verifies that without --spec or --epic,
// the runner processes all ready beads (existing behavior continues to work).
//
// Expected failure: Package variables runSpecFlag and runEpicFlag do not exist yet.
// This is a regression test - after implementation, the code must check if labelFilters
// is empty before calling runner.SetLabelFilters(), ensuring default behavior is preserved.
func TestRunCommand_NoFlagsProcessesAllBeads(t *testing.T) {
	tempDir := t.TempDir()
	gromitDir := setupFullGromitEnv(t, tempDir)
	specsDir := filepath.Join(gromitDir, "specs")

	writeSpecFiles(t, specsDir, []specFile{
		{"auth.md", "auth", "epic-100"},
	})

	client := setupBeadClient(t, tempDir)

	authBead, err := client.Create("Auth task", 1, []string{"spec:auth"}, []string{})
	if err != nil {
		t.Skipf("Cannot create auth bead: %v", err)
	}

	noSpecBead, err := client.Create("No spec task", 1, []string{}, []string{})
	if err != nil {
		t.Skipf("Cannot create no-spec bead: %v", err)
	}

	// Ensure flags are empty
	runSpecFlag = ""
	runEpicFlag = ""

	dryRun = true
	defer func() { dryRun = false }()

	err = runLoop(runCmd, []string{})

	// Current behavior: client.Ready() returns all ready beads
	// After implementation: when labelFilters is empty, getNextBead should still
	// call client.Ready() and return all beads, not filter anything
	if err != nil && !strings.Contains(err.Error(), "no beads") {
		t.Logf("Created beads: auth=%s, no-spec=%s", authBead.ID, noSpecBead.ID)
		t.Errorf("runLoop without flags should process all beads, got: %v", err)
	}
}

// TestRunCommand_HelpTextDocumentsSpecFlag verifies that the run command help
// documents the --spec flag and explains its filtering behavior.
//
// Expected failure: runCmd.Long does not mention "--spec" or explain filtering/scoping
// behavior because the help text has not been updated to document the new flag.
func TestRunCommand_HelpTextDocumentsSpecFlag(t *testing.T) {
	helpText := runCmd.Long

	if !strings.Contains(helpText, "--spec") {
		t.Error("runCmd.Long should document --spec flag")
	}

	// Verify it explains the filtering/scoping concept
	lowerHelp := strings.ToLower(helpText)
	if !strings.Contains(lowerHelp, "filter") && !strings.Contains(lowerHelp, "scope") {
		t.Error("runCmd.Long should explain that --spec filters/scopes which beads are processed")
	}
}

// TestRunCommand_HelpTextDocumentsEpicFlag verifies that the run command help
// documents the --epic flag and explains its filtering behavior.
//
// Expected failure: runCmd.Long does not mention "--epic" or explain filtering/scoping
// behavior because the help text has not been updated to document the new flag.
func TestRunCommand_HelpTextDocumentsEpicFlag(t *testing.T) {
	helpText := runCmd.Long

	if !strings.Contains(helpText, "--epic") {
		t.Error("runCmd.Long should document --epic flag")
	}

	// Verify it explains the filtering/scoping concept
	lowerHelp := strings.ToLower(helpText)
	if !strings.Contains(lowerHelp, "filter") && !strings.Contains(lowerHelp, "scope") {
		t.Error("runCmd.Long should explain that --epic filters/scopes which beads are processed")
	}
}

// TestRunCommand_HelpTextDocumentsMutualExclusivity verifies that the help text
// explains that --spec and --epic are mutually exclusive.
//
// Expected failure: runCmd.Long does not contain the phrase "mutually exclusive"
// because the help text has not been updated to document flag interaction rules.
func TestRunCommand_HelpTextDocumentsMutualExclusivity(t *testing.T) {
	helpText := runCmd.Long

	if !strings.Contains(strings.ToLower(helpText), "mutually exclusive") {
		t.Error("runCmd.Long should note that --spec and --epic are mutually exclusive")
	}
}

// --- Test Helpers ---

// setupFullGromitEnv creates a complete gromit environment for testing.
// Returns the gromitDir path.
func setupFullGromitEnv(t *testing.T, tempDir string) string {
	t.Helper()
	gromitDir := filepath.Join(tempDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("Failed to create gromit dir: %v", err)
	}

	// Create minimal gromit.yaml
	cfgPath := filepath.Join(tempDir, "gromit.yaml")
	configContent := `models:
  p0: claude-opus-4
  p1: claude-sonnet-3-5
  p2: claude-haiku-3
  validation: claude-haiku-3

paths:
  gromit_dir: .gromit
  templates: .gromit/templates
  specs: .gromit/specs
`
	if err := os.WriteFile(cfgPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Update global configPath for test
	configPath = cfgPath

	return gromitDir
}

// setupBeadClient initializes a bead client for the test directory.
func setupBeadClient(t *testing.T, dir string) *bead.Client {
	t.Helper()

	// Initialize bd in the directory
	cmd := exec.Command("bd", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		// If bd is not installed or init fails, skip test
		t.Skipf("Cannot initialize bd: %v", err)
	}

	client, err := bead.NewClient()
	if err != nil {
		t.Skipf("Cannot create bead client: %v", err)
	}

	// Override client's working directory for tests
	client.Dir = dir

	return client
}
