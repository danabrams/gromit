// Package eventtest provides shared test helpers for draining events from an emitter channel.
package eventtest

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
)

// DefaultDrainTimeout is the default timeout used when draining events.
const DefaultDrainTimeout = 50 * time.Millisecond

// DefaultSubscriberTimeout is the default timeout used when waiting for subscribers.
const DefaultSubscriberTimeout = 1 * time.Second

// DrainEvents reads all available events from ch until timeout elapses.
// If timeout is zero or negative, DefaultDrainTimeout is used.
func DrainEvents(tb testing.TB, ch <-chan events.Event, timeout ...time.Duration) []events.Event {
	tb.Helper()

	dur := DefaultDrainTimeout
	if len(timeout) > 0 && timeout[0] > 0 {
		dur = timeout[0]
	}

	var collected []events.Event
	deadline := time.After(dur)
	for {
		select {
		case evt := <-ch:
			collected = append(collected, evt)
		case <-deadline:
			return collected
		}
	}
}

// WaitForSubscriberReady polls the emitter until a subscriber is detected or context expires.
// This replaces time.Sleep for synchronizing with subscriber goroutines.
func WaitForSubscriberReady(ctx context.Context, emitter *events.Emitter) error {
	ticker := time.NewTicker(1 * time.Millisecond)
	defer ticker.Stop()

	for {
		if emitter.HasSubscribers() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Continue polling
		}
	}
}

// WaitForCondition polls a condition function until it returns true or context expires.
// This is a generic replacement for time.Sleep when waiting for events to be processed.
func WaitForCondition(ctx context.Context, condition func() bool) error {
	ticker := time.NewTicker(1 * time.Millisecond)
	defer ticker.Stop()

	for {
		if condition() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Continue polling
		}
	}
}
