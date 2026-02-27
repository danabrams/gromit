package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Model implements the bubbletea.Model interface for the Gromit TUI.
type Model struct {
	store *Store
}

// NewModel creates a new TUI model with the given store.
func NewModel(store *Store) *Model {
	return &Model{
		store: store,
	}
}

// Init initializes the model.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

// View renders the model to a string.
func (m *Model) View() string {
	return "Gromit TUI"
}
