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

func TestModel_KeyboardNavigationTab(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	// Send Tab key to move focus
	msg := tea.KeyMsg{Type: tea.KeyTab}
	model, cmd := m.Update(msg)
	m = model.(*Model)

	// Focus should move to next panel
	if m.focusedPanel != 1 {
		t.Errorf("expected focus to be 1 after Tab, got %d", m.focusedPanel)
	}
	_ = cmd
}

func TestModel_KeyboardNavigationShiftTab(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	m.FocusNext() // Move to panel 1

	// Send Shift+Tab key to move focus backward
	msg := tea.KeyMsg{Type: tea.KeyShiftTab}
	model, cmd := m.Update(msg)
	m = model.(*Model)

	// Focus should move back to panel 0
	if m.focusedPanel != 0 {
		t.Errorf("expected focus to be 0 after Shift+Tab, got %d", m.focusedPanel)
	}
	_ = cmd
}

func TestModel_ScrollHandlingUp(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	m.scrollOffset = 5

	// Send Up key to scroll up
	msg := tea.KeyMsg{Type: tea.KeyUp}
	model, _ := m.Update(msg)
	m = model.(*Model)

	// Scroll offset should decrease
	if m.scrollOffset != 4 {
		t.Errorf("expected scrollOffset to be 4 after Up, got %d", m.scrollOffset)
	}
}

func TestModel_ScrollHandlingDown(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	m.scrollOffset = 0

	// Send Down key to scroll down
	msg := tea.KeyMsg{Type: tea.KeyDown}
	model, _ := m.Update(msg)
	m = model.(*Model)

	// Scroll offset should increase
	if m.scrollOffset != 1 {
		t.Errorf("expected scrollOffset to be 1 after Down, got %d", m.scrollOffset)
	}
}

func TestModel_ScrollHandlingNegativeBound(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	m.scrollOffset = 0

	// Send Up key - should not go below 0
	msg := tea.KeyMsg{Type: tea.KeyUp}
	model, _ := m.Update(msg)
	m = model.(*Model)

	// Scroll offset should stay at 0
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset to be 0 (clamped), got %d", m.scrollOffset)
	}
}

func TestModel_QuitWithQKey(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	// Send 'q' key to quit
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	model, cmd := m.Update(msg)

	// Check if the returned cmd is a tea.Quit()
	if cmd == nil {
		t.Error("expected quit command, got nil")
	}
	_ = model
}

func TestModel_QuitWithCtrlC(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	// Send Ctrl+C to quit
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	model, cmd := m.Update(msg)

	// Check if the returned cmd is a tea.Quit()
	if cmd == nil {
		t.Error("expected quit command, got nil")
	}
	_ = model
}

func TestModel_DashboardViewHasMultiplePanels(t *testing.T) {
	store := &Store{
		Dashboard: DashboardState{
			RunProgress: &RunProgress{
				CurrentIteration: 1,
				MaxIterations:    5,
				Status:           "running",
			},
		},
	}
	m := NewModel(store)

	// View should contain panel information
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view for dashboard")
	}

	// Verify that the view indicates the current focused panel
	if m.focusedPanel == 0 {
		if !containsString(view, "panel 0") && !containsString(view, "progress") {
			t.Error("expected dashboard view to contain progress panel info")
		}
	}
}

func TestModel_ViewReflectsFocusedPanel(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	view1 := m.View()

	// Move focus to next panel
	m.FocusNext()
	view2 := m.View()

	// Views should be different if focus changed
	if view1 == view2 && !containsString(view1, "1") {
		t.Error("expected view to change when focus changed or contain focus indicator")
	}
}

// Helper function for string contains
func containsString(haystack, needle string) bool {
	for i := 0; i < len(haystack)-len(needle)+1; i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestModel_SwitchViewWithNumberKey(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	// Start with Dashboard view
	if m.currentView != ViewDashboard {
		t.Errorf("expected initial view to be Dashboard, got %v", m.currentView)
	}

	// Send number key '2' to switch to next view
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}
	model, _ := m.Update(msg)
	m = model.(*Model)

	// Should switch to Queue view
	if m.currentView != ViewQueue {
		t.Errorf("expected view to be Queue after '2' key, got %v", m.currentView)
	}

	// Send number key '1' to switch back to Dashboard
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}
	model, _ = m.Update(msg)
	m = model.(*Model)

	if m.currentView != ViewDashboard {
		t.Errorf("expected view to be Dashboard after '1' key, got %v", m.currentView)
	}
}
