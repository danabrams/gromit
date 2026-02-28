package integrationqueue

import (
	"testing"
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
