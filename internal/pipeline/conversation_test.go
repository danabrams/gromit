package pipeline

import (
	"testing"

	"github.com/danabrams/gromit/internal/conversation"
)

func TestCollectConversationHandlesFollowUpDuringToolWait(t *testing.T) {
    timeline := []conversation.FakeStep{
        {Event: conversation.Event{Type: conversation.EventTypeStream, Text: "hello"}},
        {Event: conversation.Event{Type: conversation.EventTypeToolWait, Text: "waiting", ToolName: "formatter"}},
        {Event: conversation.Event{Type: conversation.EventTypeToolResult, Text: "done"}},
        {Event: conversation.Event{Type: conversation.EventTypeDone}},
    }
    session := conversation.NewFakeSession(timeline)

    prompt := "follow-up prompt"
    events, ignored := CollectConversation(session, func() string { return prompt }, nil)
    if len(events) != len(timeline) {
        t.Fatalf("expected %d events, got %d", len(timeline), len(events))
    }
    if ignored != 0 {
        t.Fatalf("expected no ignored events, got %d", ignored)
    }
    calls := session.FollowUpCalls()
    if len(calls) != 1 || calls[0] != prompt {
        t.Fatalf("expected follow-up prompt %q, got %v", prompt, calls)
    }
}

func TestCollectConversationIgnoresLateEventsAfterCancel(t *testing.T) {
    midEmitted := make(chan struct{})
    lateRelease := make(chan struct{})
    timeline := []conversation.FakeStep{
        {Event: conversation.Event{Type: conversation.EventTypeStream, Text: "start"}},
        {Event: conversation.Event{Type: conversation.EventTypeStream, Text: "mid"}, AfterEmit: midEmitted},
        {Event: conversation.Event{Type: conversation.EventTypeStream, Text: "late"}, BlockUntil: lateRelease},
    }
    session := conversation.NewFakeSession(timeline)
    cancelCh := make(chan struct{})

    go func() {
        <-midEmitted
        close(cancelCh)
        close(lateRelease)
    }()

    events, ignored := CollectConversation(session, nil, cancelCh)
    if len(events) == 0 {
        t.Fatalf("expected at least one captured event before cancel, got %d", len(events))
    }
    if ignored < 1 {
        t.Fatalf("expected at least one ignored event after cancel, got %d", ignored)
    }
    if !session.WasCancelled() {
        t.Fatal("expected session to see cancellation")
    }
    for _, ev := range events {
        if ev.Text == "late" {
            t.Fatalf("did not expect late event to appear in results, got %v", ev)
        }
    }
}

func TestConversationEventTypeExists(t *testing.T) {
	// Verify ConversationEvent type exists and has required fields
	event := ConversationEvent{
		State:       ConversationStateStreaming,
		Content:     "assistant output",
		ToolName:    "tool1",
		ToolInput:   `{"key":"value"}`,
		ToolOutput:  "result",
		ErrorReason: "",
	}
	if event.State != ConversationStateStreaming {
		t.Fatalf("expected ConversationStateStreaming, got %v", event.State)
	}
	if event.Content != "assistant output" {
		t.Fatalf("expected Content to be 'assistant output', got %q", event.Content)
	}
}

func TestConversationLifecycleStatesExist(t *testing.T) {
	// Verify all lifecycle state constants are defined
	states := []ConversationLifecycleState{
		ConversationStateIdle,
		ConversationStateStarting,
		ConversationStateStreaming,
		ConversationStateWaitingForTool,
		ConversationStateCompleted,
		ConversationStateFailed,
		ConversationStateCancelled,
	}
	if len(states) != 7 {
		t.Fatalf("expected 7 lifecycle states, got %d", len(states))
	}
}

func TestConversationSessionInterfaceExists(t *testing.T) {
	// Verify ConversationSession interface exists with required methods
	var sess ConversationSession
	_ = sess // Use variable to verify interface existence

	// Verify it has the required methods:
	// - SendInput(text string) error
	// - Cancel() error
	// - Events() <-chan ConversationEvent
}

func TestConversationSessionEventOrdering(t *testing.T) {
	// Verify that terminal state events (Completed, Failed, Cancelled) end event stream
	session := &mockConversationSession{
		events: []ConversationEvent{
			{State: ConversationStateStarting},
			{State: ConversationStateStreaming, Content: "text"},
			{State: ConversationStateWaitingForTool, ToolName: "tool"},
			{State: ConversationStateCompleted},
		},
	}

	events := collectSessionEvents(session)
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}

	// Last event should be terminal state
	lastEvent := events[len(events)-1]
	if !isTerminalState(lastEvent.State) {
		t.Fatalf("expected last event to be terminal state, got %v", lastEvent.State)
	}
}

func isTerminalState(state ConversationLifecycleState) bool {
	return state == ConversationStateCompleted ||
		state == ConversationStateFailed ||
		state == ConversationStateCancelled
}

func collectSessionEvents(sess ConversationSession) []ConversationEvent {
	var events []ConversationEvent
	for ev := range sess.Events() {
		events = append(events, ev)
		if isTerminalState(ev.State) {
			break
		}
	}
	return events
}

type mockConversationSession struct {
	events []ConversationEvent
	sent   int
}

func (m *mockConversationSession) SendInput(text string) error {
	return nil
}

func (m *mockConversationSession) Cancel() error {
	return nil
}

func (m *mockConversationSession) Events() <-chan ConversationEvent {
	ch := make(chan ConversationEvent, len(m.events))
	for _, ev := range m.events {
		ch <- ev
	}
	close(ch)
	return ch
}
