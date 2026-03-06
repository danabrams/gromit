//go:build acceptance
// +build acceptance

package v2

import "testing"

func TestTypedEventsCarrySchemaVersion(t *testing.T) {
	t.Parallel()

	for _, evt := range sampleTypedEvents() {
		if !eventHasSchemaVersion(evt) {
			t.Fatalf("missing schema version for %T", evt)
		}
	}
}

func TestUnknownTypedEventIgnoredByBridge(t *testing.T) {
	t.Parallel()

	ignoreUnknownEvent(t, newUnknownEvent())
}

func TestTypedEventOrderingMatchesLifecycle(t *testing.T) {
	t.Parallel()

	assertTypedEventOrder(t, canonicalTypedSequence(), canonicalEventOrder())
}
