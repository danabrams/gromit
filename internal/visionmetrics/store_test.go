package visionmetrics

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

func TestAppendRecord_AppendsMultipleRecords(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "records.jsonl")

	// Append multiple records sequentially
	records := []Record{
		{
			SpecID:                     "spec-1",
			CycleStartTriggerAt:        parseTime("2026-02-01T08:00:00Z"),
			CycleEndPresentedAt:        parseTime("2026-02-01T10:00:00Z"),
			ReviewOutcome:              ReviewOutcomeAccepted,
			HumanTacticalIntervention:  Yes,
			HumanDebuggingIntervention: No,
			EscapedRegressionWithin7D:  No,
		},
		{
			SpecID:                     "spec-2",
			CycleStartTriggerAt:        parseTime("2026-02-02T09:00:00Z"),
			CycleEndPresentedAt:        parseTime("2026-02-02T11:00:00Z"),
			ReviewOutcome:              ReviewOutcomeImplementationGap,
			HumanTacticalIntervention:  No,
			HumanDebuggingIntervention: Yes,
			EscapedRegressionWithin7D:  No,
		},
		{
			SpecID:                     "spec-3",
			CycleStartTriggerAt:        parseTime("2026-02-03T07:00:00Z"),
			CycleEndPresentedAt:        parseTime("2026-02-03T09:00:00Z"),
			ReviewOutcome:              ReviewOutcomeVisionChange,
			ReviewRationale:            "Strategy adjusted",
			HumanTacticalIntervention:  No,
			HumanDebuggingIntervention: No,
			EscapedRegressionWithin7D:  Yes,
		},
	}

	for _, record := range records {
		if err := AppendRecord(tmpFile, record); err != nil {
			t.Fatalf("AppendRecord failed: %v", err)
		}
	}

	// Read all records back
	loaded, err := LoadRecords(tmpFile)
	if err != nil {
		t.Fatalf("LoadRecords failed: %v", err)
	}

	// Verify we got all 3 records
	if len(loaded) != len(records) {
		t.Errorf("expected %d records, got %d", len(records), len(loaded))
	}

	// Verify each record matches
	for i := range records {
		if loaded[i].SpecID != records[i].SpecID {
			t.Errorf("record %d SpecID mismatch: got %q, want %q", i, loaded[i].SpecID, records[i].SpecID)
		}
		if loaded[i].ReviewOutcome != records[i].ReviewOutcome {
			t.Errorf("record %d ReviewOutcome mismatch: got %q, want %q", i, loaded[i].ReviewOutcome, records[i].ReviewOutcome)
		}
		if loaded[i].ReviewRationale != records[i].ReviewRationale {
			t.Errorf("record %d ReviewRationale mismatch: got %q, want %q", i, loaded[i].ReviewRationale, records[i].ReviewRationale)
		}
	}
}

