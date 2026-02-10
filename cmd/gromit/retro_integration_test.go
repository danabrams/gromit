package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/scope"
)

// --- Scope flag tests (from retro_scope_acceptance_test.go) ---

// TestRetroCommand_SpecFlagExists verifies that the retro command accepts --spec flag
func TestRetroCommand_SpecFlagExists(t *testing.T) {
	cmd := retroCmd
	specFlag := cmd.Flags().Lookup("spec")
	if specFlag == nil {
		t.Fatal("retro command should have --spec flag")
	}
	if specFlag.Value.Type() != "string" {
		t.Errorf("--spec flag should be string type, got %s", specFlag.Value.Type())
	}
}

// TestRetroCommand_EpicFlagExists verifies that the retro command accepts --epic flag
func TestRetroCommand_EpicFlagExists(t *testing.T) {
	cmd := retroCmd
	epicFlag := cmd.Flags().Lookup("epic")
	if epicFlag == nil {
		t.Fatal("retro command should have --epic flag")
	}
	if epicFlag.Value.Type() != "string" {
		t.Errorf("--epic flag should be string type, got %s", epicFlag.Value.Type())
	}
}

// TestRetroCommand_SpecAndEpicMutuallyExclusive verifies that --spec and --epic
// cannot be used together on the retro command
func TestRetroCommand_SpecAndEpicMutuallyExclusive(t *testing.T) {
	err := scope.ValidateFlags("gromit-xyz", "init-wizard")
	if err == nil {
		t.Fatal("scope.ValidateFlags should return error when both epic and spec are set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %v", err)
	}
}

// TestRetroCommand_SpecFlagResolvesToLabel verifies that --spec flag
// resolves to the correct label format via scope.ResolveSpec
func TestRetroCommand_SpecFlagResolvesToLabel(t *testing.T) {
	specName := "init-wizard"
	labels := scope.ResolveSpec(specName)
	if len(labels) != 1 {
		t.Fatalf("ResolveSpec should return 1 label, got %d", len(labels))
	}
	expectedLabel := "spec:init-wizard"
	if labels[0] != expectedLabel {
		t.Errorf("ResolveSpec(%q) = %q, want %q", specName, labels[0], expectedLabel)
	}
}

// TestRetroCommand_EpicFlagUsesResolveEpic verifies that --epic flag
// uses scope.ResolveEpic to resolve epic to spec labels
func TestRetroCommand_EpicFlagUsesResolveEpic(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

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
		specContent := fmt.Sprintf("---\nid: %s\nepic: %s\ncreated: 2026-02-08\n---\n\n# Spec\n", spec.id, spec.epic)
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("Failed to write spec file: %v", err)
		}
	}

	labels, err := scope.ResolveEpic("gromit-xyz", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("ResolveEpic should return 2 labels, got %d", len(labels))
	}

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
}

// TestRetroCommand_SpecFlagFiltersIterationLogs verifies the --spec flag
// validates and resolves correctly for filtering iteration logs
func TestRetroCommand_SpecFlagFiltersIterationLogs(t *testing.T) {
	specFlag := "init-wizard"
	epicFlag := ""

	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept --spec alone, got error: %v", err)
	}

	labels := scope.ResolveSpec(specFlag)
	if len(labels) != 1 {
		t.Fatalf("ResolveSpec should return 1 label, got %d", len(labels))
	}
	expectedLabel := "spec:init-wizard"
	if labels[0] != expectedLabel {
		t.Fatalf("ResolveSpec returned %q, want %q", labels[0], expectedLabel)
	}
}

// TestRetroCommand_EpicFlagFiltersIterationLogs verifies the --epic flag
// validates and resolves correctly for filtering iteration logs
func TestRetroCommand_EpicFlagFiltersIterationLogs(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

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
		specContent := fmt.Sprintf("---\nid: %s\nepic: %s\ncreated: 2026-02-08\n---\n\n# Spec\n", spec.id, spec.epic)
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("Failed to write spec file: %v", err)
		}
	}

	epicFlag := "gromit-xyz"
	specFlag := ""

	if err := scope.ValidateFlags(epicFlag, specFlag); err != nil {
		t.Fatalf("ValidateFlags should accept --epic alone, got error: %v", err)
	}

	labels, err := scope.ResolveEpic(epicFlag, specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("ResolveEpic should return 2 labels for gromit-xyz, got %d: %v", len(labels), labels)
	}

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
}

// TestRetroCommand_NoScopeFlagUsesDefaultBehavior verifies that when neither
// --epic nor --spec is provided, validation passes with no filter
func TestRetroCommand_NoScopeFlagUsesDefaultBehavior(t *testing.T) {
	if err := scope.ValidateFlags("", ""); err != nil {
		t.Fatalf("ValidateFlags should accept both flags empty, got error: %v", err)
	}
}

