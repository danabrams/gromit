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

func TestNewPhaseContext_ClampsToRunDeadline(t *testing.T) {
	bc := &runtypes.BeadContext{
		ParentCtx:   context.Background(),
		BeadTimeout: 5 * time.Second,
		RunDeadline: time.Now().Add(120 * time.Millisecond),
	}

	_, cancel, meta := newPhaseContext(bc, "validate", 2)
	defer cancel()

	if !meta.ClampedByRunDeadline {
		t.Fatal("meta.ClampedByRunDeadline = false, want true")
	}
	if meta.EffectiveTimeout > 300*time.Millisecond {
		t.Fatalf("meta.EffectiveTimeout = %v, expected clamp near run deadline", meta.EffectiveTimeout)
	}
}

func TestNewPhaseContext_UsesExplicitOverride(t *testing.T) {
	bc := &runtypes.BeadContext{
		ParentCtx:   context.Background(),
		BeadTimeout: 9 * time.Second,
	}

	_, cancel, meta := newPhaseContext(bc, "green", 3)
	defer cancel()

	if meta.RequestedTimeout != 3*time.Second {
		t.Fatalf("meta.RequestedTimeout = %v, want %v", meta.RequestedTimeout, 3*time.Second)
	}
	if meta.EffectiveTimeout != 3*time.Second {
		t.Fatalf("meta.EffectiveTimeout = %v, want %v", meta.EffectiveTimeout, 3*time.Second)
	}
	if meta.TimeoutSource != phaseTimeoutSourceOverride {
		t.Fatalf("meta.TimeoutSource = %q, want %q", meta.TimeoutSource, phaseTimeoutSourceOverride)
	}
}
