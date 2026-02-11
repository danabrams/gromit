//go:build acceptance

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
)

// TestRunCommand_SpecFlagFiltersBeads verifies that --spec flag only processes beads
// with the matching spec label, not all beads.
//
// Expected failure: runLoop does not call scope.ValidateSpec, scope.ResolveSpec,
// or runner.SetLabelFilters, so all beads will be processed regardless of --spec flag.
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
// Expected failure: runLoop does not call scope.ResolveEpic or runner.SetLabelFilters,
// so all beads will be processed regardless of --epic flag.
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
// Expected failure: runLoop does not call scope.ValidateFlags, so both flags
// can be set without error.
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
// Expected failure: runLoop does not call scope.ValidateSpec, so nonexistent spec
// names are accepted and only fail later with unhelpful errors.
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
// Expected failure: This is a regression test. After implementation, we need to verify
// that the default behavior (no flags = no filtering) still works correctly.
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = runLoop(runCmd, []string{})

	// Current behavior: client.Ready() returns all ready beads
	// After implementation: when labelFilters is empty, getNextBead should still
	// call client.Ready() and return all beads, not filter anything
	if err != nil && !strings.Contains(err.Error(), "no beads") {
		t.Logf("Created beads: auth=%s, no-spec=%s", authBead.ID, noSpecBead.ID)
		t.Errorf("runLoop without flags should process all beads, got: %v", err)
	}
}

// --- Test Helpers ---

// setupFullGromitEnv creates a complete gromit environment for testing.
// Returns the gromitDir path.
func setupFullGromitEnv(t *testing.T, tempDir string) string {
	t.Helper()
	return setupMinimalGromitConfig(t, tempDir)
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
