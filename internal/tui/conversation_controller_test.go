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

func TestConversationControllerFollowUpDuringToolWait(t *testing.T) {
    toolResultRelease := make(chan struct{})
    timeline := []fakeConversationStep{
        {Event: ConversationEvent{Type: ConversationEventTypeStream, Text: "greeting"}},
        {Event: ConversationEvent{Type: ConversationEventTypeToolWait, Text: "waiting", ToolName: "formatter"}},
        {Event: ConversationEvent{Type: ConversationEventTypeToolResult, Text: "done"}, BlockUntil: toolResultRelease},
    }
    session := newFakeConversationSession(timeline)
    prompt := "please follow up"
    controller := NewConversationController(session, WithFollowUpProvider(func() string { return prompt }))

    cmd := controller.Init()
    if cmd == nil {
        t.Fatal("expected init to return a watcher command")
    }

    // drain events until tool wait
    msg := cmd()
    model, cmd := controller.Update(msg)
    ctrl, ok := model.(*ConversationController)
    if !ok {
        t.Fatalf("expected ConversationController, got %T", model)
    }

    msg = cmd()
    model, cmd = ctrl.Update(msg)
    ctrl, ok = model.(*ConversationController)
    if !ok {
        t.Fatalf("expected ConversationController, got %T", model)
    }

    view := ctrl.View()
    if !strings.Contains(view, "[waiting for tool]") {
        t.Fatalf("expected waiting state after tool wait event, got %q", view)
    }

    model, followUpCmd := ctrl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
    ctrl, ok = model.(*ConversationController)
    if !ok {
        t.Fatalf("expected ConversationController, got %T", model)
    }
    if followUpCmd == nil {
        t.Fatal("expected follow-up to restart the watcher")
    }

    if !strings.Contains(ctrl.View(), "[waiting for tool]") {
        t.Fatalf("expected to still show waiting indicator after follow-up request, got %q", ctrl.View())
    }

    calls := session.FollowUpCalls()
    if len(calls) != 1 || calls[0] != prompt {
        t.Fatalf("unexpected follow-up calls: %v", calls)
    }

    close(toolResultRelease)

    for followUpCmd != nil {
        msg = followUpCmd()
        model, followUpCmd = ctrl.Update(msg)
        ctrl, ok = model.(*ConversationController)
        if !ok {
            t.Fatalf("expected ConversationController, got %T", model)
        }
    }

    finalView := ctrl.View()
    if strings.Contains(finalView, "[waiting for tool]") {
        t.Fatalf("expected waiting indicator to disappear after tool result, got %q", finalView)
    }
    if !strings.Contains(finalView, "done") {
        t.Fatalf("expected final result in view, got %q", finalView)
    }
}

func TestConversationControllerIgnoresLateEventsAfterCancel(t *testing.T) {
    timeline := []fakeConversationStep{
        {Event: ConversationEvent{Type: ConversationEventTypeStream, Text: "start"}},
        {Event: ConversationEvent{Type: ConversationEventTypeStream, Text: "mid"}},
        {Event: ConversationEvent{Type: ConversationEventTypeStream, Text: "late"}},
    }
    session := newFakeConversationSession(timeline)
    controller := NewConversationController(session)

    cmd := controller.Init()
    if cmd == nil {
        t.Fatal("expected watcher command from init")
    }

    msg := cmd()
    model, cmd := controller.Update(msg)
    ctrl, ok := model.(*ConversationController)
    if !ok {
        t.Fatalf("expected ConversationController, got %T", model)
    }

    model, cancelCmd := ctrl.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
    ctrl, ok = model.(*ConversationController)
    if !ok {
        t.Fatalf("expected ConversationController, got %T", model)
    }
    if cancelCmd == nil {
        t.Fatal("expected cancel to continue watching the session")
    }

    for cancelCmd != nil {
        msg = cancelCmd()
        model, cancelCmd = ctrl.Update(msg)
        ctrl, ok = model.(*ConversationController)
        if !ok {
            t.Fatalf("expected ConversationController, got %T", model)
        }
    }

    view := ctrl.View()
    if !strings.Contains(view, "start") {
        t.Fatalf("expected start event, got %q", view)
    }
    if strings.Contains(view, "mid") {
        t.Fatalf("expected mid event to be ignored after cancel, got %q", view)
    }
    if strings.Contains(view, "late") {
        t.Fatalf("expected late event to be ignored after cancel, got %q", view)
    }
    if !strings.Contains(view, "[ignored 2 late events]") {
        t.Fatalf("expected indicator about ignored late events, got %q", view)
    }
}
