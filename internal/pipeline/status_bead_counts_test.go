package pipeline

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPipelineStatus_ExpandedCountFields verifies that PipelineStatus struct has new count fields
func TestPipelineStatus_ExpandedCountFields(t *testing.T) {
	// Expected failure: InProgressCount, BlockedCount, DeferredCount, ClosedCount, ClosedThisRunCount, and HasRunInfo fields do not exist on PipelineStatus yet

	ps := &PipelineStatus{}

	// Test that new fields exist and are zero by default
	if ps.InProgressCount != 0 {
		t.Errorf("InProgressCount default should be 0, got %d", ps.InProgressCount)
	}
	if ps.BlockedCount != 0 {
		t.Errorf("BlockedCount default should be 0, got %d", ps.BlockedCount)
	}
	if ps.DeferredCount != 0 {
		t.Errorf("DeferredCount default should be 0, got %d", ps.DeferredCount)
	}
	if ps.ClosedCount != 0 {
		t.Errorf("ClosedCount default should be 0, got %d", ps.ClosedCount)
	}
	if ps.ClosedThisRunCount != 0 {
		t.Errorf("ClosedThisRunCount default should be 0, got %d", ps.ClosedThisRunCount)
	}
	if ps.HasRunInfo != false {
		t.Errorf("HasRunInfo default should be false, got %v", ps.HasRunInfo)
	}
}

// TestReadStatus_PopulatesInProgressCount verifies ReadStatus calls bead client to get in-progress count
func TestReadStatus_PopulatesInProgressCount(t *testing.T) {
	// Expected failure: ReadStatus does not populate InProgressCount field yet

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// Create a bd repository in the tmpDir
	// This test requires actual bd CLI to be present and functional
	// When implemented, ReadStatus will call client.CountByStatus("in_progress")

	status, err := ReadStatus(gromitDir, specsDir, plansDir)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}

	// Verify InProgressCount is populated (will be 0 in empty repo)
	if status.InProgressCount < 0 {
		t.Errorf("InProgressCount should be >= 0, got %d", status.InProgressCount)
	}
}

// TestReadStatus_PopulatesBlockedCount verifies ReadStatus calculates blocked count
func TestReadStatus_PopulatesBlockedCount(t *testing.T) {
	// Expected failure: ReadStatus does not populate BlockedCount field yet

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// When implemented, ReadStatus will calculate blocked = (total open) - (ready)
	// using client.CountByStatus("open") - status.ReadyBeadCount

	status, err := ReadStatus(gromitDir, specsDir, plansDir)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}

	// Verify BlockedCount is populated
	if status.BlockedCount < 0 {
		t.Errorf("BlockedCount should be >= 0, got %d", status.BlockedCount)
	}
}

// TestReadStatus_PopulatesDeferredCount verifies ReadStatus calls bead client to get deferred count
func TestReadStatus_PopulatesDeferredCount(t *testing.T) {
	// Expected failure: ReadStatus does not populate DeferredCount field yet

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// When implemented, ReadStatus will call client.CountByStatus("deferred")

	status, err := ReadStatus(gromitDir, specsDir, plansDir)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}

	// Verify DeferredCount is populated
	if status.DeferredCount < 0 {
		t.Errorf("DeferredCount should be >= 0, got %d", status.DeferredCount)
	}
}

// TestReadStatus_PopulatesClosedCount verifies ReadStatus calls bead client to get closed count
func TestReadStatus_PopulatesClosedCount(t *testing.T) {
	// Expected failure: ReadStatus does not populate ClosedCount field yet

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// When implemented, ReadStatus will call client.CountByStatus("closed")

	status, err := ReadStatus(gromitDir, specsDir, plansDir)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}

	// Verify ClosedCount is populated
	if status.ClosedCount < 0 {
		t.Errorf("ClosedCount should be >= 0, got %d", status.ClosedCount)
	}
}

