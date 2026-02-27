package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// View constants for the TUI
const (
	ViewDashboard = "dashboard"
	ViewQueue     = "queue"
)

// Model implements the bubbletea.Model interface for the Gromit TUI.
type Model struct {
	store        *Store
	currentView  string
	focusedPanel int
	scrollOffset int
}

// NewModel creates a new TUI model with the given store.
func NewModel(store *Store) *Model {
	return &Model{
		store:       store,
		currentView: ViewDashboard,
	}
}

// SwitchView switches the current view to the specified view.
func (m *Model) SwitchView(view string) {
	m.currentView = view
}

// FocusNext moves focus to the next panel.
func (m *Model) FocusNext() {
	m.focusedPanel = (m.focusedPanel + 1) % 2
}

// FocusPrev moves focus to the previous panel.
func (m *Model) FocusPrev() {
	m.focusedPanel = (m.focusedPanel - 1 + 2) % 2
}

// Init initializes the model.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab:
			m.FocusNext()
		case tea.KeyShiftTab:
			m.FocusPrev()
		case tea.KeyUp:
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case tea.KeyDown:
			m.scrollOffset++
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyRunes:
			if len(msg.Runes) > 0 && msg.Runes[0] == 'q' {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

// View renders the model to a string.
func (m *Model) View() string {
	return "Gromit TUI"
}
