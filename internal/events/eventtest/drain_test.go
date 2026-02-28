//go:build !wasm

package eventtest

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
)

// TestWaitForSubscriberReady tests that WaitForSubscriberReady polls until subscriber is present.
func TestWaitForSubscriberReady(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()

	// Start a goroutine that will subscribe after a short delay
	go func() {
		time.Sleep(5 * time.Millisecond)
		_ = emitter.Subscribe()
	}()

	// WaitForSubscriberReady should poll and detect the subscriber without relying on time.Sleep(10ms)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := WaitForSubscriberReady(ctx, emitter)
	if err != nil {
		t.Fatalf("WaitForSubscriberReady failed: %v", err)
	}
}

// TestWaitForSubscriberReady_Timeout tests that WaitForSubscriberReady times out if no subscriber appears.
func TestWaitForSubscriberReady_Timeout(t *testing.T) {
	t.Parallel()

	emitter := events.NewEmitter()

	// Don't subscribe to anything
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := WaitForSubscriberReady(ctx, emitter)
	if err == nil {
		t.Fatal("WaitForSubscriberReady should timeout, but didn't")
	}
}
