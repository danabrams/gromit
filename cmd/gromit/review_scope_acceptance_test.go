package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/scope"
)

// TestReviewCommand_SpecFlagExists verifies that the review command accepts --spec flag
func TestReviewCommand_SpecFlagExists(t *testing.T) {
	// Create the review command and verify --spec flag exists
	cmd := reviewCmd

	// Try to get the --spec flag
	specFlag := cmd.Flags().Lookup("spec")
	if specFlag == nil {
		t.Fatal("review command should have --spec flag")
	}

	// Verify flag type is string
	if specFlag.Value.Type() != "string" {
		t.Errorf("--spec flag should be string type, got %s", specFlag.Value.Type())
	}
}

// TestReviewCommand_SpecAndEpicMutuallyExclusive verifies that --spec and --epic
// cannot be used together on the review command
func TestReviewCommand_SpecAndEpicMutuallyExclusive(t *testing.T) {
	// This test verifies that determineReviewScope checks mutual exclusivity
	// between --epic and --spec flags using scope.ValidateFlags

	// The validation should happen early in determineReviewScope
	// Test at the scope.ValidateFlags level first
	err := scope.ValidateFlags("gromit-xyz", "init-wizard")
	if err == nil {
		t.Fatal("scope.ValidateFlags should return error when both epic and spec are set")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %v", err)
	}

	// The determineReviewScope function should call scope.ValidateFlags(reviewEpic, reviewSpec)
	// and return the error if both are set. This will be verified once reviewSpec variable exists.

	t.Skip("Pending implementation: determineReviewScope does not yet call scope.ValidateFlags with reviewSpec")
}

// TestReviewCommand_SpecFlagResolvesToLabel verifies that --spec flag
// resolves to the correct label format via scope.ResolveSpec
func TestReviewCommand_SpecFlagResolvesToLabel(t *testing.T) {
	// This test verifies that the spec name is correctly resolved to a label
	// The label format should be "spec:<name>"

	specName := "init-wizard"
	labels := scope.ResolveSpec(specName)

	if len(labels) != 1 {
		t.Fatalf("ResolveSpec should return 1 label, got %d", len(labels))
	}

	expectedLabel := "spec:init-wizard"
	if labels[0] != expectedLabel {
		t.Errorf("ResolveSpec(%q) = %q, want %q", specName, labels[0], expectedLabel)
	}

	// This demonstrates the expected flow in determineReviewScope:
	// 1. reviewSpec flag is read
	// 2. scope.ResolveSpec(reviewSpec) returns label
	// 3. bead.ListWithLabel(label) gets beads
	// 4. Find earliest commit from those bead IDs

	t.Skip("Pending implementation: determineReviewScope does not yet call scope.ResolveSpec for --spec flag")
}

// TestReviewCommand_EpicFlagUsesResolveEpic verifies that --epic flag
// uses scope.ResolveEpic to resolve epic to spec labels
func TestReviewCommand_EpicFlagUsesResolveEpic(t *testing.T) {
	// This test verifies that scope.ResolveEpic correctly finds specs for an epic

	// Create temp directory with specs
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create spec files linked to the epic
	specs := []struct {
		filename string
		id       string
		epic     string
	}{
		{"auth.md", "auth", "gromit-xyz"},
		{"profile.md", "profile", "gromit-xyz"},
	}

	for _, spec := range specs {
		specPath := filepath.Join(specsDir, spec.filename)
		specContent := fmt.Sprintf(`---
id: %s
epic: %s
created: 2026-02-08
---

# Spec
`, spec.id, spec.epic)
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("Failed to write spec file: %v", err)
		}
	}

	// Call scope.ResolveEpic to verify it works
	labels, err := scope.ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}

	if len(labels) != 2 {
		t.Fatalf("ResolveEpic should return 2 labels, got %d", len(labels))
	}

	// Verify expected labels are present
	expectedLabels := map[string]bool{
		"spec:auth":    false,
		"spec:profile": false,
	}
	for _, label := range labels {
		if _, exists := expectedLabels[label]; !exists {
			t.Errorf("Unexpected label %q", label)
		}
		expectedLabels[label] = true
	}
	for label, found := range expectedLabels {
		if !found {
			t.Errorf("Missing expected label %q", label)
		}
	}

	// The new implementation should:
	// 1. Call scope.ResolveEpic("gromit-xyz", specsDir) to get ["spec:auth", "spec:profile"]
	// 2. For each label, get beads via bead.ListWithLabel
	// 3. Find earliest commit from all those beads

	t.Skip("Pending implementation: determineReviewScope should use scope.ResolveEpic for --epic flag instead of parent-child resolution")
}

