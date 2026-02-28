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

func TestRecordTransition_CapturesErrorCode(t *testing.T) {
	record := RecordTransition("integrating", "conflict", "merge conflict detected", "CONFLICT_DETECTED")

	if record.FromState != "integrating" {
		t.Errorf("expected FromState to be integrating, got %s", record.FromState)
	}
	if record.ToState != "conflict" {
		t.Errorf("expected ToState to be conflict, got %s", record.ToState)
	}
	if record.Reason != "merge conflict detected" {
		t.Errorf("expected Reason to be 'merge conflict detected', got %s", record.Reason)
	}
	if record.ErrorCode != "CONFLICT_DETECTED" {
		t.Errorf("expected ErrorCode to be CONFLICT_DETECTED, got %s", record.ErrorCode)
	}
}

func TestCanTransition_IntegratingToFailedGates_Allowed(t *testing.T) {
	result := CanTransition("integrating", "failed_gates")
	if !result {
		t.Errorf("expected integrating->failed_gates to be allowed, got false")
	}
}

func TestCanTransition_IntegratingToLaneViolation_Allowed(t *testing.T) {
	result := CanTransition("integrating", "lane_violation")
	if !result {
		t.Errorf("expected integrating->lane_violation to be allowed, got false")
	}
}

func TestCanTransition_ConflictToReady_Allowed(t *testing.T) {
	result := CanTransition("conflict", "ready")
	if !result {
		t.Errorf("expected conflict->ready to be allowed, got false")
	}
}

func TestCanTransition_FailedGatesToReady_Allowed(t *testing.T) {
	result := CanTransition("failed_gates", "ready")
	if !result {
		t.Errorf("expected failed_gates->ready to be allowed, got false")
	}
}

func TestCanTransition_LaneViolationToReady_Allowed(t *testing.T) {
	result := CanTransition("lane_violation", "ready")
	if !result {
		t.Errorf("expected lane_violation->ready to be allowed, got false")
	}
}

func TestNextAllowedStates_DraftReturnsReady(t *testing.T) {
	states := NextAllowedStates("draft")
	if len(states) != 1 || states[0] != "ready" {
		t.Errorf("expected draft to transition only to ready, got %v", states)
	}
}

func TestNextAllowedStates_IntegratingReturnsManyStates(t *testing.T) {
	states := NextAllowedStates("integrating")
	expected := map[string]bool{
		"merged":        true,
		"conflict":      true,
		"failed_gates":  true,
		"lane_violation": true,
	}
	if len(states) != len(expected) {
		t.Errorf("expected %d allowed states from integrating, got %d: %v", len(expected), len(states), states)
	}
	for _, state := range states {
		if !expected[state] {
			t.Errorf("unexpected state %s in allowed states from integrating", state)
		}
	}
}
