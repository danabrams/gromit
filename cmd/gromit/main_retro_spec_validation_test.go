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
)

// setupRetroTestEnv creates a minimal test environment for retro command
func setupRetroTestEnv(t *testing.T) (tempDir, gromitDir, specsDir string, cleanup func()) {
	t.Helper()

	tempDir = t.TempDir()
	gromitDir = filepath.Join(tempDir, ".gromit")
	specsDir = filepath.Join(gromitDir, "specs")
	logsDir := filepath.Join(gromitDir, "logs")
	templatesDir := filepath.Join(gromitDir, "templates")

	// Create required directories
	for _, dir := range []string{specsDir, logsDir, templatesDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	// Create minimal config
	configPath := filepath.Join(tempDir, "gromit.yaml")
	cfg := &config.Config{
		Paths: config.PathsConfig{
			GromitDir: gromitDir,
			Specs:     specsDir,
		},
		Models: config.ModelsConfig{
			P0:         "claude-opus-4",
			P1:         "claude-sonnet-3-5",
			P2:         "claude-haiku-3",
			Validation: "claude-haiku-3",
		},
	}
	configData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create required files
	for _, path := range []string{
		filepath.Join(gromitDir, "RULES.md"),
		filepath.Join(gromitDir, "LEARNINGS.md"),
		filepath.Join(templatesDir, "PROMPT_RETRO.md"),
	} {
		if err := os.WriteFile(path, []byte("# Content\n"), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", path, err)
		}
	}

	// Create at least one iteration log entry
	logPath := filepath.Join(logsDir, "iteration.log")
	logEntry := logger.IterationLog{
		Timestamp:    time.Now(),
		Iteration:    1,
		BeadID:       "test-bead-001",
		BeadTitle:    "Test Bead",
		Model:        "claude-sonnet-3-5",
		Success:      true,
		Validated:    true,
		Escalated:    false,
		DurationMs:   5000,
		CostUSD:      0.01,
		InputTokens:  1000,
		OutputTokens: 500,
	}
	logData, _ := json.Marshal(logEntry)
	if err := os.WriteFile(logPath, append(logData, '\n'), 0644); err != nil {
		t.Fatalf("Failed to write log: %v", err)
	}

	// Save original flag values
	origSpec := retroSpecFlag
	origEpic := retroEpicFlag
	origConfigPath := configPath

	// Change to temp dir so config is found
	t.Chdir(tempDir)

	cleanup = func() {
		retroSpecFlag = origSpec
		retroEpicFlag = origEpic
		configPath = origConfigPath
	}

	return tempDir, gromitDir, specsDir, cleanup
}

// TestRunRetro_CallsValidateSpecForNonexistentSpec tests that runRetro calls ValidateSpec
// and returns a helpful error when the spec file doesn't exist
func TestRunRetro_CallsValidateSpecForNonexistentSpec(t *testing.T) {
	// Expected failure: runRetro at main.go:207-208 does NOT call scope.ValidateSpec
	// Current code: labels = scope.ResolveSpec(retroSpecFlag)
	// Expected code: if err := scope.ValidateSpec(filepath.Join(gromitDir, "specs"), retroSpecFlag); err != nil { return err }
	//               labels = scope.ResolveSpec(retroSpecFlag)
	//
	// This test verifies that runRetro returns a "spec not found" error listing available specs,
	// rather than proceeding with an invalid label that would fail later with "no beads found".

	_, _, specsDir, cleanup := setupRetroTestEnv(t)
	defer cleanup()

	// Create several spec files
	specNames := []string{"auth", "user-profile", "settings"}
	for _, name := range specNames {
		specPath := filepath.Join(specsDir, name+".md")
		content := fmt.Sprintf("---\nid: %s\ncreated: 2026-02-11\n---\n\n# %s Spec\n", name, name)
		if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write spec file: %v", err)
		}
	}

	// Set flags for nonexistent spec
	retroSpecFlag = "nonexistent-spec"
	retroEpicFlag = ""

	// Call runRetro with timeout to avoid hanging
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use a channel to capture the error from runRetro
	errChan := make(chan error, 1)
	go func() {
		errChan <- runRetro(retroCmd, []string{})
	}()

	var err error
	select {
	case err = <-errChan:
		// Got error from runRetro
	case <-ctx.Done():
		// If runRetro hangs or takes too long, that's also a failure mode
		t.Fatal("runRetro hung or took too long - likely because it didn't fail fast on invalid spec")
	}

	if err == nil {
		t.Fatal("runRetro with nonexistent spec should return error, got nil")
	}

	errMsg := err.Error()

	// Error should mention the nonexistent spec name
	if !strings.Contains(errMsg, "nonexistent-spec") {
		t.Errorf("Error should mention the requested spec 'nonexistent-spec', got: %v", err)
	}

	// Error should indicate spec not found
	if !strings.Contains(strings.ToLower(errMsg), "not found") {
		t.Errorf("Error should indicate spec not found, got: %v", err)
	}

	// Error should list available specs
	foundAllSpecs := true
	for _, name := range specNames {
		if !strings.Contains(errMsg, name) {
			t.Errorf("Error should list available spec %q, got: %v", name, err)
			foundAllSpecs = false
		}
	}

	if !foundAllSpecs {
		t.Logf("Full error message: %v", err)
	}
}