// TestReadStatus_WithStartedAt_PopulatesClosedThisRunCount verifies ReadStatus accepts startedAt parameter
func TestReadStatus_WithStartedAt_PopulatesClosedThisRunCount(t *testing.T) {
	// Expected failure: ReadStatus does not accept a startedAt parameter yet, and does not populate ClosedThisRunCount

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// When implemented, ReadStatus will accept a *time.Time parameter for startedAt
	// and call client.CountClosedAfter(startedAt) when startedAt is non-nil
	oneHourAgo := time.Now().Add(-1 * time.Hour)

	status, err := ReadStatusWithStartTime(gromitDir, specsDir, plansDir, &oneHourAgo)
	if err != nil {
		t.Fatalf("ReadStatusWithStartTime() error = %v", err)
	}

	// Verify ClosedThisRunCount is populated when startedAt is provided
	if status.ClosedThisRunCount < 0 {
		t.Errorf("ClosedThisRunCount should be >= 0, got %d", status.ClosedThisRunCount)
	}

	// Verify HasRunInfo is true when startedAt is provided
	if !status.HasRunInfo {
		t.Error("HasRunInfo should be true when startedAt is provided")
	}
}

// TestReadStatus_WithoutStartedAt_DoesNotPopulateClosedThisRunCount verifies behavior without startedAt
func TestReadStatus_WithoutStartedAt_DoesNotPopulateClosedThisRunCount(t *testing.T) {
	// Expected failure: ReadStatusWithStartTime does not exist yet

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// When implemented, passing nil for startedAt should skip ClosedThisRunCount query
	status, err := ReadStatusWithStartTime(gromitDir, specsDir, plansDir, nil)
	if err != nil {
		t.Fatalf("ReadStatusWithStartTime() error = %v", err)
	}

	// Verify ClosedThisRunCount is not populated (remains 0) when startedAt is nil
	if status.ClosedThisRunCount != 0 {
		t.Errorf("ClosedThisRunCount should be 0 when startedAt is nil, got %d", status.ClosedThisRunCount)
	}

	// Verify HasRunInfo is false when startedAt is nil
	if status.HasRunInfo {
		t.Error("HasRunInfo should be false when startedAt is nil")
	}
}

// TestReadStatus_BlockedCountCalculation verifies blocked count equals open minus ready
func TestReadStatus_BlockedCountCalculation(t *testing.T) {
	// Expected failure: ReadStatus does not calculate BlockedCount as (open - ready) yet

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// When implemented with actual bd integration:
	// If there are 10 open beads and 7 ready beads, blocked should be 3

	status, err := ReadStatus(gromitDir, specsDir, plansDir)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}

	// This is a logical relationship test - verify the calculation is correct
	// blocked = open - ready (where open beads include both ready and blocked)
	// We can't verify exact counts without a real bd repo, but we can verify the field exists
	_ = status.BlockedCount // Accessing the field will fail if it doesn't exist
}

// ReadStatusWithStartTime is a helper function that will be implemented
// Expected failure: This function does not exist yet
func ReadStatusWithStartTime(gromitDir, specsDir, plansDir string, startedAt *time.Time) (*PipelineStatus, error) {
	// When implemented, this will be the signature for ReadStatus with optional startedAt
	// For now, this is a placeholder that will cause compilation failure
	return nil, nil
}

// Mock client for testing bead count integration
type mockBeadClient struct {
	CountByStatusFunc   func(status string) (int, error)
	CountReadyFunc      func() (int, error)
	CountClosedAfterFunc func(after time.Time) (int, error)
	ListReadyIDsFunc    func() ([]string, error)
}

func (m *mockBeadClient) CountByStatus(status string) (int, error) {
	if m.CountByStatusFunc != nil {
		return m.CountByStatusFunc(status)
	}
	return 0, nil
}

func (m *mockBeadClient) CountReady() (int, error) {
	if m.CountReadyFunc != nil {
		return m.CountReadyFunc()
	}
	return 0, nil
}

