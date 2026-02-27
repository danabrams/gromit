package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModel_InitialViewRendersDashboard(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	// View() should return a non-empty string for the dashboard
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view, got empty string")
	}
}

func TestModel_ImplementsTeaModel(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	// Check that the model implements the tea.Model interface
	var _ tea.Model = m

	// Test that Init returns a cmd or nil
	cmd := m.Init()
	_ = cmd // cmd can be nil, that's fine
}

func TestModel_SwitchViewToDashboard(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	// Initially should be on Dashboard
	if m.currentView != ViewDashboard {
		t.Errorf("expected initial view to be Dashboard, got %v", m.currentView)
	}

	m.SwitchView(ViewQueue)
	if m.currentView != ViewQueue {
		t.Errorf("expected view to be Queue, got %v", m.currentView)
	}

	m.SwitchView(ViewDashboard)
	if m.currentView != ViewDashboard {
		t.Errorf("expected view to be Dashboard, got %v", m.currentView)
	}
}

func TestModel_FocusMovement(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	// Should start with focus on the first panel
	if m.focusedPanel != 0 {
		t.Errorf("expected initial focus to be 0, got %d", m.focusedPanel)
	}

	// Move focus forward
	m.FocusNext()
	if m.focusedPanel != 1 {
		t.Errorf("expected focus to be 1 after FocusNext, got %d", m.focusedPanel)
	}

	// Move focus backward
	m.FocusPrev()
	if m.focusedPanel != 0 {
		t.Errorf("expected focus to be 0 after FocusPrev, got %d", m.focusedPanel)
	}
}
