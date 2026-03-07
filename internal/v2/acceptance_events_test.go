//go:build acceptance
// +build acceptance

package v2

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/event"
)

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

func sampleTypedEvents() []event.TypedEvent {
	now := time.Now()
	return []event.TypedEvent{
		&event.SpecStartedEvent{
			Event: event.Event{SchemaVersion: event.SchemaVersion, Timestamp: now, Type: event.EventTypeSpecStarted},
		},
		&event.BeadStartedEvent{
			Event: event.Event{SchemaVersion: event.SchemaVersion, Timestamp: now, Type: event.EventTypeBeadStarted},
		},
		&event.StageStartedEvent{
			Event: event.Event{SchemaVersion: event.SchemaVersion, Timestamp: now, Type: event.EventTypeStageStarted},
		},
		&event.StageCompletedEvent{
			Event: event.Event{SchemaVersion: event.SchemaVersion, Timestamp: now, Type: event.EventTypeStageCompleted},
		},
		&event.BeadCompletedEvent{
			Event: event.Event{SchemaVersion: event.SchemaVersion, Timestamp: now, Type: event.EventTypeBeadCompleted},
		},
		&event.SpecCompletedEvent{
			Event: event.Event{SchemaVersion: event.SchemaVersion, Timestamp: now, Type: event.EventTypeSpecCompleted},
		},
	}
}

func eventHasSchemaVersion(evt event.TypedEvent) bool {
	val := reflect.ValueOf(evt)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	field := val.FieldByName("Event")
	if !field.IsValid() {
		return false
	}
	schema := field.FieldByName("SchemaVersion")
	if !schema.IsValid() {
		return false
	}
	return int(schema.Int()) == event.SchemaVersion
}

type unknownTypedEvent struct {
	event.Event
}

func (unknownTypedEvent) EventType() string { return "unknown.typed" }

func newUnknownEvent() event.TypedEvent {
	return unknownTypedEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          "unknown.typed",
		},
	}
}

func ignoreUnknownEvent(t *testing.T, evt event.TypedEvent) {
	t.Helper()

	typedEmitter := event.NewEmitter()
	defer typedEmitter.Close()

	legacyEmitter := events.NewEmitter()
	defer legacyEmitter.Close()

	ch := legacyEmitter.Subscribe()

	event.BridgeTypedToLegacy(typedEmitter, legacyEmitter)
	typedEmitter.Emit(evt)

	select {
	case <-ch:
		t.Fatalf("unexpected legacy event delivered for %T", evt)
	case <-time.After(50 * time.Millisecond):
	}

	legacyEmitter.Unsubscribe(ch)
}

func canonicalTypedSequence() []event.TypedEvent {
	now := time.Now()
	return []event.TypedEvent{
		&event.SpecStartedEvent{
			Event: event.Event{SchemaVersion: event.SchemaVersion, Timestamp: now, Type: event.EventTypeSpecStarted},
		},
		&event.BeadStartedEvent{
			Event: event.Event{SchemaVersion: event.SchemaVersion, Timestamp: now, Type: event.EventTypeBeadStarted},
		},
		&event.StageStartedEvent{
			Event: event.Event{SchemaVersion: event.SchemaVersion, Timestamp: now, Type: event.EventTypeStageStarted},
		},
		&event.StageCompletedEvent{
			Event: event.Event{SchemaVersion: event.SchemaVersion, Timestamp: now, Type: event.EventTypeStageCompleted},
		},
		&event.BeadCompletedEvent{
			Event: event.Event{SchemaVersion: event.SchemaVersion, Timestamp: now, Type: event.EventTypeBeadCompleted},
		},
		&event.SpecCompletedEvent{
			Event: event.Event{SchemaVersion: event.SchemaVersion, Timestamp: now, Type: event.EventTypeSpecCompleted},
		},
	}
}

func canonicalEventOrder() []string {
	return []string{
		event.EventTypeSpecStarted,
		event.EventTypeBeadStarted,
		event.EventTypeStageStarted,
		event.EventTypeStageCompleted,
		event.EventTypeBeadCompleted,
		event.EventTypeSpecCompleted,
	}
}

func assertTypedEventOrder(t *testing.T, seq []event.TypedEvent, want []string) {
	t.Helper()

	if len(seq) != len(want) {
		t.Fatalf("sequence length = %d, want %d", len(seq), len(want))
	}

	emitter := event.NewEmitter()
	defer emitter.Close()

	var (
		mu       sync.Mutex
		observed []string
		wg       sync.WaitGroup
	)
	wg.Add(len(seq))

	emitter.Subscribe(func(evt event.TypedEvent) {
		mu.Lock()
		observed = append(observed, evt.EventType())
		mu.Unlock()
		wg.Done()
	})

	for _, evt := range seq {
		emitter.Emit(evt)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for typed events")
	}

	mu.Lock()
	defer mu.Unlock()

	for i, wantType := range want {
		if observed[i] != wantType {
			t.Fatalf("event[%d] = %s, want %s", i, observed[i], wantType)
		}
	}
}
