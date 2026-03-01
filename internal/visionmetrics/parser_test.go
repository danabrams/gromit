package visionmetrics

import (
	"testing"
)

func TestParseFromPRBody_AllFieldsValid(t *testing.T) {
	prBody := `
# Vision Metrics

spec_id: spec-2026-001
cycle_start_trigger_at: 2026-02-25T08:00:00Z
cycle_end_presented_at: 2026-02-28T16:00:00Z
review_outcome: accepted
human_tactical_intervention: no
human_debugging_intervention: no
escaped_regression_within_7d: no
`

	rec, err := ParseFromPRBody(prBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.SpecID != "spec-2026-001" {
		t.Fatalf("spec_id mismatch: got %q", rec.SpecID)
	}
	if rec.ReviewOutcome != ReviewOutcomeAccepted {
		t.Fatalf("review_outcome mismatch: got %v", rec.ReviewOutcome)
	}
	if rec.HumanTacticalIntervention != No {
		t.Fatalf("human_tactical_intervention mismatch: got %v", rec.HumanTacticalIntervention)
	}

	// Validate it passes the visionmetrics validation
	errs := Validate(rec)
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %d: %v", len(errs), errs)
	}
}

func TestParseFromPRBody_VisionChangeWithRationale(t *testing.T) {
	prBody := `
# Vision Metrics

spec_id: spec-2026-002
cycle_start_trigger_at: 2026-02-24T10:00:00Z
cycle_end_presented_at: 2026-02-27T14:00:00Z
review_outcome: rework_vision_change
review_rationale: Design pattern changed to accommodate new requirement
human_tactical_intervention: yes
human_debugging_intervention: no
escaped_regression_within_7d: pending
`

	rec, err := ParseFromPRBody(prBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.ReviewOutcome != ReviewOutcomeVisionChange {
		t.Fatalf("expected rework_vision_change, got %v", rec.ReviewOutcome)
	}
	if rec.ReviewRationale != "Design pattern changed to accommodate new requirement" {
		t.Fatalf("review_rationale mismatch: got %q", rec.ReviewRationale)
	}
	if rec.EscapedRegressionWithin7D != EscapedRegressionPending {
		t.Fatalf("expected pending regression status, got %v", rec.EscapedRegressionWithin7D)
	}

	// Validate it passes the visionmetrics validation
	errs := Validate(rec)
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %d: %v", len(errs), errs)
	}
}

func TestParseFromPRBody_DebuggingIntervention(t *testing.T) {
	prBody := `
# Vision Metrics

spec_id: spec-2026-003
cycle_start_trigger_at: 2026-02-20T09:00:00Z
cycle_end_presented_at: 2026-02-25T17:00:00Z
review_outcome: accepted
human_tactical_intervention: yes
human_debugging_intervention: yes
escaped_regression_within_7d: yes
`

	rec, err := ParseFromPRBody(prBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.HumanDebuggingIntervention != Yes {
		t.Fatalf("expected debugging intervention=yes, got %v", rec.HumanDebuggingIntervention)
	}
	if rec.HumanTacticalIntervention != Yes {
		t.Fatalf("expected tactical intervention=yes, got %v", rec.HumanTacticalIntervention)
	}

	errs := Validate(rec)
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %d: %v", len(errs), errs)
	}
}

func TestParseFromPRBody_MissingFields(t *testing.T) {
	prBody := `
# Vision Metrics

review_outcome: accepted
`

	rec, err := ParseFromPRBody(prBody)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	// Validation should catch missing fields
	errs := Validate(rec)
	if len(errs) == 0 {
		t.Fatalf("expected validation errors for missing fields")
	}

	// Should have errors for spec_id, dates, etc.
	expectedFields := map[string]bool{
		FieldSpecID:              false,
		FieldCycleStartTriggerAt: false,
		FieldCycleEndPresentedAt: false,
	}

	for _, err := range errs {
		for field := range expectedFields {
			if err.Field == field {
				expectedFields[field] = true
			}
		}
	}

	for field, found := range expectedFields {
		if !found {
			t.Fatalf("expected error for field %s", field)
		}
	}
}

func TestParseFromPRBody_InvalidDateFormat(t *testing.T) {
	prBody := `
# Vision Metrics

spec_id: test-spec
cycle_start_trigger_at: invalid-date
`

	_, err := ParseFromPRBody(prBody)
	if err == nil {
		t.Fatalf("expected parse error for invalid date format")
	}
}

func TestParseFromPRBody_IgnoresNonMetadataLines(t *testing.T) {
	prBody := `
# PR Title and Description

This is a regular PR description with some content.

## Vision Metrics

spec_id: spec-2026-004
cycle_start_trigger_at: 2026-02-21T10:00:00Z
cycle_end_presented_at: 2026-02-26T15:00:00Z
review_outcome: accepted
human_tactical_intervention: no
human_debugging_intervention: no
escaped_regression_within_7d: no

## Additional Notes

Some additional context about the PR that shouldn't affect parsing.
`

	rec, err := ParseFromPRBody(prBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.SpecID != "spec-2026-004" {
		t.Fatalf("expected spec_id 'spec-2026-004', got %q", rec.SpecID)
	}

	errs := Validate(rec)
	if len(errs) != 0 {
		t.Fatalf("expected no validation errors, got %d: %v", len(errs), errs)
	}
}

func TestParseFromPRBody_HandlesPresentedAtBeforeStart(t *testing.T) {
	prBody := `
# Vision Metrics

spec_id: test-spec
cycle_start_trigger_at: 2026-02-25T10:00:00Z
cycle_end_presented_at: 2026-02-25T09:00:00Z
review_outcome: accepted
human_tactical_intervention: no
human_debugging_intervention: no
escaped_regression_within_7d: no
`

	rec, err := ParseFromPRBody(prBody)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	// Validation should catch this chronological error
	errs := Validate(rec)
	if len(errs) == 0 {
		t.Fatalf("expected validation error for end before start")
	}

	foundChronoError := false
	for _, err := range errs {
		if err.Field == FieldCycleEndPresentedAt && err.Reason == "must be after cycle start" {
			foundChronoError = true
		}
	}
	if !foundChronoError {
		t.Fatalf("expected chronological validation error, got: %v", errs)
	}
}
