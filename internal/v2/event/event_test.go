package event

import (
	"reflect"
	"testing"
)

func TestEventDefinitions(t *testing.T) {
	t.Run("BaseEventFields", func(t *testing.T) {
		t.Helper()
		typ := reflect.TypeOf(Event{})
		for _, field := range []string{"SchemaVersion", "Timestamp", "Type"} {
			if _, ok := typ.FieldByName(field); !ok {
				t.Fatalf("expected Event to expose %s", field)
			}
		}
		if SchemaVersion == 0 {
			t.Fatalf("schema version must be non-zero")
		}
	})

	t.Run("LifecycleEventsEmbedBase", func(t *testing.T) {
		expectEmbeddedEvent(t, reflect.TypeOf(SpecStartedEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(SpecCompletedEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(SpecFailedEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(BeadStartedEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(BeadCompletedEvent{}))
	})

	t.Run("StageEventsEmbedBase", func(t *testing.T) {
		expectEmbeddedEvent(t, reflect.TypeOf(StageStartedEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(StageCompletedEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(StageFailedEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(StageRetryingEvent{}))
	})

	t.Run("ValidationReviewScopeTelemetryEmbedBase", func(t *testing.T) {
		expectEmbeddedEvent(t, reflect.TypeOf(ValidationEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(ReviewEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(ScopeEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(TelemetryEvent{}))
	})
}

func expectEmbeddedEvent(t *testing.T, typ reflect.Type) {
	t.Helper()
	field, ok := typ.FieldByName("Event")
	if !ok {
		t.Fatalf("type %s must embed Event", typ.Name())
	}
	if !field.Anonymous {
		t.Fatalf("field Event in %s must be anonymous", typ.Name())
	}
}
