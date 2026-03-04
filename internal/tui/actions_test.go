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

func TestHandleActionRefreshCommandIncludesRequestedTab(t *testing.T) {
	m := &Model{}
	_, cmd := handleAction(m, "R", Tab("queue"), nil, nil)

	if cmd == nil {
		t.Fatal("expected refresh command to be non-nil")
	}
	msg := cmd()
	refreshMsg, ok := msg.(pipelineRefreshedMsg)
	if !ok {
		t.Fatalf("expected pipelineRefreshedMsg, got %T", msg)
	}
	if refreshMsg.RequestedTab != Tab("queue") {
		t.Fatalf("expected requested tab to be queue, got %q", refreshMsg.RequestedTab)
	}
}

func TestHandleActionDeleteConfirmationFlow(t *testing.T) {
	store := &Store{}
	deleted := []struct {
		tab        Tab
		identifier string
	}{}
	store.DeletePipelineItemFunc = func(tab Tab, identifier string) {
		deleted = append(deleted, struct {
			tab        Tab
			identifier string
		}{tab: tab, identifier: identifier})
	}
	item := &mockActionableListItem{id: "idea-9"}
	m := &Model{}

	if _, _ = handleAction(m, "x", Tab("backlog"), item, store); !m.confirmDelete {
		t.Fatalf("expected confirmDelete after pressing x")
	}

	if _, _ = handleAction(m, "y", Tab("backlog"), item, store); m.confirmDelete {
		t.Fatalf("expected confirmDelete cleared after confirming deletion")
	}
	if len(deleted) != 1 || deleted[0].tab != Tab("backlog") || deleted[0].identifier != "idea-9" {
		t.Fatalf("expected delete call for backlog item, got %v", deleted)
	}

	m.confirmDelete = true
	if _, _ = handleAction(m, "n", Tab("backlog"), item, store); m.confirmDelete {
		t.Fatalf("expected confirmDelete cleared after pressing n")
	}

	m.confirmDelete = true
	if _, _ = handleAction(m, "esc", Tab("backlog"), item, store); m.confirmDelete {
		t.Fatalf("expected confirmDelete cleared after pressing esc")
	}
	if len(deleted) != 1 {
		t.Fatalf("expected no additional delete calls, got %d", len(deleted))
	}
}

func TestHandleActionDeleteGuardWithEmptySelection(t *testing.T) {
	store := &Store{}
	called := false
	store.DeletePipelineItemFunc = func(Tab, string) {
		called = true
	}
	m := &Model{}

	if _, _ = handleAction(m, "x", Tab("backlog"), nil, store); m.confirmDelete {
		t.Fatalf("expected confirmDelete to remain false when no item is selected")
	}
	if called {
		t.Fatalf("expected no delete call when pressing x with empty selection")
	}

	m.confirmDelete = true
	if _, _ = handleAction(m, "y", Tab("backlog"), nil, store); called {
		t.Fatalf("expected no delete call when confirming without a selection")
	}
}