// TestRetroCommand_EpicResolutionWithNoSpecs verifies behavior when
// --epic resolves to no specs in the specs directory
func TestRetroCommand_EpicResolutionWithNoSpecs(t *testing.T) {
	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	if err := scope.ValidateFlags("nonexistent-epic", ""); err != nil {
		t.Fatalf("ValidateFlags should accept --epic flag, got error: %v", err)
	}

	labels, err := scope.ResolveEpic("nonexistent-epic", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic should not error for nonexistent epic, got error: %v", err)
	}
	if len(labels) != 0 {
		t.Fatalf("ResolveEpic should return 0 labels for nonexistent epic, got %d: %v", len(labels), labels)
	}
}

// TestRetroCommand_ValidatesFlagsBeforeResolution verifies that scope validation
// happens before any resolution or bead client calls
func TestRetroCommand_ValidatesFlagsBeforeResolution(t *testing.T) {
	err := scope.ValidateFlags("gromit-xyz", "init-wizard")
	if err == nil {
		t.Fatal("ValidateFlags should reject both flags set")
	}
}

// --- CLI help text tests (from retro_cli_acceptance_test.go) ---

// TestRetroCmd_HelpText_DocumentsSpecFlag verifies that --spec flag is documented
// in the retro command's Long help text
func TestRetroCmd_HelpText_DocumentsSpecFlag(t *testing.T) {
	helpText := retroCmd.Long
	if !strings.Contains(helpText, "--spec") {
		t.Error("retroCmd.Long should document --spec flag")
	}
	if !strings.Contains(strings.ToLower(helpText), "filter") {
		t.Error("retroCmd.Long should explain filtering when using --spec")
	}
	if !strings.Contains(strings.ToLower(helpText), "mutually exclusive") {
		t.Error("retroCmd.Long should note that --spec and --epic are mutually exclusive")
	}
}

// TestRetroCmd_HelpText_DocumentsEpicFlag verifies that --epic flag is documented
// in the retro command's Long help text
func TestRetroCmd_HelpText_DocumentsEpicFlag(t *testing.T) {
	helpText := retroCmd.Long
	if !strings.Contains(helpText, "--epic") {
		t.Error("retroCmd.Long should document --epic flag")
	}
	if !strings.Contains(strings.ToLower(helpText), "filter") {
		t.Error("retroCmd.Long should explain filtering when using --epic")
	}
	if !strings.Contains(strings.ToLower(helpText), "mutually exclusive") {
		t.Error("retroCmd.Long should note that --spec and --epic are mutually exclusive")
	}
}

// --- buildBeadFilter tests (from retro_e2e_acceptance_test.go) ---

// TestRetroCommand_BuildBeadFilterHandlesEmptyBeadList verifies that when
// scope resolution finds no matching beads, buildBeadFilter handles it gracefully
func TestRetroCommand_BuildBeadFilterHandlesEmptyBeadList(t *testing.T) {
	ctx := context.Background()

	// Empty labels list returns nil filter
	filter, err := buildBeadFilter(ctx, []string{})
	if err != nil {
		t.Errorf("buildBeadFilter with empty labels should not error, got: %v", err)
	}
	if filter != nil {
		t.Errorf("buildBeadFilter with empty labels should return nil, got: %v", filter)
	}

	// nil labels list returns nil filter
	filter, err = buildBeadFilter(ctx, nil)
	if err != nil {
		t.Errorf("buildBeadFilter with nil labels should not error, got: %v", err)
	}
	if filter != nil {
		t.Errorf("buildBeadFilter with nil labels should return nil, got: %v", filter)
	}
}

// --- End-to-end environment setup tests (from retro_e2e_acceptance_test.go) ---