// TestRunRetro_SpecValidationBeforeBeadQuery tests that spec validation happens
// before attempting to query beads
func TestRunRetro_SpecValidationBeforeBeadQuery(t *testing.T) {
	// Expected failure: runRetro does not call ValidateSpec before ResolveSpec at line 208
	//
	// This test verifies the error message priority: spec validation errors should come
	// BEFORE any bead-related errors. The behavioral difference:
	// - WITHOUT ValidateSpec: Error is "failed to build bead filter" or similar
	// - WITH ValidateSpec: Error is "spec 'typo' not found. Available specs: ..."

	_, _, specsDir, cleanup := setupRetroTestEnv(t)
	defer cleanup()

	// Create one spec file
	specPath := filepath.Join(specsDir, "existing-spec.md")
	content := "---\nid: existing-spec\ncreated: 2026-02-11\n---\n\n# Existing Spec\n"
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	// Set flags for nonexistent spec
	retroSpecFlag = "typo-in-spec-name"
	retroEpicFlag = ""

	err := runRetro(retroCmd, []string{})
	if err == nil {
		t.Fatal("runRetro with nonexistent spec should return error")
	}

	errMsg := err.Error()

	// The key assertion: error should be about spec validation, NOT about beads or filters
	// This verifies that ValidateSpec is called BEFORE attempting to use the spec label
	if strings.Contains(strings.ToLower(errMsg), "bead") {
		t.Errorf("Error should be about spec validation, not beads. Got: %v", err)
		t.Logf("This indicates ValidateSpec was not called before buildBeadFilter")
	}

	if strings.Contains(strings.ToLower(errMsg), "filter") {
		t.Errorf("Error should be about spec validation, not filter building. Got: %v", err)
		t.Logf("This indicates ValidateSpec was not called before buildBeadFilter")
	}

	// Should indicate spec not found
	if !strings.Contains(strings.ToLower(errMsg), "not found") {
		t.Errorf("Error should indicate spec not found, got: %v", err)
	}

	// Should list the available spec
	if !strings.Contains(errMsg, "existing-spec") {
		t.Errorf("Error should list available spec 'existing-spec', got: %v", err)
	}
}

// TestRunRetro_SpecValidationWithEmptyDirectory tests error when specs directory is empty
func TestRunRetro_SpecValidationWithEmptyDirectory(t *testing.T) {
	// Expected failure: runRetro does not call ValidateSpec before ResolveSpec
	//
	// This test verifies that an empty specs directory produces a helpful error message
	// indicating no specs are available, rather than a generic "no beads found" error.

	_, _, _, cleanup := setupRetroTestEnv(t)
	defer cleanup()

	// specs directory exists but is empty (no .md files)

	retroSpecFlag = "any-spec"
	retroEpicFlag = ""

	err := runRetro(retroCmd, []string{})
	if err == nil {
		t.Fatal("runRetro with empty specs directory should return error")
	}

	errMsg := err.Error()

	// Error should indicate no specs available
	if !strings.Contains(strings.ToLower(errMsg), "no spec") && !strings.Contains(strings.ToLower(errMsg), "available") {
		t.Errorf("Error should indicate no specs available, got: %v", err)
	}

	// Should mention the requested spec
	if !strings.Contains(errMsg, "any-spec") {
		t.Errorf("Error should mention requested spec 'any-spec', got: %v", err)
	}
}

// TestRunRetro_ValidSpecPassesValidation tests that ValidateSpec
// allows retro to proceed when spec exists
func TestRunRetro_ValidSpecPassesValidation(t *testing.T) {
	// Expected failure: This test documents that valid specs should pass validation
	// After ValidateSpec is added to main.go:207, retro with valid spec should proceed
	// past validation (though it may fail later for other reasons like missing beads)

	_, _, specsDir, cleanup := setupRetroTestEnv(t)
	defer cleanup()

	// Create a valid spec file
	specPath := filepath.Join(specsDir, "valid-spec.md")
	content := "---\nid: valid-spec\ncreated: 2026-02-11\n---\n\n# Valid Spec\n"
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	retroSpecFlag = "valid-spec"
	retroEpicFlag = ""

	err := runRetro(retroCmd, []string{})

	// The test may fail for other reasons (no beads, etc), but the error should NOT
	// be about spec validation
	if err != nil {
		errMsg := err.Error()

		// Should NOT be a spec validation error
		if strings.Contains(errMsg, "spec") && strings.Contains(errMsg, "not found") {
			t.Errorf("Error should not be about spec validation for existing spec, got: %v", err)
		}

		// Acceptable errors: no beads found, missing git info, etc
		// Just ensure it's not rejecting the spec itself
		if strings.Contains(errMsg, "Available specs") {
			t.Errorf("Error lists available specs, suggesting spec was not found. Got: %v", err)
		}
	}
}
