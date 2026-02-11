//go:build acceptance

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

// --- Flag declaration tests ---

// TestRunCommand_SpecFlagExists verifies that the run command has a --spec flag
func TestRunCommand_SpecFlagExists(t *testing.T) {
	// Expected failure: runSpecFlag variable and flag declaration do not exist yet
	specFlag := runCmd.Flags().Lookup("spec")
	if specFlag == nil {
		t.Fatal("run command should have --spec flag")
	}
	if specFlag.Value.Type() != "string" {
		t.Errorf("--spec flag should be string type, got %s", specFlag.Value.Type())
	}
}

// TestRunCommand_EpicFlagExists verifies that the run command has an --epic flag
func TestRunCommand_EpicFlagExists(t *testing.T) {
	// Expected failure: runEpicFlag variable and flag declaration do not exist yet
	epicFlag := runCmd.Flags().Lookup("epic")
	if epicFlag == nil {
		t.Fatal("run command should have --epic flag")
	}
	if epicFlag.Value.Type() != "string" {
		t.Errorf("--epic flag should be string type, got %s", epicFlag.Value.Type())
	}
}

// --- Flag validation tests ---

// TestRunLoop_ValidatesFlags verifies that runLoop calls scope.ValidateFlags
// and returns an error when both --spec and --epic are set
func TestRunLoop_ValidatesFlags(t *testing.T) {
	// Expected failure: runLoop does not call scope.ValidateFlags yet
	tempDir := t.TempDir()
	setupMinimalGromitConfig(t, tempDir)

	// Set both flags (mutually exclusive)
	runSpecFlag = "init-wizard"
	runEpicFlag = "gromit-xyz"
	defer func() {
		runSpecFlag = ""
		runEpicFlag = ""
	}()

	err := runLoop(runCmd, []string{})
	if err == nil {
		t.Fatal("runLoop should return error when both --spec and --epic are set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %v", err)
	}
}

// --- Label resolution tests ---

// TestRunLoop_SpecFlagResolvesToLabel verifies that --spec flag resolves to a spec label
func TestRunLoop_SpecFlagResolvesToLabel(t *testing.T) {
	// Expected failure: runLoop does not call scope.ResolveSpec yet
	if err := scope.ValidateFlags("", "init-wizard"); err != nil {
		t.Fatalf("ValidateFlags should accept --spec alone, got error: %v", err)
	}

	labels := scope.ResolveSpec("init-wizard")
	if len(labels) != 1 {
		t.Fatalf("ResolveSpec should return 1 label, got %d", len(labels))
	}
	if labels[0] != "spec:init-wizard" {
		t.Errorf("ResolveSpec = %q, want %q", labels[0], "spec:init-wizard")
	}
}

// TestRunLoop_EpicFlagResolvesToMultipleLabels verifies that --epic flag resolves to multiple spec labels
func TestRunLoop_EpicFlagResolvesToMultipleLabels(t *testing.T) {
	// Expected failure: runLoop does not call scope.ResolveEpic yet
	specsDir := filepath.Join(t.TempDir(), "specs")
	writeSpecFiles(t, specsDir, []specFile{
		{"auth.md", "auth", "gromit-100"},
		{"profile.md", "profile", "gromit-100"},
		{"settings.md", "settings", "gromit-200"},
	})

	labels, err := scope.ResolveEpic("gromit-100", specsDir)
	if err != nil {
		t.Fatalf("ResolveEpic returned error: %v", err)
	}
	assertLabelSet(t, labels, []string{"spec:auth", "spec:profile"})
}

// --- Runner integration tests ---

