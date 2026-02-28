package tui

import (
    "strings"
    "testing"
)

func TestConversationControllerStreamsEvents(t *testing.T) {
    timeline := []fakeConversationStep{
        {Event: ConversationEvent{Type: ConversationEventTypeStream, Text: "hello"}},
        {Event: ConversationEvent{Type: ConversationEventTypeStream, Text: " world"}},
        {Event: ConversationEvent{Type: ConversationEventTypeDone, Text: "complete"}},
    }
    session := newFakeConversationSession(timeline)
    controller := NewConversationController(session)

    cmd := controller.Init()
    for cmd != nil {
        msg := cmd()
        model, next := controller.Update(msg)
        var ok bool
        controller, ok = model.(*ConversationController)
        if !ok {
            t.Fatalf("expected ConversationController, got %T", model)
        }
        cmd = next
    }

    view := controller.View()
    if !strings.Contains(view, "hello") || !strings.Contains(view, "world") {
        t.Fatalf("view output missing streamed text: %q", view)
    }
}
