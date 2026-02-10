package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/scope"
)

// TestRetroCommand_SpecFlagExists verifies that the retro command accepts --spec flag
func TestRetroCommand_SpecFlagExists(t *testing.T) {
	// Create the retro command and verify --spec flag exists
	cmd := retroCmd

	// Try to get the --spec flag
	specFlag := cmd.Flags().Lookup("spec")
	if specFlag == nil {
		t.Fatal("retro command should have --spec flag")
	}

	// Verify flag type is string
	if specFlag.Value.Type() != "string" {
		t.Errorf("--spec flag should be string type, got %s", specFlag.Value.Type())
	}
}

// TestRetroCommand_EpicFlagExists verifies that the retro command accepts --epic flag
func TestRetroCommand_EpicFlagExists(t *testing.T) {
	// Create the retro command and verify --epic flag exists
	cmd := retroCmd

	// Try to get the --epic flag
	epicFlag := cmd.Flags().Lookup("epic")
	if epicFlag == nil {
		t.Fatal("retro command should have --epic flag")
	}

	// Verify flag type is string
	if epicFlag.Value.Type() != "string" {
		t.Errorf("--epic flag should be string type, got %s", epicFlag.Value.Type())
	}
}

// TestRetroCommand_SpecAndEpicMutuallyExclusive verifies that --spec and --epic
// cannot be used together on the retro command
func TestRetroCommand_SpecAndEpicMutuallyExclusive(t *testing.T) {
	// This test verifies that retro command checks mutual exclusivity
	// between --epic and --spec flags using scope.ValidateFlags

	// The validation should happen early in the retro command execution
	// Test at the scope.ValidateFlags level first
	err := scope.ValidateFlags("gromit-xyz", "init-wizard")
	if err == nil {
		t.Fatal("scope.ValidateFlags should return error when both epic and spec are set")
	}

	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %v", err)
	}

	// The retro command should call scope.ValidateFlags(epic, spec)
	// and return the error if both are set.
}

// TestRetroCommand_SpecFlagResolvesToLabel verifies that --spec flag
// resolves to the correct label format via scope.ResolveSpec
func TestRetroCommand_SpecFlagResolvesToLabel(t *testing.T) {
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

	// This demonstrates the expected flow in retro command:
	// 1. spec flag is read
	// 2. scope.ResolveSpec(spec) returns label
	// 3. Pass label to retro logic as filter parameter
}

// TestRetroCommand_EpicFlagUsesResolveEpic verifies that --epic flag
// uses scope.ResolveEpic to resolve epic to spec labels
func TestRetroCommand_EpicFlagUsesResolveEpic(t *testing.T) {
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
	// 3. Filter iteration logs by those bead IDs
}

// TestRetroCommand_SpecFlagFiltersIterationLogs verifies that --spec flag
// filters iteration logs to only include beads with the matching spec label
func TestRetroCommand_SpecFlagFiltersIterationLogs(t *testing.T) {
	// This test verifies the end-to-end flow of --spec flag for retro command:
	// 1. Parse --spec flag from command line
	// 2. Call scope.ResolveSpec to get label
	// 3. Call bead.ListWithLabel to get bead IDs
	// 4. Filter iteration logs to only include those bead IDs

	// Set up flag values as they would be from CLI
	specFlag := "init-wizard"
	epicFlag := ""

	// Verify scope.ValidateFlags accepts this combination
	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept --spec alone, got error: %v", err)
	}

	// Verify scope.ResolveSpec returns correct label
	labels := scope.ResolveSpec(specFlag)
	if len(labels) != 1 {
		t.Fatalf("ResolveSpec should return 1 label, got %d", len(labels))
	}
	expectedLabel := "spec:init-wizard"
	if labels[0] != expectedLabel {
		t.Fatalf("ResolveSpec returned %q, want %q", labels[0], expectedLabel)
	}

	// TODO: This test will fully pass when retro command accepts --spec flag
	// and passes the resolved bead IDs to retro.Run() as a filter parameter
	t.Skip("Pending implementation: retro command does not yet accept --spec flag")
}

