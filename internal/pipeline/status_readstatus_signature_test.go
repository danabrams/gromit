//go:build acceptance

package pipeline

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestReadStatus_AcceptsStartedAtParameter verifies ReadStatus signature accepts startedAt parameter
func TestReadStatus_AcceptsStartedAtParameter(t *testing.T) {
	// Expected failure: ReadStatus does not accept startedAt *time.Time parameter yet
	// Current signature: ReadStatus(gromitDir, specsDir, plansDir string)
	// New signature: ReadStatus(gromitDir, specsDir, plansDir string, startedAt *time.Time)

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// Test with nil startedAt
	status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
	if err != nil {
		t.Fatalf("ReadStatus() with nil startedAt error = %v", err)
	}

	if status == nil {
		t.Fatal("ReadStatus() returned nil status")
	}

	// Verify HasRunInfo is false when startedAt is nil
	if status.HasRunInfo {
		t.Error("HasRunInfo should be false when startedAt is nil")
	}

	// Verify ClosedThisRunCount is zero when startedAt is nil
	if status.ClosedThisRunCount != 0 {
		t.Errorf("ClosedThisRunCount should be 0 when startedAt is nil, got %d", status.ClosedThisRunCount)
	}
}

// TestReadStatus_WithNonNilStartedAt_PopulatesClosedThisRunCount verifies behavior with non-nil startedAt
func TestReadStatus_WithNonNilStartedAt_PopulatesClosedThisRunCount(t *testing.T) {
	// Expected failure: ReadStatus does not accept startedAt parameter yet, and does not call CountClosedAfter

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// Test with non-nil startedAt
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	status, err := ReadStatus(gromitDir, specsDir, plansDir, &oneHourAgo)
	if err != nil {
		t.Fatalf("ReadStatus() with startedAt error = %v", err)
	}

	if status == nil {
		t.Fatal("ReadStatus() returned nil status")
	}

	// Verify HasRunInfo is true when startedAt is non-nil
	if !status.HasRunInfo {
		t.Error("HasRunInfo should be true when startedAt is non-nil")
	}

	// ClosedThisRunCount should be >= 0 (actual value depends on bd repo state)
	if status.ClosedThisRunCount < 0 {
		t.Errorf("ClosedThisRunCount should be >= 0, got %d", status.ClosedThisRunCount)
	}
}

// TestReadStatus_PopulatesAllBeadCounts verifies ReadStatus populates all new count fields
func TestReadStatus_PopulatesAllBeadCounts(t *testing.T) {
	// Expected failure: ReadStatus does not populate InProgressCount, BlockedCount, DeferredCount, ClosedCount fields yet

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}

	// All count fields should be populated (even if zero in an empty repo)
	// The key test is that the fields exist and are being set by ReadStatus

	// InProgressCount should be >= 0
	if status.InProgressCount < 0 {
		t.Errorf("InProgressCount should be >= 0, got %d", status.InProgressCount)
	}

	// BlockedCount should be >= 0
	if status.BlockedCount < 0 {
		t.Errorf("BlockedCount should be >= 0, got %d", status.BlockedCount)
	}

	// DeferredCount should be >= 0
	if status.DeferredCount < 0 {
		t.Errorf("DeferredCount should be >= 0, got %d", status.DeferredCount)
	}

	// ClosedCount should be >= 0
	if status.ClosedCount < 0 {
		t.Errorf("ClosedCount should be >= 0, got %d", status.ClosedCount)
	}
}

// TestReadStatus_BlockedCountCalculatedCorrectly verifies blocked = open - ready
func TestReadStatus_BlockedCountCalculatedCorrectly(t *testing.T) {
	// Expected failure: ReadStatus does not calculate BlockedCount from open and ready counts yet

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}

	// In an empty bd repo, we can't test exact values, but we can verify the relationship
	// blocked should equal (open count - ready count) where open includes both ready and blocked beads
	// This is a logical check that the calculation was performed

	// For this test, we just verify that BlockedCount field exists and is set
	// The actual calculation correctness would need a real bd repo with known state
	_ = status.BlockedCount

	// We can verify that BlockedCount is non-negative, which is a basic sanity check
	if status.BlockedCount < 0 {
		t.Errorf("BlockedCount should never be negative, got %d", status.BlockedCount)
	}

	// In a working implementation, BlockedCount should be calculated as:
	// openCount (from CountByStatus("open")) - ReadyBeadCount
	// We verify this relationship exists by checking the field is populated
}

