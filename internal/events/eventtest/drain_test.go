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

// TestWaitForCondition tests that WaitForCondition polls until condition is true or timeout.
func TestWaitForCondition(t *testing.T) {
	t.Parallel()

	counter := 0
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := WaitForCondition(ctx, func() bool {
		counter++
		return counter >= 5
	})
	if err != nil {
		t.Fatalf("WaitForCondition failed: %v", err)
	}
	if counter < 5 {
		t.Errorf("Expected counter >= 5, got %d", counter)
	}
}

// TestWaitForCondition_Timeout tests that WaitForCondition times out if condition never becomes true.
func TestWaitForCondition_Timeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := WaitForCondition(ctx, func() bool {
		return false // Always false
	})
	if err == nil {
		t.Fatal("WaitForCondition should timeout, but didn't")
	}
}
