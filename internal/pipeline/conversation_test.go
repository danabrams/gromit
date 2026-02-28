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
