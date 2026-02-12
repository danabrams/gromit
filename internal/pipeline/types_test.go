package pipeline

import "testing"

func TestEvent_Creation(t *testing.T) {
	event := Event{
		Type:    EventOutput,
		Content: "test output",
	}

	if event.Type != EventOutput {
		t.Errorf("Type = %v, want %v", event.Type, EventOutput)
	}
	if event.Content != "test output" {
		t.Errorf("Content = %q, want %q", event.Content, "test output")
	}
}

func TestEventType_Constants(t *testing.T) {
	if EventOutput != 0 {
		t.Errorf("EventOutput = %d, want 0", EventOutput)
	}
	if EventSessionStarted != 1 {
		t.Errorf("EventSessionStarted = %d, want 1", EventSessionStarted)
	}
	if EventSessionEnded != 2 {
		t.Errorf("EventSessionEnded = %d, want 2", EventSessionEnded)
	}
	if EventError != 3 {
		t.Errorf("EventError = %d, want 3", EventError)
	}
}
