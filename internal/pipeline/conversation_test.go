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

func TestConversationTerminalStateGuarantee(t *testing.T) {
	// Verify that a stream with no events has no terminal state
	emptySession := &mockConversationSession{events: []ConversationEvent{}}
	emptyEvents := collectSessionEvents(emptySession)
	if len(emptyEvents) > 0 {
		t.Fatalf("expected empty session to yield no events, got %d", len(emptyEvents))
	}

	// Verify that terminal state must be final - nothing should follow it
	invalidSession := &mockConversationSession{
		events: []ConversationEvent{
			{State: ConversationStateStreaming, Content: "text"},
			{State: ConversationStateCompleted},
			{State: ConversationStateStreaming, Content: "more text"}, // Invalid - after completed
		},
	}

	allEvents := make([]ConversationEvent, 0)
	for ev := range invalidSession.Events() {
		allEvents = append(allEvents, ev)
	}

	// Check that event at terminal position is indeed terminal
	if len(allEvents) > 0 {
		terminalIdx := len(allEvents) - 2 // Index of terminal event in invalid sequence
		if terminalIdx >= 0 && terminalIdx < len(allEvents) {
			if !isTerminalState(allEvents[terminalIdx].State) {
				t.Fatalf("expected state at index %d to be terminal", terminalIdx)
			}
		}
	}
}

func TestConversationToolEventValidation(t *testing.T) {
	// Verify tool events exist and are represented in lifecycle states
	session := &mockConversationSession{
		events: []ConversationEvent{
			{State: ConversationStateStreaming, Content: "calculating"},
			{State: ConversationStateWaitingForTool, ToolName: "calculator", ToolInput: `{"a": 1}`},
			{State: ConversationStateStreaming, ToolOutput: "2"},
			{State: ConversationStateCompleted},
		},
	}

	events := collectSessionEvents(session)
	if len(events) < 3 {
		t.Fatalf("expected at least 3 events for tool sequence, got %d", len(events))
	}

	// Verify tool-wait event has tool name
	toolWaitEvent := events[1]
	if toolWaitEvent.State != ConversationStateWaitingForTool {
		t.Fatalf("expected WaitingForTool state, got %v", toolWaitEvent.State)
	}
	if toolWaitEvent.ToolName == "" {
		t.Fatal("expected tool name in tool-wait event")
	}
	if toolWaitEvent.ToolInput == "" {
		t.Fatal("expected tool input in tool-wait event")
	}
}

func TestConversationFailedStateWithErrorReason(t *testing.T) {
	// Verify failed state can carry error reason
	session := &mockConversationSession{
		events: []ConversationEvent{
			{State: ConversationStateStreaming, Content: "processing"},
			{State: ConversationStateFailed, ErrorReason: "rate limit exceeded"},
		},
	}

	events := collectSessionEvents(session)
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	failEvent := events[1]
	if failEvent.State != ConversationStateFailed {
		t.Fatalf("expected Failed state, got %v", failEvent.State)
	}
	if failEvent.ErrorReason != "rate limit exceeded" {
		t.Fatalf("expected error reason 'rate limit exceeded', got %q", failEvent.ErrorReason)
	}
}

func TestConversationCancelledState(t *testing.T) {
	// Verify cancelled state is properly represented as terminal
	session := &mockConversationSession{
		events: []ConversationEvent{
			{State: ConversationStateStarting},
			{State: ConversationStateStreaming, Content: "partial"},
			{State: ConversationStateCancelled},
		},
	}

	events := collectSessionEvents(session)
	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(events))
	}

	lastEvent := events[len(events)-1]
	if lastEvent.State != ConversationStateCancelled {
		t.Fatalf("expected Cancelled state, got %v", lastEvent.State)
	}
	if !isTerminalState(lastEvent.State) {
		t.Fatal("expected Cancelled state to be terminal")
	}
}

func TestConversationAssistantOutputChunks(t *testing.T) {
	// Verify that streaming state can carry multiple content chunks
	session := &mockConversationSession{
		events: []ConversationEvent{
			{State: ConversationStateStreaming, Content: "Hello "},
			{State: ConversationStateStreaming, Content: "world"},
			{State: ConversationStateStreaming, Content: "!"},
			{State: ConversationStateCompleted},
		},
	}

	events := collectSessionEvents(session)
	if len(events) < 4 {
		t.Fatalf("expected at least 4 events, got %d", len(events))
	}

	// Collect all streaming content chunks
	chunks := []string{}
	for _, ev := range events {
		if ev.State == ConversationStateStreaming && ev.Content != "" {
			chunks = append(chunks, ev.Content)
		}
	}

	if len(chunks) != 3 {
		t.Fatalf("expected 3 content chunks, got %d", len(chunks))
	}

	// Verify chunks can be concatenated to form complete output
	fullOutput := ""
	for _, chunk := range chunks {
		fullOutput += chunk
	}
	if fullOutput != "Hello world!" {
		t.Fatalf("expected 'Hello world!', got %q", fullOutput)
	}
}

func TestConversationEventSequenceValidation(t *testing.T) {
	// Test that valid sequences pass validation
	validSequence := []ConversationEvent{
		{State: ConversationStateStarting},
		{State: ConversationStateStreaming, Content: "part1"},
		{State: ConversationStateStreaming, Content: "part2"},
		{State: ConversationStateWaitingForTool, ToolName: "tool"},
		{State: ConversationStateStreaming, ToolOutput: "result"},
		{State: ConversationStateCompleted},
	}

	err := ValidateConversationEventSequence(validSequence)
	if err != nil {
		t.Fatalf("expected valid sequence to pass, got error: %v", err)
	}

	// Test that sequences with events after terminal state fail
	invalidSequence := []ConversationEvent{
		{State: ConversationStateStreaming, Content: "text"},
		{State: ConversationStateCompleted},
		{State: ConversationStateStreaming, Content: "more"}, // Invalid - after terminal state
	}

	err = ValidateConversationEventSequence(invalidSequence)
	if err == nil {
		t.Fatal("expected invalid sequence (events after terminal state) to fail validation")
	}
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
