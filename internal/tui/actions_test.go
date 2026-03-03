package tui

import (
    "testing"

    tea "github.com/charmbracelet/bubbletea"
)

type mockActionableListItem struct {
    id      string
    title   string
    summary string
}

func (m *mockActionableListItem) Title() string {
    return m.title
}

func (m *mockActionableListItem) Summary() string {
    return m.summary
}

func (m *mockActionableListItem) Identifier() string {
    return m.id
}

func TestHandleActionTransitionActionsSetPendingAction(t *testing.T) {
    cases := []struct {
        key     string
        tab     Tab
        command string
        id      string
    }{
        {key: "r", tab: Tab("backlog"), command: "refine", id: "idea-123"},
        {key: "p", tab: Tab("specs"), command: "plan", id: "specs/my-spec"},
        {key: "d", tab: Tab("plans"), command: "decompose", id: "plan-teen"},
    }

    for _, tc := range cases {
        tc := tc
        t.Run(tc.key, func(t *testing.T) {
            m := &Model{}
            item := &mockActionableListItem{id: tc.id}
            model, cmd := handleAction(m, tc.key, tc.tab, item, nil)

            if model != m {
                t.Fatalf("expected handleAction to return the original model")
            }
            if cmd == nil {
                t.Fatalf("expected non-nil command for key %q", tc.key)
            }
            if _, ok := cmd().(tea.QuitMsg); !ok {
                t.Fatalf("expected tea.QuitMsg for key %q", tc.key)
            }
            if m.pendingAction == nil {
                t.Fatalf("expected pending action for key %q", tc.key)
            }
            if m.pendingAction.Command != tc.command {
                t.Fatalf("unexpected command: got %q want %q", m.pendingAction.Command, tc.command)
            }
            if len(m.pendingAction.Args) != 1 || m.pendingAction.Args[0] != tc.id {
                t.Fatalf("unexpected args: %+v", m.pendingAction.Args)
            }
        })
    }
}
