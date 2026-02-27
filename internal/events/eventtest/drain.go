// Package eventtest provides shared test helpers for draining events from an emitter channel.
package eventtest

import (
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
)

// DefaultDrainTimeout is the default timeout used when draining events.
const DefaultDrainTimeout = 50 * time.Millisecond

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