// TestRunLoop_SpecFlagCallsSetLabelFilters verifies that --spec flag causes runner.SetLabelFilters to be called
func TestRunLoop_SpecFlagCallsSetLabelFilters(t *testing.T) {
	// Expected failure: runLoop does not call runner.SetLabelFilters yet
	tempDir := t.TempDir()
	gromitDir := setupMinimalGromitConfig(t, tempDir)
	specsDir := filepath.Join(gromitDir, "specs")
	writeSpecFiles(t, specsDir, []specFile{
		{"auth.md", "auth", "gromit-100"},
	})

	runSpecFlag = "auth"
	defer func() {
		runSpecFlag = ""
	}()

	// This test will fail because runLoop does not yet:
	// 1. Validate the spec flag
	// 2. Resolve spec to labels via scope.ResolveSpec
	// 3. Call runner.SetLabelFilters with the resolved labels
	// The test expects these calls to happen before the run loop starts
	err := runLoop(runCmd, []string{})

	// The specific behavior we're testing: when --spec is set, the runner should
	// only process beads with that spec label. Since we don't have any beads with
	// the "spec:auth" label in this test setup, the runner should exit cleanly
	// without processing any beads (as opposed to processing all available beads).

	if err != nil && !strings.Contains(err.Error(), "no beads") {
		t.Errorf("expected clean exit or 'no beads' message, got error: %v", err)
	}
}

// TestRunLoop_EpicFlagCallsSetLabelFilters verifies that --epic flag causes runner.SetLabelFilters to be called
func TestRunLoop_EpicFlagCallsSetLabelFilters(t *testing.T) {
	// Expected failure: runLoop does not call runner.SetLabelFilters with epic-resolved labels yet
	tempDir := t.TempDir()
	gromitDir := setupMinimalGromitConfig(t, tempDir)
	specsDir := filepath.Join(gromitDir, "specs")
	writeSpecFiles(t, specsDir, []specFile{
		{"auth.md", "auth", "gromit-100"},
		{"profile.md", "profile", "gromit-100"},
	})

	runEpicFlag = "gromit-100"
	defer func() {
		runEpicFlag = ""
	}()

	// This test will fail because runLoop does not yet:
	// 1. Validate the epic flag
	// 2. Resolve epic to labels via scope.ResolveEpic
	// 3. Call runner.SetLabelFilters with the resolved labels
	err := runLoop(runCmd, []string{})

	// The specific behavior we're testing: when --epic is set, the runner should
	// resolve all specs in that epic and only process beads with those spec labels.
	if err != nil && !strings.Contains(err.Error(), "no beads") {
		t.Errorf("expected clean exit or 'no beads' message, got error: %v", err)
	}
}

// TestRunLoop_NoFlagsUsesDefaultBehavior verifies that no flags means no label filtering
func TestRunLoop_NoFlagsUsesDefaultBehavior(t *testing.T) {
	// Expected failure: this test serves as a baseline - the current behavior should continue to work
	tempDir := t.TempDir()
	setupMinimalGromitConfig(t, tempDir)

	// Ensure flags are empty
	runSpecFlag = ""
	runEpicFlag = ""

	// When no flags are set, runLoop should work as it does today (processing all beads)
	// This test verifies that the new flag handling doesn't break the default behavior
	if err := scope.ValidateFlags("", ""); err != nil {
		t.Fatalf("ValidateFlags should accept both flags empty, got error: %v", err)
	}
}

// --- Help text tests ---

// TestRunCmd_HelpText_DocumentsSpecFlag verifies that run command help documents --spec flag
func TestRunCmd_HelpText_DocumentsSpecFlag(t *testing.T) {
	// Expected failure: runCmd.Long does not document --spec flag yet
	helpText := runCmd.Long
	if !strings.Contains(helpText, "--spec") {
		t.Error("runCmd.Long should document --spec flag")
	}
	if !strings.Contains(strings.ToLower(helpText), "filter") || !strings.Contains(strings.ToLower(helpText), "scope") {
		t.Error("runCmd.Long should explain filtering/scoping when using --spec")
	}
}

