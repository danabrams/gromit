//go:build acceptance

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestGetSpecBaseCommit_ValidatesSpecBeforeResolution tests that getSpecBaseCommit
// validates the spec file exists before attempting to resolve it to beads
func TestGetSpecBaseCommit_ValidatesSpecBeforeResolution(t *testing.T) {
	// Expected failure: getSpecBaseCommit does not accept specsDir parameter yet
	// Current signature: func getSpecBaseCommit(specName string) (string, error)
	// Expected signature: func getSpecBaseCommit(specName string, specsDir string) (string, error)
	//
	// This test verifies that getSpecBaseCommit calls ValidateSpec before ResolveSpec,
	// providing better error messages when spec files don't exist.

	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create a spec file
	specPath := filepath.Join(specsDir, "existing-spec.md")
	specContent := `---
id: existing-spec
created: 2026-02-11
---

# Existing Spec
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	// Test with nonexistent spec - should fail with helpful error
	_, err := getSpecBaseCommit("nonexistent-spec", specsDir)
	if err == nil {
		t.Fatal("getSpecBaseCommit with nonexistent spec should return error")
	}

	errMsg := err.Error()
	// Error should indicate spec not found
	if !strings.Contains(strings.ToLower(errMsg), "not found") {
		t.Errorf("Error should indicate spec not found, got: %v", err)
	}

	// Error should list available specs
	if !strings.Contains(errMsg, "existing-spec") {
		t.Errorf("Error should list available spec 'existing-spec', got: %v", err)
	}

	// Error should NOT be the generic "no beads found" message
	if strings.Contains(errMsg, "no beads found") {
		t.Errorf("Error should be about spec validation, not bead listing. Got: %v", err)
	}
}

// TestGetSpecBaseCommit_ValidatesBeforeBeadLookup tests that spec validation
// happens before attempting to query beads
func TestGetSpecBaseCommit_ValidatesBeforeBeadLookup(t *testing.T) {
	// Expected failure: getSpecBaseCommit does not call ValidateSpec yet
	//
	// This test verifies that when a spec file doesn't exist, we get a validation error
	// before attempting to call bead.ListWithLabel, which would return "no beads found".

	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create several spec files
	for _, name := range []string{"auth", "profile", "settings"} {
		specPath := filepath.Join(specsDir, name+".md")
		content := fmt.Sprintf(`---
id: %s
created: 2026-02-11
---

# Spec
`, name)
		if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write spec file: %v", err)
		}
	}

	// Try to get base commit for typo'd spec name
	_, err := getSpecBaseCommit("authh", specsDir)
	if err == nil {
		t.Fatal("getSpecBaseCommit with typo'd spec name should return error")
	}

	errMsg := err.Error()

	// Should get spec validation error (listing available specs)
	// Not bead lookup error ("no beads found")
	if strings.Contains(errMsg, "no beads found") {
		t.Errorf("Should fail at validation stage (not bead lookup). Got: %v", err)
	}

	// Should suggest available specs
	availableSpecs := []string{"auth", "profile", "settings"}
	for _, spec := range availableSpecs {
		if !strings.Contains(errMsg, spec) {
			t.Errorf("Error should list available spec %q, got: %v", spec, err)
		}
	}
}

// TestGetSpecBaseCommit_AcceptsSpecsDirParameter tests that getSpecBaseCommit
// accepts specsDir as a parameter instead of using a global
func TestGetSpecBaseCommit_AcceptsSpecsDirParameter(t *testing.T) {
	// Expected failure: getSpecBaseCommit signature is currently:
	//   func getSpecBaseCommit(specName string) (string, error)
	// Expected signature:
	//   func getSpecBaseCommit(specName string, specsDir string) (string, error)
	//
	// This test verifies the function signature has been updated to accept specsDir.

	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	specName := "test-spec"
	specPath := filepath.Join(specsDir, specName+".md")
	content := `---
id: test-spec
created: 2026-02-11
---

# Test Spec
`
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	// This call should compile with the new signature
	_, err := getSpecBaseCommit(specName, specsDir)

	// We expect an error because there are no beads for this spec,
	// but it should NOT be a validation error
	if err != nil {
		if strings.Contains(err.Error(), "not found") && strings.Contains(err.Error(), "available") {
			t.Errorf("Should not fail validation (spec exists), got: %v", err)
		}
		// "no beads found" or "no commits found" are acceptable errors at this stage
	}
}

// TestDetermineReviewScope_PassesSpecsDirToGetSpecBaseCommit tests that
// determineReviewScope passes cfg.Paths.Specs to getSpecBaseCommit
func TestDetermineReviewScope_PassesSpecsDirToGetSpecBaseCommit(t *testing.T) {
	// Expected failure: determineReviewScope calls getSpecBaseCommit without specsDir parameter
	// Current call at line 119: return getSpecBaseCommit(reviewSpec)
	// Expected call: return getSpecBaseCommit(reviewSpec, cfg.Paths.Specs)
	//
	// This test verifies that determineReviewScope correctly passes the specsDir
	// from config to getSpecBaseCommit.

	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create spec file
	specName := "test-review-spec"
	specPath := filepath.Join(specsDir, specName+".md")
	content := `---
id: test-review-spec
created: 2026-02-11
---

# Test Review Spec
`
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	// Create config with custom specs directory
	cfg := &config.Config{}
	cfg.Paths.Specs = specsDir

	// Set review flags
	saveReviewFlags(t)
	reviewSpec = specName
	reviewSince = ""
	reviewEpic = ""

	// Call determineReviewScope - it should pass specsDir to getSpecBaseCommit
	_, err := determineReviewScope(cfg)

	// We expect an error because there are no beads for this spec
	// But the error should NOT be about the spec file not existing
	if err != nil {
		if strings.Contains(err.Error(), "not found") && strings.Contains(err.Error(), "Available") {
			t.Errorf("Spec file should have been found via cfg.Paths.Specs. Got validation error: %v", err)
		}
		// "no beads found" or "no commits found" are expected at this stage
	}
}

func TestReviewCommand_SpecValidationScenarios(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		reviewSpec   string
		specFiles    []string
		otherFiles   []string
		wantContains []string
		wantExcludes []string
	}{
		{
			name:         "typo includes available specs",
			reviewSpec:   "user-managment",
			specFiles:    []string{"auth-service.md", "user-management.md", "api-gateway.md"},
			wantContains: []string{"not found", "auth-service", "user-management", "api-gateway"},
			wantExcludes: []string{"no beads found"},
		},
		{
			name:         "empty specs directory reports missing specs",
			reviewSpec:   "any-spec",
			specFiles:    nil,
			otherFiles:   nil,
			wantContains: []string{"no spec"},
			wantExcludes: []string{"no beads found"},
		},
		{
			name:         "non markdown files are ignored",
			reviewSpec:   "missing-spec",
			specFiles:    []string{"feature-a.md", "feature-b.md"},
			otherFiles:   []string{"README.txt", "notes.json", "template.yaml"},
			wantContains: []string{"feature-a", "feature-b"},
			wantExcludes: []string{"readme", "notes", "template"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errMsg := runReviewSpecValidationScenario(t, tc.reviewSpec, tc.specFiles, tc.otherFiles)
			for _, want := range tc.wantContains {
				if !strings.Contains(strings.ToLower(errMsg), strings.ToLower(want)) {
					t.Fatalf("error should contain %q, got: %s", want, errMsg)
				}
			}
			for _, avoid := range tc.wantExcludes {
				if strings.Contains(strings.ToLower(errMsg), strings.ToLower(avoid)) {
					t.Fatalf("error should not contain %q, got: %s", avoid, errMsg)
				}
			}
		})
	}
}

// TestReviewCommand_SpecValidationBeforeBeadClient tests that ValidateSpec
// is called before creating a bead client
func TestReviewCommand_SpecValidationBeforeBeadClient(t *testing.T) {
	// Expected failure: getSpecBaseCommit creates bead.NewClient() before validating spec
	//
	// This test verifies the calling order:
	// 1. ValidateSpec (file system check)
	// 2. ResolveSpec (label construction)
	// 3. bead.NewClient() (only if spec is valid)
	// 4. ListWithLabel (only if spec is valid)
	//
	// Currently getSpecBaseCommit goes straight to ResolveSpec -> NewClient -> ListWithLabel

	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create one valid spec
	specPath := filepath.Join(specsDir, "valid-spec.md")
	if err := os.WriteFile(specPath, []byte("---\nid: valid-spec\n---\n# Spec"), 0644); err != nil {
		t.Fatalf("Failed to write spec: %v", err)
	}

	// Try with invalid spec name
	_, err := getSpecBaseCommit("invalid-spec", specsDir)
	if err == nil {
		t.Fatal("getSpecBaseCommit with invalid spec should return error")
	}

	errMsg := err.Error()

	// Error should be about spec validation (file not found)
	if !strings.Contains(strings.ToLower(errMsg), "not found") {
		t.Errorf("Error should be about spec file not found, got: %v", err)
	}

	// Error should list available specs
	if !strings.Contains(errMsg, "valid-spec") {
		t.Errorf("Error should list available spec, got: %v", err)
	}

	// Should NOT reach bead operations
	if strings.Contains(errMsg, "bead") || strings.Contains(errMsg, "creating bead client") {
		t.Errorf("Should fail at validation before reaching bead operations. Got: %v", err)
	}
}

// TestGetSpecBaseCommit_ExistingSpecNoBeads tests that when spec exists but has no beads,
// we get a different error than when spec doesn't exist
func TestGetSpecBaseCommit_ExistingSpecNoBeads(t *testing.T) {
	// Expected failure: Without ValidateSpec, both cases give "no beads found" error
	//
	// This test verifies we can distinguish between:
	// - Spec file doesn't exist → validation error listing available specs
	// - Spec file exists but no beads tagged with it → bead lookup error
	//
	// Currently both cases return "no beads found for spec X"

	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create a spec file
	specName := "empty-spec"
	specPath := filepath.Join(specsDir, specName+".md")
	content := `---
id: empty-spec
created: 2026-02-11
---

# Empty Spec

This spec exists but has no beads tagged with it.
`
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	// Test with existing spec (no beads)
	_, existingErr := getSpecBaseCommit(specName, specsDir)
	if existingErr == nil {
		t.Fatal("getSpecBaseCommit should return error when spec has no beads")
	}

	// Test with nonexistent spec
	_, nonexistentErr := getSpecBaseCommit("does-not-exist", specsDir)
	if nonexistentErr == nil {
		t.Fatal("getSpecBaseCommit should return error when spec doesn't exist")
	}

	// Errors should be different
	existingMsg := existingErr.Error()
	nonexistentMsg := nonexistentErr.Error()

	// Nonexistent spec should mention "not found" and list available specs
	if !strings.Contains(strings.ToLower(nonexistentMsg), "not found") {
		t.Errorf("Nonexistent spec error should mention 'not found', got: %v", nonexistentErr)
	}
	if !strings.Contains(nonexistentMsg, specName) {
		t.Errorf("Nonexistent spec error should list available spec %q, got: %v", specName, nonexistentErr)
	}

	// Existing spec error should mention "no beads" or "no commits"
	if !strings.Contains(strings.ToLower(existingMsg), "no bead") && !strings.Contains(strings.ToLower(existingMsg), "no commit") {
		t.Errorf("Existing spec error should mention no beads/commits, got: %v", existingErr)
	}

	// Existing spec error should NOT list "Available specs:"
	if strings.Contains(existingMsg, "Available") && strings.Contains(existingMsg, "spec") {
		t.Errorf("Existing spec error should not suggest alternatives (spec exists!), got: %v", existingErr)
	}
}

func runReviewSpecValidationScenario(t *testing.T, specName string, specFiles []string, otherFiles []string) string {
	t.Helper()

	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	for _, file := range specFiles {
		baseName := strings.TrimSuffix(file, filepath.Ext(file))
		content := fmt.Sprintf(`---
id: %s
created: 2026-02-11
---

# %s
`, baseName, baseName)
		path := filepath.Join(specsDir, file)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write spec: %v", err)
		}
	}
	for _, file := range otherFiles {
		path := filepath.Join(specsDir, file)
		if err := os.WriteFile(path, []byte("not a spec"), 0644); err != nil {
			t.Fatalf("Failed to write non-spec file: %v", err)
		}
	}

	cfg := &config.Config{}
	cfg.Paths.Specs = specsDir

	saveReviewFlags(t)
	reviewSpec = specName
	reviewSince = ""
	reviewEpic = ""

	_, err := determineReviewScope(cfg)
	if err == nil {
		t.Fatal("determineReviewScope should return error for invalid spec")
	}

	return err.Error()
}