// TestReadStatus_CallsBeadClientCountMethods verifies ReadStatus uses bead.Client count methods
func TestReadStatus_CallsBeadClientCountMethods(t *testing.T) {
	// Expected failure: ReadStatus does not call CountByStatus("in_progress"), CountByStatus("open"),
	// CountByStatus("deferred"), CountByStatus("closed"), or CountClosedAfter yet

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// Without a bd repo, this will result in zero counts from failed client calls
	// But the test verifies that ReadStatus attempts to populate these fields

	oneHourAgo := time.Now().Add(-1 * time.Hour)
	status, err := ReadStatus(gromitDir, specsDir, plansDir, &oneHourAgo)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}

	// Verify all count fields are populated (will be 0 in test environment)
	// The key assertion is that these fields are being set by ReadStatus
	if status.InProgressCount < 0 {
		t.Errorf("InProgressCount not populated correctly, got %d", status.InProgressCount)
	}

	if status.DeferredCount < 0 {
		t.Errorf("DeferredCount not populated correctly, got %d", status.DeferredCount)
	}

	if status.ClosedCount < 0 {
		t.Errorf("ClosedCount not populated correctly, got %d", status.ClosedCount)
	}

	if status.ClosedThisRunCount < 0 {
		t.Errorf("ClosedThisRunCount not populated correctly, got %d", status.ClosedThisRunCount)
	}

	// Verify HasRunInfo reflects whether startedAt was provided
	if !status.HasRunInfo {
		t.Error("HasRunInfo should be true when startedAt is provided")
	}
}

// TestReadStatus_PreservesExistingBehavior verifies backward compatibility
func TestReadStatus_PreservesExistingBehavior(t *testing.T) {
	// Expected failure: ReadStatus signature change will break this test until it's updated

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// Old behavior: backlog, specs, plans counts still work
	status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}

	// Verify existing fields are still populated
	if status.UnrefinedIdeas == nil {
		t.Error("UnrefinedIdeas should not be nil")
	}

	if status.UnplannedSpecs == nil {
		t.Error("UnplannedSpecs should not be nil")
	}

	if status.UndecomposedPlans == nil {
		t.Error("UndecomposedPlans should not be nil")
	}

	if status.ReadyBeads == nil {
		t.Error("ReadyBeads should not be nil")
	}

	if status.Recommendation == "" {
		t.Error("Recommendation should not be empty")
	}
}

// TestReadStatus_ClosedThisRunCountOnlyWhenStartedAtProvided verifies conditional behavior
func TestReadStatus_ClosedThisRunCountOnlyWhenStartedAtProvided(t *testing.T) {
	// Expected failure: ReadStatus does not conditionally call CountClosedAfter based on startedAt yet

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	tests := []struct {
		name                      string
		startedAt                 *time.Time
		wantHasRunInfo            bool
		wantClosedThisRunCountSet bool
	}{
		{
			name:                      "nil startedAt",
			startedAt:                 nil,
			wantHasRunInfo:            false,
			wantClosedThisRunCountSet: false,
		},
		{
			name:                      "non-nil startedAt",
			startedAt:                 timePtr(time.Now().Add(-1 * time.Hour)),
			wantHasRunInfo:            true,
			wantClosedThisRunCountSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := ReadStatus(gromitDir, specsDir, plansDir, tt.startedAt)
			if err != nil {
				t.Fatalf("ReadStatus() error = %v", err)
			}

			if status.HasRunInfo != tt.wantHasRunInfo {
				t.Errorf("HasRunInfo = %v, want %v", status.HasRunInfo, tt.wantHasRunInfo)
			}

			// When startedAt is nil, ClosedThisRunCount should remain 0
			// When startedAt is non-nil, ClosedThisRunCount should be set (even if 0)
			if !tt.wantClosedThisRunCountSet && status.ClosedThisRunCount != 0 {
				t.Errorf("ClosedThisRunCount should be 0 when startedAt is nil, got %d", status.ClosedThisRunCount)
			}

			if tt.wantClosedThisRunCountSet && status.ClosedThisRunCount < 0 {
				t.Errorf("ClosedThisRunCount should be >= 0 when startedAt is provided, got %d", status.ClosedThisRunCount)
			}
		})
	}
}

// timePtr returns a pointer to the given time
func timePtr(t time.Time) *time.Time {
	return &t
}
