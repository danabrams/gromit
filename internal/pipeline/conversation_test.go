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