// TestRetroCommand_EpicFlagFiltersIterationLogs verifies that --epic flag
// filters iteration logs to only include beads from specs in that epic
func TestRetroCommand_EpicFlagFiltersIterationLogs(t *testing.T) {
	// This test verifies the end-to-end flow of --epic flag for retro command:
	// 1. Parse --epic flag from command line
	// 2. Call scope.ResolveEpic to get labels for all specs in that epic
	// 3. Call bead.ListWithLabel for each label to get bead IDs
	// 4. Filter iteration logs to only include those bead IDs

	// Create temp directory with spec files
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
		{"settings.md", "settings", "other-epic"},
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

	// Set up flag values
	epicFlag := "gromit-xyz"
	specFlag := ""

	// Verify scope.ValidateFlags accepts this combination
	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept --epic alone, got error: %v", err)
	}

	// Verify scope.ResolveEpic returns correct labels
	labels, err := scope.ResolveEpic(epicFlag, specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("ResolveEpic should return 2 labels for gromit-xyz, got %d: %v", len(labels), labels)
	}

	// Verify labels are correct
	expectedLabels := map[string]bool{"spec:auth": false, "spec:profile": false}
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

	// TODO: This test will fully pass when retro command accepts --epic flag
	// and passes the resolved bead IDs to retro.Run() as a filter parameter
	t.Skip("Pending implementation: retro command does not yet accept --epic flag")
}

// TestRetroCommand_NoScopeFlagUsesDefaultBehavior verifies that when neither
// --epic nor --spec is provided, all iteration logs are included (default behavior)
func TestRetroCommand_NoScopeFlagUsesDefaultBehavior(t *testing.T) {
	// This test verifies that when no scope flags are set, validation passes
	// and no bead ID filter is passed to retro logic (default behavior)

	epicFlag := ""
	specFlag := ""

	// Verify scope.ValidateFlags accepts empty flags
	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept both flags empty, got error: %v", err)
	}

	// When both flags are empty, no filter should be passed to retro.Run()
	// The retro logic should process all iteration logs

	// TODO: This test will fully pass when we verify retro command behavior with no flags
	t.Skip("Pending implementation: need to verify retro command preserves default behavior when flags are empty")
}

// TestRetroCommand_SpecFlagInHelpText verifies that --spec flag appears
// in the retro command help text
func TestRetroCommand_SpecFlagInHelpText(t *testing.T) {
	cmd := retroCmd

	// Get the long description
	helpText := cmd.Long

	// The help text should document the --spec flag
	// Current text documents various features
	// After implementation should have:
	//   --spec <name>      Filter retro to a spec's beads

	if !strings.Contains(helpText, "--spec") {
		t.Fatal("--spec flag should be documented in retro command help text")
	}
}

// TestRetroCommand_EpicFlagInHelpText verifies that --epic flag appears
// in the retro command help text
func TestRetroCommand_EpicFlagInHelpText(t *testing.T) {
	cmd := retroCmd

	// Get the long description
	helpText := cmd.Long

	// The help text should document the --epic flag
	// After implementation should have:
	//   --epic <id>        Filter retro to an epic's beads

	if !strings.Contains(helpText, "--epic") {
		t.Fatal("--epic flag should be documented in retro command help text")
	}
}

// TestRetroCommand_SpecResolutionWithNoBeads verifies behavior when
// --spec resolves to a label with no matching beads
func TestRetroCommand_SpecResolutionWithNoBeads(t *testing.T) {
	// This test verifies that when --spec is used but no beads match the label,
	// the retro command handles it gracefully (no error, empty analysis or warning)

	// The expected behavior is similar to run/review with no beads:
	// Process completes but notes that no beads were found for the spec

	// This will be tested once the implementation is complete
	t.Skip("Pending implementation: handling of --spec with no matching beads")
}

// TestRetroCommand_EpicResolutionWithNoSpecs verifies behavior when
// --epic resolves to no specs in the specs directory
func TestRetroCommand_EpicResolutionWithNoSpecs(t *testing.T) {
	// This test verifies that when --epic is used but no specs match,
	// the retro command handles it gracefully

	// Create temp directory with no matching specs
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	epicFlag := "nonexistent-epic"
	specFlag := ""

	// Verify scope.ValidateFlags accepts the flag
	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept --epic flag, got error: %v", err)
	}

	// Verify scope.ResolveEpic returns empty labels (no specs match)
	labels, err := scope.ResolveEpic(epicFlag, specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic should not error for nonexistent epic, got error: %v", err)
	}
	if len(labels) != 0 {
		t.Fatalf("ResolveEpic should return 0 labels for nonexistent epic, got %d: %v", len(labels), labels)
	}

	// Expected behavior:
	// - When labels slice is empty, retro should complete with no beads found
	// - Should not error, just complete with empty analysis

	t.Skip("Pending implementation: handling of --epic with no matching specs")
}