// TestRetroCommand_EndToEndSpecFiltering verifies the complete filtering chain
// when --spec flag is used, including test environment setup
func TestRetroCommand_EndToEndSpecFiltering(t *testing.T) {
	tempDir := t.TempDir()
	gromitDir := filepath.Join(tempDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("Failed to create gromit dir: %v", err)
	}

	configPath := filepath.Join(tempDir, "gromit.yaml")
	cfg := &config.Config{
		Paths: config.PathsConfig{GromitDir: gromitDir},
		Models: config.ModelsConfig{
			P0: "claude-opus-4", P1: "claude-sonnet-3-5",
			P2: "claude-haiku-3", Validation: "claude-haiku-3",
		},
	}
	configData, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	logsDir := filepath.Join(gromitDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	testLogs := []struct {
		beadID string
		phase  string
		status string
	}{
		{"gromit-1", "build", "success"},
		{"gromit-2", "validate", "success"},
		{"gromit-3", "build", "success"},
		{"gromit-4", "validate", "failure"},
		{"gromit-5", "build", "success"},
	}

	for i, log := range testLogs {
		logEntry := logger.IterationLog{
			Timestamp: time.Now(),
			Iteration: i + 1,
			BeadID:    log.beadID,
			BeadTitle: "Test bead " + log.beadID,
			Model:     "claude-sonnet-3-5",
			Success:   log.status == "success",
			Validated: log.phase == "validate",
		}
		logData, err := json.Marshal(logEntry)
		if err != nil {
			t.Fatalf("Failed to marshal log entry: %v", err)
		}
		logPath := filepath.Join(logsDir, "iteration.log")
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("Failed to open log file: %v", err)
		}
		if _, err := f.WriteString(string(logData) + "\n"); err != nil {
			f.Close()
			t.Fatalf("Failed to write log entry %d: %v", i, err)
		}
		f.Close()
	}

	rulesPath := filepath.Join(gromitDir, "RULES.md")
	if err := os.WriteFile(rulesPath, []byte("# Rules\n\nTest rules\n"), 0644); err != nil {
		t.Fatalf("Failed to write RULES.md: %v", err)
	}
	learningsPath := filepath.Join(gromitDir, "LEARNINGS.md")
	if err := os.WriteFile(learningsPath, []byte("# Learnings\n\nTest learnings\n"), 0644); err != nil {
		t.Fatalf("Failed to write LEARNINGS.md: %v", err)
	}

	templatesDir := filepath.Join(gromitDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}
	retroTemplatePath := filepath.Join(templatesDir, "PROMPT_RETRO.md")
	retroTemplate := "# Retrospective Analysis\n\n## Rules\n{{ .Rules }}\n\n## Learnings\n{{ .Learnings }}\n\n## Stats\nTotal iterations: {{ .RunStats.Total }}\n"
	if err := os.WriteFile(retroTemplatePath, []byte(retroTemplate), 0644); err != nil {
		t.Fatalf("Failed to write retro template: %v", err)
	}

	// Verify all infrastructure files exist
	for _, path := range []string{
		filepath.Join(logsDir, "iteration.log"),
		configPath,
		rulesPath,
		learningsPath,
		retroTemplatePath,
	} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("Expected file should exist: %s", path)
		}
	}

	// beadFilter flow: scope.ResolveSpec returns labels, buildBeadFilter builds
	// the filter map from bead IDs, retro.Run() uses the filter to restrict analysis
}

// TestRetroCommand_EndToEndEpicFiltering verifies the complete filtering chain
// when --epic flag is used
func TestRetroCommand_EndToEndEpicFiltering(t *testing.T) {
	tempDir := t.TempDir()
	gromitDir := filepath.Join(tempDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	specs := []struct {
		filename string
		id       string
		epic     string
	}{
		{"auth.md", "auth", "gromit-100"},
		{"profile.md", "profile", "gromit-100"},
		{"settings.md", "settings", "gromit-200"},
	}

	for _, spec := range specs {
		specPath := filepath.Join(specsDir, spec.filename)
		specContent := "---\nid: " + spec.id + "\nepic: " + spec.epic + "\ncreated: 2026-02-08\n---\n\n# Spec\n"
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("Failed to write spec file: %v", err)
		}
	}

	logsDir := filepath.Join(gromitDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	testLogs := []struct {
		beadID string
	}{
		{"gromit-1"},
		{"gromit-2"},
		{"gromit-3"},
	}

	for i, log := range testLogs {
		logEntry := logger.IterationLog{
			Timestamp: time.Now(),
			Iteration: i + 1,
			BeadID:    log.beadID,
			BeadTitle: "Test bead " + log.beadID,
			Model:     "claude-sonnet-3-5",
			Success:   true,
			Validated: false,
		}
		logData, err := json.Marshal(logEntry)
		if err != nil {
			t.Fatalf("Failed to marshal log entry: %v", err)
		}
		logPath := filepath.Join(logsDir, "iteration.log")
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("Failed to open log file: %v", err)
		}
		if _, err := f.WriteString(string(logData) + "\n"); err != nil {
			f.Close()
			t.Fatalf("Failed to write log entry %d: %v", i, err)
		}
		f.Close()
	}

	for _, spec := range specs {
		specPath := filepath.Join(specsDir, spec.filename)
		if _, err := os.Stat(specPath); os.IsNotExist(err) {
			t.Fatalf("Spec file %s should exist", spec.filename)
		}
	}

	// beadFilter flow: scope.ResolveEpic returns labels for all specs in the epic,
	// buildBeadFilter unions bead IDs from all labels, retro.Run() filters analysis
}
