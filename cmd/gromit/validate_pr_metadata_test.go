package main

import (
	"os"
	"strings"
	"testing"
)

func TestValidatePRMetadata_ValidPRBody(t *testing.T) {
	prBody := `# PR Description

## Vision Metrics

spec_id: test-spec-001
cycle_start_trigger_at: 2026-02-25T10:00:00Z
cycle_end_presented_at: 2026-02-28T14:30:00Z
review_outcome: accepted
human_tactical_intervention: no
human_debugging_intervention: no
escaped_regression_within_7d: no

Other PR content...
`

	// Test that ValidatePRMetadata returns no errors for valid metadata
	errs := ValidatePRMetadata(prBody)
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %d: %v", len(errs), errs)
	}
}

func TestValidatePRMetadata_MissingRequiredField(t *testing.T) {
	prBody := `# PR Description

## Vision Metrics

cycle_start_trigger_at: 2026-02-25T10:00:00Z
cycle_end_presented_at: 2026-02-28T14:30:00Z
review_outcome: accepted
human_tactical_intervention: no
human_debugging_intervention: no
escaped_regression_within_7d: no
`

	// Test that ValidatePRMetadata returns error for missing spec_id
	errs := ValidatePRMetadata(prBody)
	if len(errs) == 0 {
		t.Fatalf("expected validation errors for missing spec_id, got none")
	}

	foundSpecIDError := false
	for _, err := range errs {
		if strings.Contains(err, "spec_id") {
			foundSpecIDError = true
		}
	}
	if !foundSpecIDError {
		t.Fatalf("expected error mentioning spec_id field, got: %v", errs)
	}
}

func TestValidatePRMetadata_InvalidEnumValue(t *testing.T) {
	prBody := `# PR Description

## Vision Metrics

spec_id: test-spec-001
cycle_start_trigger_at: 2026-02-25T10:00:00Z
cycle_end_presented_at: 2026-02-28T14:30:00Z
review_outcome: invalid_value
human_tactical_intervention: no
human_debugging_intervention: no
escaped_regression_within_7d: no
`

	errs := ValidatePRMetadata(prBody)
	if len(errs) == 0 {
		t.Fatalf("expected validation error for invalid review_outcome, got none")
	}

	foundReviewOutcomeError := false
	for _, err := range errs {
		if strings.Contains(err, "review_outcome") {
			foundReviewOutcomeError = true
		}
	}
	if !foundReviewOutcomeError {
		t.Fatalf("expected error mentioning review_outcome field, got: %v", errs)
	}
}

func TestValidatePRMetadata_VisionChangeRequiresRationale(t *testing.T) {
	prBody := `# PR Description

## Vision Metrics

spec_id: test-spec-001
cycle_start_trigger_at: 2026-02-25T10:00:00Z
cycle_end_presented_at: 2026-02-28T14:30:00Z
review_outcome: rework_vision_change
human_tactical_intervention: no
human_debugging_intervention: no
escaped_regression_within_7d: no
`

	errs := ValidatePRMetadata(prBody)
	if len(errs) == 0 {
		t.Fatalf("expected validation error for missing review_rationale, got none")
	}

	foundRationaleError := false
	for _, err := range errs {
		if strings.Contains(err, "review_rationale") {
			foundRationaleError = true
		}
	}
	if !foundRationaleError {
		t.Fatalf("expected error mentioning review_rationale field, got: %v", errs)
	}
}

func TestRunValidatePRMetadataCommand_Success(t *testing.T) {
	prBody := `# PR Description

## Vision Metrics

spec_id: test-spec-001
cycle_start_trigger_at: 2026-02-25T10:00:00Z
cycle_end_presented_at: 2026-02-28T14:30:00Z
review_outcome: accepted
human_tactical_intervention: no
human_debugging_intervention: no
escaped_regression_within_7d: no
`

	oldPRBody := os.Getenv("PR_BODY")
	defer func() {
		if oldPRBody != "" {
			os.Setenv("PR_BODY", oldPRBody)
		} else {
			os.Unsetenv("PR_BODY")
		}
	}()

	os.Setenv("PR_BODY", prBody)

	// Test that running the command with valid metadata exits successfully
	exitCode := runValidatePRMetadataCommand()
	if exitCode != 0 {
		t.Fatalf("expected exit code 0 for valid metadata, got %d", exitCode)
	}
}

func TestRunValidatePRMetadataCommand_Failure(t *testing.T) {
	prBody := `# PR Description

## Vision Metrics

spec_id:
cycle_start_trigger_at: 2026-02-25T10:00:00Z
cycle_end_presented_at: 2026-02-28T14:30:00Z
review_outcome: invalid_value
human_tactical_intervention: no
human_debugging_intervention: no
escaped_regression_within_7d: no
`

	oldPRBody := os.Getenv("PR_BODY")
	defer func() {
		if oldPRBody != "" {
			os.Setenv("PR_BODY", oldPRBody)
		} else {
			os.Unsetenv("PR_BODY")
		}
	}()

	os.Setenv("PR_BODY", prBody)

	// Test that running the command with invalid metadata exits with error code
	exitCode := runValidatePRMetadataCommand()
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code for invalid metadata, got %d", exitCode)
	}
}

func TestValidatePRMetadataCmd_Exists(t *testing.T) {
	// Test that validatePRMetadataCmd is registered and callable
	if validatePRMetadataCmd == nil {
		t.Fatalf("validatePRMetadataCmd should be defined")
	}

	if validatePRMetadataCmd.Use != "validate-pr-metadata" {
		t.Fatalf("expected command Use='validate-pr-metadata', got %q", validatePRMetadataCmd.Use)
	}
}
