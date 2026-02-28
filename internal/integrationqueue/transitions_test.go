package integrationqueue

import (
	"testing"
	"time"
)

func TestCanTransition_DraftToReady_Allowed(t *testing.T) {
	result := CanTransition("draft", "ready")
	if !result {
		t.Errorf("expected draft->ready to be allowed, got false")
	}
}

func TestCanTransition_ReadyToIntegrating_Allowed(t *testing.T) {
	result := CanTransition("ready", "integrating")
	if !result {
		t.Errorf("expected ready->integrating to be allowed, got false")
	}
}

func TestCanTransition_IntegratingToMerged_Allowed(t *testing.T) {
	result := CanTransition("integrating", "merged")
	if !result {
		t.Errorf("expected integrating->merged to be allowed, got false")
	}
}

func TestCanTransition_IntegratingToConflict_Allowed(t *testing.T) {
	result := CanTransition("integrating", "conflict")
	if !result {
		t.Errorf("expected integrating->conflict to be allowed, got false")
	}
}

func TestCanTransition_DraftToIntegrating_Disallowed(t *testing.T) {
	result := CanTransition("draft", "integrating")
	if result {
		t.Errorf("expected draft->integrating to be disallowed, got true")
	}
}

func TestRecordTransition_CapturesReasonAndErrorCode(t *testing.T) {
	before := time.Now()
	record := RecordTransition("ready", "integrating", "automatic dequeue", "")
	after := time.Now()

	if record.FromState != "ready" {
		t.Errorf("expected FromState to be ready, got %s", record.FromState)
	}
	if record.ToState != "integrating" {
		t.Errorf("expected ToState to be integrating, got %s", record.ToState)
	}
	if record.Reason != "automatic dequeue" {
		t.Errorf("expected Reason to be 'automatic dequeue', got %s", record.Reason)
	}
	if record.ErrorCode != "" {
		t.Errorf("expected ErrorCode to be empty, got %s", record.ErrorCode)
	}
	if record.Timestamp.Before(before) || record.Timestamp.After(after) {
		t.Errorf("expected Timestamp to be set to now, got %v", record.Timestamp)
	}
}
