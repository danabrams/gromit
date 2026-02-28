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

func TestIsTerminalState_MergedIsTerminal(t *testing.T) {
	if !IsTerminalState("merged") {
		t.Errorf("expected merged to be terminal state, got false")
	}
}

func TestIsTerminalState_ReadyIsNotTerminal(t *testing.T) {
	if IsTerminalState("ready") {
		t.Errorf("expected ready to not be terminal state, got true")
	}
}

func TestIsBlockedState_ConflictIsBlocked(t *testing.T) {
	if !IsBlockedState("conflict") {
		t.Errorf("expected conflict to be blocked state, got false")
	}
}

func TestIsBlockedState_FailedGatesIsBlocked(t *testing.T) {
	if !IsBlockedState("failed_gates") {
		t.Errorf("expected failed_gates to be blocked state, got false")
	}
}

func TestIsBlockedState_LaneViolationIsBlocked(t *testing.T) {
	if !IsBlockedState("lane_violation") {
		t.Errorf("expected lane_violation to be blocked state, got false")
	}
}

func TestIsBlockedState_ReadyIsNotBlocked(t *testing.T) {
	if IsBlockedState("ready") {
		t.Errorf("expected ready to not be blocked state, got true")
	}
}

func TestCheckTransition_DraftToReady_AllowedReturnsNil(t *testing.T) {
	err := CheckTransition("draft", "ready")
	if err != nil {
		t.Errorf("expected draft->ready to be allowed, got error: %v", err)
	}
}

func TestCheckTransition_DraftToIntegrating_DisallowedReturnsError(t *testing.T) {
	err := CheckTransition("draft", "integrating")
	if err != ErrInvalidTransition {
		t.Errorf("expected ErrInvalidTransition, got: %v", err)
	}
}

func TestApplyTransition_ValidTransition_UpdatesEntryAndReturnsNil(t *testing.T) {
	entry := &Entry{
		State:     "draft",
		UpdatedAt: time.Now().Add(-time.Hour),
	}
	oldTime := entry.UpdatedAt

	err := ApplyTransition(entry, "ready", "manual transition")
	if err != nil {
		t.Errorf("expected nil error for valid transition, got: %v", err)
	}
	if entry.State != "ready" {
		t.Errorf("expected state to be ready, got: %s", entry.State)
	}
	if entry.LastTransitionReason != "manual transition" {
		t.Errorf("expected last_transition_reason to be 'manual transition', got: %s", entry.LastTransitionReason)
	}
	if entry.UpdatedAt.Before(oldTime) || entry.UpdatedAt.Equal(oldTime) {
		t.Errorf("expected UpdatedAt to be updated, old: %v, new: %v", oldTime, entry.UpdatedAt)
	}
}

func TestApplyTransition_InvalidTransition_ReturnsErrorWithoutModifyingEntry(t *testing.T) {
	entry := &Entry{
		State:                 "draft",
		UpdatedAt:             time.Now().Add(-time.Hour),
		LastTransitionReason:  "old reason",
	}
	oldTime := entry.UpdatedAt
	oldState := entry.State
	oldReason := entry.LastTransitionReason

	err := ApplyTransition(entry, "integrating", "invalid transition")
	if err != ErrInvalidTransition {
		t.Errorf("expected ErrInvalidTransition, got: %v", err)
	}
	if entry.State != oldState {
		t.Errorf("expected state to remain unchanged, got: %s", entry.State)
	}
	if entry.LastTransitionReason != oldReason {
		t.Errorf("expected reason to remain unchanged, got: %s", entry.LastTransitionReason)
	}
	if entry.UpdatedAt != oldTime {
		t.Errorf("expected UpdatedAt to remain unchanged, got: %v", entry.UpdatedAt)
	}
}

func TestCheckTransition_AllAllowedTransitions(t *testing.T) {
	allowedCases := []struct {
		from, to string
	}{
		{"draft", "ready"},
		{"ready", "integrating"},
		{"integrating", "merged"},
		{"integrating", "conflict"},
		{"integrating", "failed_gates"},
		{"integrating", "lane_violation"},
		{"conflict", "ready"},
		{"failed_gates", "ready"},
		{"lane_violation", "ready"},
	}

	for _, tc := range allowedCases {
		t.Run(tc.from+"->"+tc.to, func(t *testing.T) {
			err := CheckTransition(tc.from, tc.to)
			if err != nil {
				t.Errorf("expected %s->%s to be allowed, got error: %v", tc.from, tc.to, err)
			}
		})
	}
}
