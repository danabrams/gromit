package tui

import (
    "strings"
    "testing"

    tea "github.com/charmbracelet/bubbletea"
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

func TestConversationControllerCancelsDuringStream(t *testing.T) {
    timeline := []fakeConversationStep{
        {Event: ConversationEvent{Type: ConversationEventTypeStream, Text: "hello"}},
        {Event: ConversationEvent{Type: ConversationEventTypeStream, Text: "world"}},
    }
    session := newFakeConversationSession(timeline)
    controller := NewConversationController(session)

    cmd := controller.Init()
    if cmd == nil {
        t.Fatal("expected initial command to start session watch")
    }

    // Process first event
    msg := cmd()
    model, _ := controller.Update(msg)
    ctrl, ok := model.(*ConversationController)
    if !ok {
        t.Fatalf("expected ConversationController, got %T", model)
    }

    // Cancel before draining remaining events
    model, cancelCmd := ctrl.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
    ctrl, ok = model.(*ConversationController)
    if !ok {
        t.Fatalf("expected ConversationController, got %T", model)
    }
    if cancelCmd == nil {
        t.Fatal("expected cancel to continue watching the session")
    }

    // Drain any pending events so channels close
    for cancelCmd != nil {
        msg := cancelCmd()
        model, cancelCmd = ctrl.Update(msg)
        var ok bool
        ctrl, ok = model.(*ConversationController)
        if !ok {
            t.Fatalf("expected ConversationController, got %T", model)
        }
    }

    view := ctrl.View()
    if !strings.Contains(view, "[cancelled]") {
        t.Fatalf("expected cancelled indicator, got %q", view)
    }
    if strings.Contains(view, "world") {
        t.Fatalf("expected stream after cancel to be ignored, got %q", view)
    }
    if !session.WasCancelled() {
        t.Fatal("expected fake session to see cancellation")
    }
}
