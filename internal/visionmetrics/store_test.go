package visionmetrics

import (
	"os"
	"path/filepath"
	"testing"
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
