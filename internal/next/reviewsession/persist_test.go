package reviewsession

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteOutcome(t *testing.T) {
	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, "evidence")

	outcome := &ReviewOutcome{
		RunID:      "test-run-001",
		ReviewedAt: time.Date(2026, 3, 19, 10, 30, 0, 0, time.UTC),
		Outcome:    OutcomeAccepted,
		Summary:    "All checks passed",
		ManualResults: []ManualCheckResult{
			{
				ID:     "check-1",
				Result: ResultPass,
				Notes:  "Implementation correct",
			},
		},
	}

	err := WriteOutcome(evidenceDir, outcome)
	if err != nil {
		t.Fatalf("WriteOutcome failed: %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("review-outcome.json not created: %v", err)
	}

	// Verify file is valid JSON by reading it back
	read, err := ReadOutcome(evidenceDir)
	if err != nil {
		t.Fatalf("ReadOutcome failed: %v", err)
	}

	// Verify content
	if read.RunID != outcome.RunID {
		t.Errorf("RunID mismatch: got %q, want %q", read.RunID, outcome.RunID)
	}
	if read.Outcome != outcome.Outcome {
		t.Errorf("Outcome mismatch: got %q, want %q", read.Outcome, outcome.Outcome)
	}
	if read.Summary != outcome.Summary {
		t.Errorf("Summary mismatch: got %q, want %q", read.Summary, outcome.Summary)
	}
	if len(read.ManualResults) != len(outcome.ManualResults) {
		t.Errorf("ManualResults length mismatch: got %d, want %d", len(read.ManualResults), len(outcome.ManualResults))
	}
	if len(read.ManualResults) > 0 && read.ManualResults[0].ID != outcome.ManualResults[0].ID {
		t.Errorf("ManualResult ID mismatch: got %q, want %q", read.ManualResults[0].ID, outcome.ManualResults[0].ID)
	}
}

func TestReadOutcomeNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, "nonexistent")

	_, err := ReadOutcome(evidenceDir)
	if err == nil {
		t.Error("ReadOutcome should fail for nonexistent directory")
	}
}

func TestWriteOutcomeNormalizesNilFields(t *testing.T) {
	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, "evidence")

	outcome := &ReviewOutcome{
		RunID:      "test-run-002",
		ReviewedAt: time.Date(2026, 3, 19, 10, 30, 0, 0, time.UTC),
		Outcome:    OutcomeAccepted,
		Summary:    "Test",
		// ManualResults is nil - should be normalized to empty slice
	}

	err := WriteOutcome(evidenceDir, outcome)
	if err != nil {
		t.Fatalf("WriteOutcome failed: %v", err)
	}

	read, err := ReadOutcome(evidenceDir)
	if err != nil {
		t.Fatalf("ReadOutcome failed: %v", err)
	}

	// ManualResults should be an empty slice, not nil
	if read.ManualResults == nil {
		t.Error("ManualResults should be normalized to empty slice, not nil")
	}
	if len(read.ManualResults) != 0 {
		t.Errorf("ManualResults should be empty, got %d items", len(read.ManualResults))
	}
}

func TestWriteOutcomeWithOverrideReason(t *testing.T) {
	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, "evidence")

	outcome := &ReviewOutcome{
		RunID:          "test-run-003",
		ReviewedAt:     time.Date(2026, 3, 19, 14, 45, 0, 0, time.UTC),
		Outcome:        OutcomeReworkImplementationGap,
		Summary:        "Implementation gap detected but overridden",
		OverrideReason: "Temporary workaround approved by tech lead",
		ManualResults: []ManualCheckResult{
			{
				ID:     "check-2",
				Result: ResultFail,
				Notes:  "Gap identified",
			},
			{
				ID:     "check-3",
				Result: ResultPass,
				Notes:  "Other checks passed",
			},
		},
	}

	err := WriteOutcome(evidenceDir, outcome)
	if err != nil {
		t.Fatalf("WriteOutcome failed: %v", err)
	}

	read, err := ReadOutcome(evidenceDir)
	if err != nil {
		t.Fatalf("ReadOutcome failed: %v", err)
	}

	// Verify all fields round-trip correctly
	if read.RunID != outcome.RunID {
		t.Errorf("RunID mismatch: got %q, want %q", read.RunID, outcome.RunID)
	}
	if !read.ReviewedAt.Equal(outcome.ReviewedAt) {
		t.Errorf("ReviewedAt mismatch: got %v, want %v", read.ReviewedAt, outcome.ReviewedAt)
	}
	if read.Outcome != outcome.Outcome {
		t.Errorf("Outcome mismatch: got %q, want %q", read.Outcome, outcome.Outcome)
	}
	if read.Summary != outcome.Summary {
		t.Errorf("Summary mismatch: got %q, want %q", read.Summary, outcome.Summary)
	}
	if read.OverrideReason != outcome.OverrideReason {
		t.Errorf("OverrideReason mismatch: got %q, want %q", read.OverrideReason, outcome.OverrideReason)
	}
	if len(read.ManualResults) != 2 {
		t.Errorf("ManualResults count mismatch: got %d, want 2", len(read.ManualResults))
	}
	if len(read.ManualResults) > 1 {
		if read.ManualResults[1].Result != ResultPass {
			t.Errorf("Second ManualResult mismatch: got %q, want %q", read.ManualResults[1].Result, ResultPass)
		}
	}
}

func TestReadOutcomeRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, "evidence")

	// Test with all fields populated
	original := &ReviewOutcome{
		RunID:          "round-trip-001",
		ReviewedAt:     time.Date(2026, 3, 19, 16, 20, 30, 0, time.UTC),
		Outcome:        OutcomeReworkVisionChange,
		Summary:        "Vision needs adjustment",
		OverrideReason: "Approved by product",
		ManualResults: []ManualCheckResult{
			{ID: "m1", Result: ResultUnsure, Notes: "Unclear specification"},
			{ID: "m2", Result: ResultSkipped, Notes: ""},
		},
	}

	// Write
	if err := WriteOutcome(evidenceDir, original); err != nil {
		t.Fatalf("WriteOutcome failed: %v", err)
	}

	// Read back
	read, err := ReadOutcome(evidenceDir)
	if err != nil {
		t.Fatalf("ReadOutcome failed: %v", err)
	}

	// Verify all fields match
	if read.RunID != original.RunID {
		t.Errorf("RunID mismatch")
	}
	if !read.ReviewedAt.Equal(original.ReviewedAt) {
		t.Errorf("ReviewedAt mismatch")
	}
	if read.Outcome != original.Outcome {
		t.Errorf("Outcome mismatch")
	}
	if read.Summary != original.Summary {
		t.Errorf("Summary mismatch")
	}
	if read.OverrideReason != original.OverrideReason {
		t.Errorf("OverrideReason mismatch")
	}
	if len(read.ManualResults) != len(original.ManualResults) {
		t.Errorf("ManualResults length mismatch: got %d, want %d", len(read.ManualResults), len(original.ManualResults))
	}
	for i, mr := range read.ManualResults {
		if mr.ID != original.ManualResults[i].ID {
			t.Errorf("ManualResult[%d].ID mismatch: got %q, want %q", i, mr.ID, original.ManualResults[i].ID)
		}
		if mr.Result != original.ManualResults[i].Result {
			t.Errorf("ManualResult[%d].Result mismatch: got %q, want %q", i, mr.Result, original.ManualResults[i].Result)
		}
		if mr.Notes != original.ManualResults[i].Notes {
			t.Errorf("ManualResult[%d].Notes mismatch: got %q, want %q", i, mr.Notes, original.ManualResults[i].Notes)
		}
	}
}

func TestWriteOutcomeIdempotency(t *testing.T) {
	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, "evidence")

	outcome := &ReviewOutcome{
		RunID:      "idempotent-test",
		ReviewedAt: time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC),
		Outcome:    OutcomeAccepted,
		Summary:    "All good",
		ManualResults: []ManualCheckResult{
			{ID: "idem-1", Result: ResultPass},
		},
	}

	// Write first time
	if err := WriteOutcome(evidenceDir, outcome); err != nil {
		t.Fatalf("First WriteOutcome failed: %v", err)
	}

	firstRead, err := ReadOutcome(evidenceDir)
	if err != nil {
		t.Fatalf("First ReadOutcome failed: %v", err)
	}

	// Write same outcome again (overwrite)
	if err := WriteOutcome(evidenceDir, outcome); err != nil {
		t.Fatalf("Second WriteOutcome failed: %v", err)
	}

	secondRead, err := ReadOutcome(evidenceDir)
	if err != nil {
		t.Fatalf("Second ReadOutcome failed: %v", err)
	}

	// Both reads should be identical
	if firstRead.RunID != secondRead.RunID {
		t.Errorf("RunID changed after overwrite")
	}
	if !firstRead.ReviewedAt.Equal(secondRead.ReviewedAt) {
		t.Errorf("ReviewedAt changed after overwrite")
	}
	if firstRead.Outcome != secondRead.Outcome {
		t.Errorf("Outcome changed after overwrite")
	}
	if firstRead.Summary != secondRead.Summary {
		t.Errorf("Summary changed after overwrite")
	}
	if len(firstRead.ManualResults) != len(secondRead.ManualResults) {
		t.Errorf("ManualResults count changed after overwrite")
	}
}

func TestWriteOutcomeCreatesEvidenceDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, "new", "evidence", "dir")

	outcome := &ReviewOutcome{
		RunID:   "dir-creation-test",
		Outcome: OutcomeAccepted,
		Summary: "Test directory creation",
	}

	// Directory should not exist before write
	if _, err := os.Stat(evidenceDir); err == nil {
		t.Error("Directory should not exist before WriteOutcome")
	}

	if err := WriteOutcome(evidenceDir, outcome); err != nil {
		t.Fatalf("WriteOutcome failed: %v", err)
	}

	// Directory should exist after write
	if info, err := os.Stat(evidenceDir); err != nil || !info.IsDir() {
		t.Error("Directory was not created by WriteOutcome")
	}

	// File should exist
	filePath := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(filePath); err != nil {
		t.Error("review-outcome.json was not created")
	}
}

func TestWriteOutcomeNilError(t *testing.T) {
	tmpDir := t.TempDir()

	err := WriteOutcome(tmpDir, nil)
	if err == nil {
		t.Error("WriteOutcome should return error for nil outcome")
	}
}
