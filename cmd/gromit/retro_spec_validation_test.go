package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/scope"
)

// TestRunRetro_SpecValidationPassesForExistingSpec tests that ValidateSpec succeeds
// when spec file exists
func TestRunRetro_SpecValidationPassesForExistingSpec(t *testing.T) {
	// Expected failure: This test documents expected behavior - ValidateSpec should return nil
	// for existing specs. The test will pass once ValidateSpec is called in main.go:207-208.

	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create a valid spec file
	specPath := filepath.Join(specsDir, "valid-spec.md")
	content := "---\nid: valid-spec\ncreated: 2026-02-11\n---\n\n# Valid Spec\n"
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write spec file: %v", err)
	}

	// Call ValidateSpec directly - should return nil
	err := scope.ValidateSpec(specsDir, "valid-spec")
	if err != nil {
		t.Errorf("ValidateSpec should return nil for existing spec, got error: %v", err)
	}

	// Verify ResolveSpec still works as expected
	labels := scope.ResolveSpec("valid-spec")
	if len(labels) != 1 || labels[0] != "spec:valid-spec" {
		t.Errorf("ResolveSpec should return [spec:valid-spec], got: %v", labels)
	}
}

// TestValidateSpecBeforeResolveSpec_OrderMatters tests that ValidateSpec is called
// before ResolveSpec in the retro handler
func TestValidateSpecBeforeResolveSpec_OrderMatters(t *testing.T) {
	// Expected failure: The code at main.go:207-208 does not call scope.ValidateSpec before scope.ResolveSpec
	//
	// This test verifies the behavioral difference between calling ValidateSpec first vs not:
	// - WITHOUT ValidateSpec: ResolveSpec succeeds and returns "spec:nonexistent",
	//   then later operations fail with less helpful errors like "no beads found"
	// - WITH ValidateSpec: Immediate error with helpful message listing available specs
	//
	// The key behavioral change is getting a helpful error BEFORE attempting to use the spec label.

	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create one spec file
	specPath := filepath.Join(specsDir, "real-spec.md")
	if err := os.WriteFile(specPath, []byte("---\nid: real-spec\n---\n# Real Spec\n"), 0644); err != nil {
		t.Fatalf("Failed to write spec: %v", err)
	}

	// Without ValidateSpec: ResolveSpec works but produces a label for nonexistent spec
	labelsWithoutValidation := scope.ResolveSpec("nonexistent")
	if len(labelsWithoutValidation) != 1 || labelsWithoutValidation[0] != "spec:nonexistent" {
		t.Fatalf("ResolveSpec without validation should return label, got: %v", labelsWithoutValidation)
	}

	// With ValidateSpec: Should get error with helpful message
	err := scope.ValidateSpec(specsDir, "nonexistent")
	if err == nil {
		t.Fatal("ValidateSpec should return error for nonexistent spec")
	}

	errMsg := err.Error()

	// The key behavioral difference: helpful error message with available specs
	if !strings.Contains(errMsg, "not found") {
		t.Errorf("ValidateSpec error should say 'not found', got: %v", err)
	}

	if !strings.Contains(errMsg, "real-spec") {
		t.Errorf("ValidateSpec error should list available spec 'real-spec', got: %v", err)
	}

	// This demonstrates the value: calling ValidateSpec before ResolveSpec gives users
	// a helpful error immediately, rather than letting them proceed with an invalid label
	// that will fail later with a cryptic "no beads found" message.
}

// TestReviewCommandSpecValidation tests that review command validates specs
func TestReviewCommandSpecValidation(t *testing.T) {
	// Expected failure: This test demonstrates the target behavior that retro should match.
	// The review command calls ValidateSpec at review.go:150 in getSpecBaseCommit.
	// The retro command should do the same at main.go:207-208.
	//
	// This test verifies getSpecBaseCommit returns helpful error with available specs.

	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create spec files
	for _, name := range []string{"auth", "profile"} {
		path := filepath.Join(specsDir, name+".md")
		content := fmt.Sprintf("---\nid: %s\n---\n# %s\n", name, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write spec: %v", err)
		}
	}

	// Test getSpecBaseCommit with nonexistent spec
	_, err := getSpecBaseCommit(mockBeadClientEmptyList(), "nonexistent", specsDir)
	if err == nil {
		t.Fatal("getSpecBaseCommit should return error for nonexistent spec")
	}

	errMsg := err.Error()

	// Should get validation error with available specs
	if !strings.Contains(errMsg, "not found") {
		t.Errorf("Error should indicate spec not found, got: %v", err)
	}

	// Should list available specs
	if !strings.Contains(errMsg, "auth") || !strings.Contains(errMsg, "profile") {
		t.Errorf("Error should list available specs, got: %v", err)
	}
}

// TestRetroSpecValidation_HelpfulErrorForTypo tests that retro command provides
// helpful error when spec name has a typo
func TestRetroSpecValidation_HelpfulErrorForTypo(t *testing.T) {
	// Expected failure: main.go:207-208 does not call scope.ValidateSpec
	// Current behavior: ResolveSpec("auth-typo") returns "spec:auth-typo", which later fails
	//   with "no beads found for spec" - unhelpful for typos
	// Expected behavior: ValidateSpec("auth-typo") returns error listing "auth" and other
	//   available specs, helping user spot the typo immediately
	//
	// This test demonstrates the user-facing value of validation.

	tempDir := t.TempDir()
	specsDir := filepath.Join(tempDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	// Create multiple spec files
	specNames := []string{"auth", "profile", "database"}
	for _, name := range specNames {
		path := filepath.Join(specsDir, name+".md")
		content := fmt.Sprintf("---\nid: %s\n---\n# %s\n", name, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write spec: %v", err)
		}
	}

	// Simulate typo: "auht" instead of "auth"
	err := scope.ValidateSpec(specsDir, "auht")
	if err == nil {
		t.Fatal("ValidateSpec should return error for typo in spec name")
	}

	errMsg := err.Error()

	// Error should mention the typo'd name
	if !strings.Contains(errMsg, "auht") {
		t.Errorf("Error should mention the requested spec 'auht', got: %v", err)
	}

	// Error should list all available specs so user can spot the correct name
	for _, name := range specNames {
		if !strings.Contains(errMsg, name) {
			t.Errorf("Error should list available spec %q so user can spot typo, got: %v", name, err)
		}
	}

	// The value: user sees "auht not found. Available specs: auth, profile, database"
	// and immediately realizes the typo, rather than getting "no beads found for spec auht"
}