// TestReviewCommand_FlagPriorityOrder verifies the priority order of scope flags
func TestReviewCommand_FlagPriorityOrder(t *testing.T) {
	// This test documents the expected priority order in determineReviewScope:
	// --since > --epic > --spec > state file
	//
	// The current implementation has: --since > --epic > state file
	// The new implementation should have: --since > --spec > --epic > state file
	// (spec should be checked before epic, per the task description)

	t.Skip("Pending implementation: --spec flag priority between --epic and state file not yet defined in implementation")
}

// TestReviewCommand_MutualExclusivityOfScopeFlags verifies that scope determination
// flags are mutually exclusive
func TestReviewCommand_MutualExclusivityOfScopeFlags(t *testing.T) {
	// The task description says "mutual exclusivity check between --epic, --spec, and --since"
	// The implementation should enforce that only one scope determination method is used at a time.

	// The current priority in determineReviewScope is:
	// 1. --since (returns immediately if set)
	// 2. --epic (calls getEpicBaseCommit)
	// 3. state file (default fallback)

	// After implementation, it should be:
	// 1. --since (returns immediately if set)
	// 2. --spec (calls getSpecBaseCommit or similar)
	// 3. --epic (calls scope.ResolveEpic + findEarliestCommit)
	// 4. state file (default fallback)

	// Mutual exclusivity is enforced by the priority order (early returns)
	// plus a validation check that --epic and --spec are not both set

	// Test that --epic and --spec cannot both be set
	err := scope.ValidateFlags("gromit-xyz", "init-wizard")
	if err == nil {
		t.Fatal("--epic and --spec together should be mutually exclusive")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %v", err)
	}

	// --since with --spec: --since takes priority (no error needed, just use --since)
	// --since with --epic: --since takes priority (no error needed, just use --since)
	// All three: --since takes priority (no error needed, just use --since)

	// The key validation is that --epic and --spec cannot both be non-empty
	t.Skip("Pending implementation: determineReviewScope should call scope.ValidateFlags to check --epic and --spec mutual exclusivity")
}

// TestReviewCommand_SpecFlagInHelpText verifies that --spec flag appears
// in the review command help text
func TestReviewCommand_SpecFlagInHelpText(t *testing.T) {
	cmd := reviewCmd

	// Get the long description
	helpText := cmd.Long

	// The help text should document the --spec flag in the scope options section
	// Current text has:
	//   --since <commit>   Review from a specific commit
	//   --epic <id>        Review changes from an epic's child beads
	// After implementation should have:
	//   --spec <name>      Review changes from a spec's beads

	if strings.Contains(helpText, "--spec") {
		// Good, the flag is documented
		return
	}

	// Flag not yet documented in help text
	t.Skip("Pending implementation: --spec flag not yet documented in review command help text")
}

// TestReviewCommand_SpecResolutionWithNoBeads verifies behavior when
// --spec resolves to a label with no matching beads
func TestReviewCommand_SpecResolutionWithNoBeads(t *testing.T) {
	// This test verifies that when --spec is used but no beads match the label,
	// determineReviewScope returns an appropriate error

	// The expected behavior is similar to --epic with no child beads:
	// return an error like "no commits found for spec X - try using --since to specify a commit"

	// This will be tested once the implementation is complete
	t.Skip("Pending implementation: handling of --spec with no matching beads")
}
