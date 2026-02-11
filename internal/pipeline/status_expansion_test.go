package pipeline

import (
	"testing"
)

func TestPipelineStatus_HasCountFields(t *testing.T) {
	// Verify that PipelineStatus struct has the new count fields
	status := &PipelineStatus{
		InProgressCount:    5,
		BlockedCount:       3,
		DeferredCount:      2,
		ClosedCount:        100,
		ClosedThisRunCount: 10,
		HasRunInfo:         true,
	}

	if status.InProgressCount != 5 {
		t.Errorf("InProgressCount = %d, want 5", status.InProgressCount)
	}

	if status.BlockedCount != 3 {
		t.Errorf("BlockedCount = %d, want 3", status.BlockedCount)
	}

	if status.DeferredCount != 2 {
		t.Errorf("DeferredCount = %d, want 2", status.DeferredCount)
	}

	if status.ClosedCount != 100 {
		t.Errorf("ClosedCount = %d, want 100", status.ClosedCount)
	}

	if status.ClosedThisRunCount != 10 {
		t.Errorf("ClosedThisRunCount = %d, want 10", status.ClosedThisRunCount)
	}

	if !status.HasRunInfo {
		t.Error("HasRunInfo = false, want true")
	}
}
