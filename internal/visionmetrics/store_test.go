package visionmetrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadRecords_ReadsValidJSONLFile(t *testing.T) {
	// Create a temporary file with valid JSONL content
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "records.jsonl")

	// Write sample records
	content := `{"spec_id":"spec-1","cycle_start_trigger_at":"2026-02-01T08:00:00Z","cycle_end_presented_at":"2026-02-01T10:00:00Z","review_outcome":"accepted","human_tactical_intervention":"yes","human_debugging_intervention":"no","escaped_regression_within_7d":"no"}
{"spec_id":"spec-2","cycle_start_trigger_at":"2026-02-02T09:00:00Z","cycle_end_presented_at":"2026-02-02T11:30:00Z","review_outcome":"rework_implementation_gap","human_tactical_intervention":"yes","human_debugging_intervention":"yes","escaped_regression_within_7d":"yes"}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Load records
	records, err := LoadRecords(tmpFile)
	if err != nil {
		t.Fatalf("LoadRecords failed: %v", err)
	}

	// Verify we got 2 records
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}

	// Verify first record
	if records[0].SpecID != "spec-1" {
		t.Errorf("first record SpecID: got %q, want %q", records[0].SpecID, "spec-1")
	}
	if records[0].ReviewOutcome != ReviewOutcomeAccepted {
		t.Errorf("first record ReviewOutcome: got %q, want %q", records[0].ReviewOutcome, ReviewOutcomeAccepted)
	}
	if records[0].HumanTacticalIntervention != Yes {
		t.Errorf("first record HumanTacticalIntervention: got %q, want %q", records[0].HumanTacticalIntervention, Yes)
	}

	// Verify second record
	if records[1].SpecID != "spec-2" {
		t.Errorf("second record SpecID: got %q, want %q", records[1].SpecID, "spec-2")
	}
	if records[1].ReviewOutcome != ReviewOutcomeImplementationGap {
		t.Errorf("second record ReviewOutcome: got %q, want %q", records[1].ReviewOutcome, ReviewOutcomeImplementationGap)
	}
}

func TestLoadRecords_ErrorOnMalformedJSON(t *testing.T) {
	// Create a temporary file with malformed JSON
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "records.jsonl")

	// Write malformed JSON
	content := `{"spec_id":"spec-1","cycle_start_trigger_at":"2026-02-01T08:00:00Z"
this is not valid json at all
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// LoadRecords should return an error
	_, err := LoadRecords(tmpFile)
	if err == nil {
		t.Error("LoadRecords should return error for malformed JSON")
	}
}

func TestAppendRecord_WritesToJSONLFile(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "records.jsonl")

	// Append a single record
	record := Record{
		SpecID:                     "test-spec",
		CycleStartTriggerAt:        parseTime("2026-02-01T08:00:00Z"),
		CycleEndPresentedAt:        parseTime("2026-02-01T10:00:00Z"),
		ReviewOutcome:              ReviewOutcomeAccepted,
		HumanTacticalIntervention:  Yes,
		HumanDebuggingIntervention: No,
		EscapedRegressionWithin7D:  No,
	}

	if err := AppendRecord(tmpFile, record); err != nil {
		t.Fatalf("AppendRecord failed: %v", err)
	}

	// Read the file back
	records, err := LoadRecords(tmpFile)
	if err != nil {
		t.Fatalf("LoadRecords failed: %v", err)
	}

	// Verify we got 1 record
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}

	// Verify the record matches
	loaded := records[0]
	if loaded.SpecID != record.SpecID {
		t.Errorf("SpecID mismatch: got %q, want %q", loaded.SpecID, record.SpecID)
	}
	if loaded.ReviewOutcome != record.ReviewOutcome {
		t.Errorf("ReviewOutcome mismatch: got %q, want %q", loaded.ReviewOutcome, record.ReviewOutcome)
	}
	if loaded.HumanTacticalIntervention != record.HumanTacticalIntervention {
		t.Errorf("HumanTacticalIntervention mismatch: got %q, want %q", loaded.HumanTacticalIntervention, record.HumanTacticalIntervention)
	}
}

func parseTime(s string) time.Time {
	// Helper to parse RFC3339 timestamp
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