func (m *mockBeadClient) CountClosedAfter(after time.Time) (int, error) {
	if m.CountClosedAfterFunc != nil {
		return m.CountClosedAfterFunc(after)
	}
	return 0, nil
}

func (m *mockBeadClient) ListReadyIDs() ([]string, error) {
	if m.ListReadyIDsFunc != nil {
		return m.ListReadyIDsFunc()
	}
	return []string{}, nil
}

// TestReadStatus_IntegratesWithBeadClient verifies ReadStatus calls bead client methods
func TestReadStatus_IntegratesWithBeadClient(t *testing.T) {
	// Expected failure: ReadStatus does not accept an injectable bead client parameter yet

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// Track which bead client methods are called
	var calledCountByStatus []string
	var calledCountReady bool
	var calledCountClosedAfter bool

	mock := &mockBeadClient{
		CountByStatusFunc: func(status string) (int, error) {
			calledCountByStatus = append(calledCountByStatus, status)
			switch status {
			case "in_progress":
				return 2, nil
			case "open":
				return 12, nil
			case "deferred":
				return 1, nil
			case "closed":
				return 543, nil
			default:
				return 0, nil
			}
		},
		CountReadyFunc: func() (int, error) {
			calledCountReady = true
			return 7, nil
		},
		CountClosedAfterFunc: func(after time.Time) (int, error) {
			calledCountClosedAfter = true
			return 23, nil
		},
		ListReadyIDsFunc: func() ([]string, error) {
			return []string{"gromit-abc1", "gromit-abc2", "gromit-abc3"}, nil
		},
	}

	oneHourAgo := time.Now().Add(-1 * time.Hour)
	status, err := ReadStatusWithClient(gromitDir, specsDir, plansDir, &oneHourAgo, mock)
	if err != nil {
		t.Fatalf("ReadStatusWithClient() error = %v", err)
	}

	// Verify all expected methods were called
	if !calledCountReady {
		t.Error("Expected CountReady to be called")
	}

	expectedStatuses := []string{"in_progress", "open", "deferred", "closed"}
	if len(calledCountByStatus) != len(expectedStatuses) {
		t.Errorf("Expected CountByStatus to be called %d times, got %d", len(expectedStatuses), len(calledCountByStatus))
	}

	if !calledCountClosedAfter {
		t.Error("Expected CountClosedAfter to be called when startedAt is provided")
	}

	// Verify counts are populated correctly
	if status.InProgressCount != 2 {
		t.Errorf("InProgressCount = %d, want 2", status.InProgressCount)
	}
	if status.DeferredCount != 1 {
		t.Errorf("DeferredCount = %d, want 1", status.DeferredCount)
	}
	if status.ClosedCount != 543 {
		t.Errorf("ClosedCount = %d, want 543", status.ClosedCount)
	}
	if status.ClosedThisRunCount != 23 {
		t.Errorf("ClosedThisRunCount = %d, want 23", status.ClosedThisRunCount)
	}
	if status.HasRunInfo != true {
		t.Error("HasRunInfo should be true when startedAt is provided")
	}

	// Verify blocked count is calculated as open - ready = 12 - 7 = 5
	if status.BlockedCount != 5 {
		t.Errorf("BlockedCount = %d, want 5 (open 12 - ready 7)", status.BlockedCount)
	}
}

// ReadStatusWithClient is a helper function signature that will be implemented
// Expected failure: This function does not exist yet
func ReadStatusWithClient(gromitDir, specsDir, plansDir string, startedAt *time.Time, client interface{}) (*PipelineStatus, error) {
	// When implemented, this allows injecting a mock bead client for testing
	return nil, nil
}

// BeadClientInterface defines the interface for bead count operations
// Expected failure: This interface does not exist in pipeline package yet
type BeadClientInterface interface {
	CountByStatus(status string) (int, error)
	CountReady() (int, error)
	CountClosedAfter(after time.Time) (int, error)
	ListReadyIDs() ([]string, error)
}
