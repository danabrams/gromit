package procutil

import "testing"

func TestCollectDescendantsImplInvalidPID(t *testing.T) {
	if got := collectDescendantsImpl(999999999); got != nil {
		t.Fatalf("expected nil for invalid PID, got %v", got)
	}
}