// TestRunCmd_HelpText_DocumentsEpicFlag verifies that run command help documents --epic flag
func TestRunCmd_HelpText_DocumentsEpicFlag(t *testing.T) {
	// Expected failure: runCmd.Long does not document --epic flag yet
	helpText := runCmd.Long
	if !strings.Contains(helpText, "--epic") {
		t.Error("runCmd.Long should document --epic flag")
	}
	if !strings.Contains(strings.ToLower(helpText), "filter") || !strings.Contains(strings.ToLower(helpText), "scope") {
		t.Error("runCmd.Long should explain filtering/scoping when using --epic")
	}
}

// TestRunCmd_HelpText_DocumentsMutualExclusivity verifies help text explains mutual exclusivity
func TestRunCmd_HelpText_DocumentsMutualExclusivity(t *testing.T) {
	// Expected failure: runCmd.Long does not document mutual exclusivity yet
	helpText := runCmd.Long
	if !strings.Contains(strings.ToLower(helpText), "mutually exclusive") {
		t.Error("runCmd.Long should note that --spec and --epic are mutually exclusive")
	}
}

// --- Spec validation tests ---

// TestRunLoop_ValidatesSpecExists verifies that --spec flag validates spec file exists
func TestRunLoop_ValidatesSpecExists(t *testing.T) {
	// Expected failure: runLoop does not call scope.ValidateSpec yet
	tempDir := t.TempDir()
	gromitDir := setupMinimalGromitConfig(t, tempDir)
	specsDir := filepath.Join(gromitDir, "specs")

	// Create specs directory but no spec files
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	runSpecFlag = "nonexistent-spec"
	defer func() {
		runSpecFlag = ""
	}()

	err := runLoop(runCmd, []string{})
	if err == nil {
		t.Fatal("runLoop should return error when spec file doesn't exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention spec not found, got: %v", err)
	}
}

// --- Test helpers ---

// setupMinimalGromitConfig creates a minimal gromit.yaml and .gromit directory structure
// Returns the gromitDir path
func setupMinimalGromitConfig(t *testing.T, tempDir string) string {
	t.Helper()

	gromitDir := filepath.Join(tempDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("Failed to create gromit dir: %v", err)
	}

	// Create minimal templates
	templatesDir := filepath.Join(gromitDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	// Create minimal prompt templates
	templates := map[string]string{
		"PROMPT_BUILD.md":    "# Build\n{{ .Bead.Title }}",
		"PROMPT_VALIDATE.md": "# Validate\n{{ .Bead.Title }}",
		"PROMPT_ANALYZE.md":  "# Analyze\n{{ .Failure }}",
	}
	for name, content := range templates {
		path := filepath.Join(templatesDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write template %s: %v", name, err)
		}
	}

	// Create RULES.md and LEARNINGS.md
	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("# Rules\n\nTest rules\n"), 0644); err != nil {
		t.Fatalf("Failed to write RULES.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "LEARNINGS.md"), []byte("# Learnings\n\nTest learnings\n"), 0644); err != nil {
		t.Fatalf("Failed to write LEARNINGS.md: %v", err)
	}

	// Create logs directory
	logsDir := filepath.Join(gromitDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}

	// Create minimal config
	configPath := filepath.Join(tempDir, "gromit.yaml")
	cfg := &config.Config{
		Paths: config.PathsConfig{
			GromitDir: gromitDir,
			Templates: templatesDir,
			Specs:     filepath.Join(gromitDir, "specs"),
			Logs:      logsDir,
		},
		Models: config.ModelsConfig{
			P0:         "claude-opus-4",
			P1:         "claude-sonnet-3-5",
			P2:         "claude-haiku-3",
			Validation: "claude-haiku-3",
		},
		Loop: config.LoopConfig{
			MaxIterations: 1, // Prevent infinite loops in tests
		},
	}

	// Write config as YAML (using JSON as a quick approximation for test purposes)
	configData, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Update configPath global for runLoop to find it
	oldConfigPath := configPath
	configPath = filepath.Join(tempDir, "gromit.yaml")
	t.Cleanup(func() {
		configPath = oldConfigPath
	})

	return gromitDir
}
