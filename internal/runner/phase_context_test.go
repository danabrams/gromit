package runner

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/runner/runtypes"
)

type phaseContextTestKey string

func TestNewPhaseContext_UsesBeadTimeoutFallback(t *testing.T) {
	parent := context.WithValue(context.Background(), phaseContextTestKey("scope"), "parent")
	bc := &runtypes.BeadContext{
		ParentCtx:   parent,
		BeadTimeout: 2 * time.Second,
	}

	phaseCtx, cancel, meta := newPhaseContext(bc, "red", 0)
	defer cancel()

	if got := phaseCtx.Value(phaseContextTestKey("scope")); got != "parent" {
		t.Fatalf("expected phase context to derive from parent context value, got %v", got)
	}
	if meta.Phase != "red" {
		t.Fatalf("meta.Phase = %q, want %q", meta.Phase, "red")
	}
	if meta.RequestedTimeout != 2*time.Second {
		t.Fatalf("meta.RequestedTimeout = %v, want %v", meta.RequestedTimeout, 2*time.Second)
	}
	if meta.EffectiveTimeout != 2*time.Second {
		t.Fatalf("meta.EffectiveTimeout = %v, want %v", meta.EffectiveTimeout, 2*time.Second)
	}
	if meta.ClampedByRunDeadline {
		t.Fatal("meta.ClampedByRunDeadline = true, want false")
	}
}