// TestRetroCommand_SpecFlagPassedToRetroLogic verifies that the --spec flag
// value is correctly passed to the retro logic for filtering
func TestRetroCommand_SpecFlagPassedToRetroLogic(t *testing.T) {
	// This test verifies the full integration chain from CLI flag to retro logic

	// The expected flow is:
	// 1. retro command parses --spec flag
	// 2. Calls scope.ValidateFlags(epic, spec)
	// 3. Calls scope.ResolveSpec(spec) to get labels
	// 4. Calls bead.ListWithLabel(label) to get bead IDs
	// 5. Passes bead ID set to retro.Run() as filter parameter
	// 6. retro.Run() filters iteration logs by bead ID set before analysis

	specFlag := "init-wizard"
	epicFlag := ""

	// Verify validation passes
	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept --spec flag, got error: %v", err)
	}

	// Verify label resolution works
	labels := scope.ResolveSpec(specFlag)
	if len(labels) != 1 {
		t.Fatalf("ResolveSpec should return 1 label, got %d", len(labels))
	}

	// TODO: This test will fully pass when retro command calls bead.ListWithLabel
	// and passes the bead ID set to retro.Run()
	t.Skip("Pending implementation: retro command does not yet pass bead ID filter to retro.Run()")
}

// TestRetroCommand_EpicFlagPassedToRetroLogic verifies that the --epic flag
// value is correctly passed to the retro logic for filtering
func TestRetroCommand_EpicFlagPassedToRetroLogic(t *testing.T) {
	// Create temp directory with spec files
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create spec file linked to epic
	specPath := filepath.Join(specsDir, "auth.md")
	specContent := `---
id: auth
epic: gromit-xyz
created: 2026-02-08
---

# Auth Spec
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	epicFlag := "gromit-xyz"
	specFlag := ""

	// Verify validation passes
	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept --epic flag, got error: %v", err)
	}

	// Verify label resolution works
	labels, err := scope.ResolveEpic(epicFlag, specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	if len(labels) != 1 {
		t.Fatalf("ResolveEpic should return 1 label, got %d", len(labels))
	}

	// The expected flow is:
	// 1. retro command parses --epic flag
	// 2. Calls scope.ValidateFlags(epic, spec)
	// 3. Calls scope.ResolveEpic(epic, specsDir) to get labels
	// 4. For each label, calls bead.ListWithLabel(label) to get bead IDs
	// 5. Unions all bead IDs into a set
	// 6. Passes bead ID set to retro.Run() as filter parameter
	// 7. retro.Run() filters iteration logs by bead ID set before analysis

	// TODO: This test will fully pass when retro command resolves labels to bead IDs
	// and passes them to retro.Run()
	t.Skip("Pending implementation: retro command does not yet pass bead ID filter to retro.Run()")
}

// TestRetroCommand_ScopeValidationCalledBeforeRetroRun verifies that
// scope.ValidateFlags is called before retro.Run() to enforce mutual exclusivity
func TestRetroCommand_ScopeValidationCalledBeforeRetroRun(t *testing.T) {
	// This test verifies that validation happens before retro logic execution

	epicFlag := "gromit-xyz"
	specFlag := "init-wizard"

	// Verify scope.ValidateFlags rejects this combination
	err := scope.ValidateFlags(epicFlag, specFlag)
	if err == nil {
		t.Fatal("ValidateFlags should return error when both --epic and --spec are provided")
	}

	// Verify error message is clear
	errMsg := err.Error()
	if !strings.Contains(strings.ToLower(errMsg), "mutually exclusive") {
		t.Errorf("Error message should mention 'mutually exclusive', got: %q", errMsg)
	}

	// The retro command should call scope.ValidateFlags(epic, spec) early
	// and return the error before calling retro.Run()
	t.Skip("Pending implementation: retro command does not yet call scope.ValidateFlags")
}

// TestRetroCommand_FilteredLogsExcludeOtherSpecBeads verifies that when
// --spec is used, iteration logs from other specs are excluded
func TestRetroCommand_FilteredLogsExcludeOtherSpecBeads(t *testing.T) {
	// This test verifies that the filtering is exclusive:
	// only beads matching the specified spec label are included,
	// all other beads are excluded

	// The expected behavior:
	// 1. scope.ResolveSpec("init-wizard") returns ["spec:init-wizard"]
	// 2. bead.ListWithLabel("spec:init-wizard") returns beads [bead1, bead2]
	// 3. retro.Run() receives beadIDSet = {bead1, bead2}
	// 4. When processing iteration logs, only logs with bead_id in {bead1, bead2} are included
	// 5. Logs with other bead IDs (from other specs) are excluded

	t.Skip("Pending implementation: retro command filtering logic not yet implemented")
}

// TestRetroCommand_FilteredLogsExcludeOtherEpicBeads verifies that when
// --epic is used, iteration logs from other epics are excluded
func TestRetroCommand_FilteredLogsExcludeOtherEpicBeads(t *testing.T) {
	// This test verifies that the filtering is exclusive:
	// only beads from specs in the specified epic are included,
	// all other beads are excluded

	// The expected behavior:
	// 1. scope.ResolveEpic("gromit-xyz", specsDir) returns ["spec:auth", "spec:profile"]
	// 2. For each label, bead.ListWithLabel() returns bead IDs
	// 3. Union of all bead IDs: {bead1, bead2, bead3, bead4}
	// 4. retro.Run() receives beadIDSet = {bead1, bead2, bead3, bead4}
	// 5. When processing iteration logs, only logs with bead_id in the set are included
	// 6. Logs from beads in other epics are excluded

	t.Skip("Pending implementation: retro command filtering logic not yet implemented")
}

// TestRetroCommand_BeadStatsFilteredByScope verifies that the BeadStats map
// passed to the retro prompt is filtered by the scope (epic or spec)
func TestRetroCommand_BeadStatsFilteredByScope(t *testing.T) {
	// This test verifies that when --spec or --epic is used,
	// the BeadStats map in the retro analysis only includes beads
	// from the specified scope

	// The expected behavior:
	// 1. Iteration logs are filtered by bead ID set
	// 2. Per-bead stats are computed from filtered logs
	// 3. BeadStats map only includes stats for beads in the scope
	// 4. Retro prompt template receives filtered BeadStats map
	// 5. Analysis focuses only on beads within the specified scope

	t.Skip("Pending implementation: retro filtering of BeadStats not yet implemented")
}

// TestRetroCommand_RunStatsReflectFilteredScope verifies that the RunStats
// aggregates in the retro analysis reflect only the filtered scope
func TestRetroCommand_RunStatsReflectFilteredScope(t *testing.T) {
	// This test verifies that when --spec or --epic is used,
	// the RunStats aggregates (total iterations, success/failure counts)
	// are computed from the filtered iteration logs only

	// The expected behavior:
	// 1. Iteration logs are filtered by bead ID set
	// 2. RunStats are computed from filtered logs
	// 3. Total/Succeeded/Failed counts reflect only filtered scope
	// 4. Retro analysis provides accurate stats for the specified scope

	t.Skip("Pending implementation: retro filtering of RunStats not yet implemented")
}

// TestRetroCommand_EfficiencyReportFilteredByScope verifies that the
// EfficiencyReport in the retro analysis reflects only the filtered scope
func TestRetroCommand_EfficiencyReportFilteredByScope(t *testing.T) {
	// This test verifies that when --spec or --epic is used,
	// the EfficiencyReport (cost per bead, per-model stats) is computed
	// from the filtered iteration logs only

	// The expected behavior:
	// 1. Iteration logs are filtered by bead ID set
	// 2. EfficiencyReport is computed from filtered logs
	// 3. Cost metrics reflect only the specified scope
	// 4. Per-model aggregates include only iterations from the scope

	t.Skip("Pending implementation: retro filtering of EfficiencyReport not yet implemented")
}