func TestAppendRecord_CreatesFileIfNotExists(t *testing.T) {
	// Create a temporary directory but don't create the file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "nonexistent", "records.jsonl")

	// Create the directory structure (this is what we're testing - that AppendRecord will create the file)
	if err := os.MkdirAll(filepath.Dir(tmpFile), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Append a record to a non-existent file
	record := Record{
		SpecID:                     "new-spec",
		CycleStartTriggerAt:        parseTime("2026-02-01T08:00:00Z"),
		CycleEndPresentedAt:        parseTime("2026-02-01T10:00:00Z"),
		ReviewOutcome:              ReviewOutcomeAccepted,
		HumanTacticalIntervention:  No,
		HumanDebuggingIntervention: No,
		EscapedRegressionWithin7D:  No,
	}

	if err := AppendRecord(tmpFile, record); err != nil {
		t.Fatalf("AppendRecord failed: %v", err)
	}

	// Verify file exists and can be read
	loaded, err := LoadRecords(tmpFile)
	if err != nil {
		t.Fatalf("LoadRecords failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Errorf("expected 1 record, got %d", len(loaded))
	}
	if loaded[0].SpecID != "new-spec" {
		t.Errorf("SpecID mismatch: got %q, want %q", loaded[0].SpecID, "new-spec")
	}
}

func TestLoadRecords_EmptyFile(t *testing.T) {
	// Create an empty temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "empty.jsonl")

	if err := os.WriteFile(tmpFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Load from empty file
	records, err := LoadRecords(tmpFile)
	if err != nil {
		t.Fatalf("LoadRecords failed: %v", err)
	}

	// Should return empty slice, not nil
	if records == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestLoadRecords_NonExistentFile(t *testing.T) {
	// Try to load from a non-existent file
	_, err := LoadRecords("/nonexistent/path/to/file.jsonl")
	if err == nil {
		t.Error("LoadRecords should return error for non-existent file")
	}
}

func TestAppendRecord_PreservesTimestamps(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "timestamps.jsonl")

	// Create a record with specific timestamps
	startTime := parseTime("2026-02-15T14:30:45Z")
	endTime := parseTime("2026-02-15T16:45:30Z")

	record := Record{
		SpecID:                     "timestamp-test",
		CycleStartTriggerAt:        startTime,
		CycleEndPresentedAt:        endTime,
		ReviewOutcome:              ReviewOutcomeAccepted,
		HumanTacticalIntervention:  No,
		HumanDebuggingIntervention: No,
		EscapedRegressionWithin7D:  No,
	}

	if err := AppendRecord(tmpFile, record); err != nil {
		t.Fatalf("AppendRecord failed: %v", err)
	}

	// Read back and verify timestamps are preserved
	loaded, err := LoadRecords(tmpFile)
	if err != nil {
		t.Fatalf("LoadRecords failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("expected 1 record, got %d", len(loaded))
	}

	// Verify timestamps match exactly (including nanosecond precision)
	if !loaded[0].CycleStartTriggerAt.Equal(startTime) {
		t.Errorf("CycleStartTriggerAt mismatch: got %v, want %v", loaded[0].CycleStartTriggerAt, startTime)
	}
	if !loaded[0].CycleEndPresentedAt.Equal(endTime) {
		t.Errorf("CycleEndPresentedAt mismatch: got %v, want %v", loaded[0].CycleEndPresentedAt, endTime)
	}
}

func TestAppendRecord_PreservesOptionalFields(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "optional.jsonl")

	// Create a record with optional ReviewRationale
	record := Record{
		SpecID:                     "rationale-test",
		CycleStartTriggerAt:        parseTime("2026-02-01T08:00:00Z"),
		CycleEndPresentedAt:        parseTime("2026-02-01T10:00:00Z"),
		ReviewOutcome:              ReviewOutcomeVisionChange,
		ReviewRationale:            "Business priorities shifted due to market analysis",
		HumanTacticalIntervention:  Yes,
		HumanDebuggingIntervention: No,
		EscapedRegressionWithin7D:  No,
	}

	if err := AppendRecord(tmpFile, record); err != nil {
		t.Fatalf("AppendRecord failed: %v", err)
	}

	// Read back and verify optional fields are preserved
	loaded, err := LoadRecords(tmpFile)
	if err != nil {
		t.Fatalf("LoadRecords failed: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("expected 1 record, got %d", len(loaded))
	}

	if loaded[0].ReviewRationale != record.ReviewRationale {
		t.Errorf("ReviewRationale mismatch: got %q, want %q", loaded[0].ReviewRationale, record.ReviewRationale)
	}

	// Create and append record without ReviewRationale
	recordNoRationale := Record{
		SpecID:                     "no-rationale-test",
		CycleStartTriggerAt:        parseTime("2026-02-02T08:00:00Z"),
		CycleEndPresentedAt:        parseTime("2026-02-02T10:00:00Z"),
		ReviewOutcome:              ReviewOutcomeAccepted,
		HumanTacticalIntervention:  No,
		HumanDebuggingIntervention: No,
		EscapedRegressionWithin7D:  No,
	}

	if err := AppendRecord(tmpFile, recordNoRationale); err != nil {
		t.Fatalf("AppendRecord failed: %v", err)
	}

	// Reload and verify both records
	loaded, err = LoadRecords(tmpFile)
	if err != nil {
		t.Fatalf("LoadRecords failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 records, got %d", len(loaded))
	}

	// Verify first record has rationale
	if loaded[0].ReviewRationale != "Business priorities shifted due to market analysis" {
		t.Errorf("first record ReviewRationale mismatch: got %q", loaded[0].ReviewRationale)
	}

	// Verify second record has no rationale
	if loaded[1].ReviewRationale != "" {
		t.Errorf("second record ReviewRationale should be empty, got %q", loaded[1].ReviewRationale)
	}
}

func TestAppendRecord_PreservesAllYesNoValues(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "yesno.jsonl")

	// Test all combinations of YesNo values
	testCases := []struct {
		tactical   YesNo
		debugging  YesNo
		regression YesNo
	}{
		{Yes, Yes, Yes},
		{Yes, Yes, No},
		{Yes, No, Yes},
		{Yes, No, No},
		{No, Yes, Yes},
		{No, Yes, No},
		{No, No, Yes},
		{No, No, No},
	}

	for i, tc := range testCases {
		record := Record{
			SpecID:                     "test-" + string(rune(i)),
			CycleStartTriggerAt:        parseTime("2026-02-01T08:00:00Z"),
			CycleEndPresentedAt:        parseTime("2026-02-01T10:00:00Z"),
			ReviewOutcome:              ReviewOutcomeAccepted,
			HumanTacticalIntervention:  tc.tactical,
			HumanDebuggingIntervention: tc.debugging,
			EscapedRegressionWithin7D:  tc.regression,
		}

		if err := AppendRecord(tmpFile, record); err != nil {
			t.Fatalf("AppendRecord failed at case %d: %v", i, err)
		}
	}

	// Read all records back
	loaded, err := LoadRecords(tmpFile)
	if err != nil {
		t.Fatalf("LoadRecords failed: %v", err)
	}

	if len(loaded) != len(testCases) {
		t.Fatalf("expected %d records, got %d", len(testCases), len(loaded))
	}

	// Verify each combination is preserved
	for i, tc := range testCases {
		if loaded[i].HumanTacticalIntervention != tc.tactical {
			t.Errorf("record %d tactical mismatch: got %q, want %q", i, loaded[i].HumanTacticalIntervention, tc.tactical)
		}
		if loaded[i].HumanDebuggingIntervention != tc.debugging {
			t.Errorf("record %d debugging mismatch: got %q, want %q", i, loaded[i].HumanDebuggingIntervention, tc.debugging)
		}
		if loaded[i].EscapedRegressionWithin7D != tc.regression {
			t.Errorf("record %d regression mismatch: got %q, want %q", i, loaded[i].EscapedRegressionWithin7D, tc.regression)
		}
	}
}

func TestAppendRecord_PreservesAllReviewOutcomes(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "outcomes.jsonl")

	// Test all ReviewOutcome values
	testCases := []struct {
		outcome   ReviewOutcome
		rationale string
	}{
		{ReviewOutcomeAccepted, ""},
		{ReviewOutcomeImplementationGap, ""},
		{ReviewOutcomeVisionChange, "Scope has changed"},
	}

	for _, tc := range testCases {
		record := Record{
			SpecID:                     "outcome-" + string(tc.outcome),
			CycleStartTriggerAt:        parseTime("2026-02-01T08:00:00Z"),
			CycleEndPresentedAt:        parseTime("2026-02-01T10:00:00Z"),
			ReviewOutcome:              tc.outcome,
			ReviewRationale:            tc.rationale,
			HumanTacticalIntervention:  No,
			HumanDebuggingIntervention: No,
			EscapedRegressionWithin7D:  No,
		}

		if err := AppendRecord(tmpFile, record); err != nil {
			t.Fatalf("AppendRecord failed for %q: %v", tc.outcome, err)
		}
	}

	// Read all records back
	loaded, err := LoadRecords(tmpFile)
	if err != nil {
		t.Fatalf("LoadRecords failed: %v", err)
	}

	if len(loaded) != len(testCases) {
		t.Fatalf("expected %d records, got %d", len(testCases), len(loaded))
	}

	// Verify each outcome is preserved
	for i, tc := range testCases {
		if loaded[i].ReviewOutcome != tc.outcome {
			t.Errorf("record %d ReviewOutcome mismatch: got %q, want %q", i, loaded[i].ReviewOutcome, tc.outcome)
		}
		if loaded[i].ReviewRationale != tc.rationale {
			t.Errorf("record %d ReviewRationale mismatch: got %q, want %q", i, loaded[i].ReviewRationale, tc.rationale)
		}
	}
}

func TestAppendRecord_ErrorOnNonExistentDirectory(t *testing.T) {
	// Try to append to a file in a non-existent directory
	record := Record{
		SpecID:                     "test",
		CycleStartTriggerAt:        parseTime("2026-02-01T08:00:00Z"),
		CycleEndPresentedAt:        parseTime("2026-02-01T10:00:00Z"),
		ReviewOutcome:              ReviewOutcomeAccepted,
		HumanTacticalIntervention:  No,
		HumanDebuggingIntervention: No,
		EscapedRegressionWithin7D:  No,
	}

	// This should fail because the directory doesn't exist
	err := AppendRecord("/nonexistent/path/to/file.jsonl", record)
	if err == nil {
		t.Error("AppendRecord should return error for non-existent directory")
	}
}

func TestAppendRecord_AtomicWriteUnderConcurrentAccess(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "concurrent.jsonl")

	// Number of concurrent goroutines
	const numGoroutines = 10
	const recordsPerGoroutine = 5

	// Create a wait group to coordinate goroutines
	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)

	// Launch concurrent appends
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < recordsPerGoroutine; i++ {
				record := Record{
					SpecID:                     fmt.Sprintf("goroutine-%d-record-%d", goroutineID, i),
					CycleStartTriggerAt:        parseTime("2026-02-01T08:00:00Z"),
					CycleEndPresentedAt:        parseTime("2026-02-01T10:00:00Z"),
					ReviewOutcome:              ReviewOutcomeAccepted,
					HumanTacticalIntervention:  No,
					HumanDebuggingIntervention: No,
					EscapedRegressionWithin7D:  No,
				}
				if err := AppendRecord(tmpFile, record); err != nil {
					errChan <- err
					return
				}
			}
		}(g)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			t.Fatalf("AppendRecord failed: %v", err)
		}
	}

	// Read the file back and verify no interleaving occurred
	// If lines were interleaved, LoadRecords would fail with JSON parse errors
	loaded, err := LoadRecords(tmpFile)
	if err != nil {
		t.Fatalf("LoadRecords failed (possible interleaving): %v", err)
	}

	// Verify we got all records
	expectedCount := numGoroutines * recordsPerGoroutine
	if len(loaded) != expectedCount {
		t.Errorf("expected %d records, got %d", expectedCount, len(loaded))
	}

	// Verify all SpecIDs are properly formatted (would be corrupted if interleaved)
	seenIDs := make(map[string]bool)
	for _, record := range loaded {
		if seenIDs[record.SpecID] {
			t.Errorf("duplicate SpecID: %q", record.SpecID)
		}
		seenIDs[record.SpecID] = true
	}
}

func parseTime(s string) time.Time {
	// Helper to parse RFC3339 timestamp
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
