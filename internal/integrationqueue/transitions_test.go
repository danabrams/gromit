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
