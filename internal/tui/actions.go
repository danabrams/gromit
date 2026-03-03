package tui

import tea "github.com/charmbracelet/bubbletea"

// PendingAction describes a CLI command that should be executed after the TUI quits.
type PendingAction struct {
	Command string
	Args    []string
}

// Tab identifies a top-level pipeline tab in the TUI.
type Tab string

// ListItem is used to abstract items displayed in the pipeline tabs.
type ListItem interface {
	Title() string
	Summary() string
}

// handleAction will eventually route pipeline tab keystrokes to their respective behaviors.
// For now it is a placeholder that maintains the intended signature.
func handleAction(key string, activeTab Tab, selectedItem ListItem, store *Store) (tea.Model, tea.Cmd) {
	_ = key
	_ = activeTab
	_ = selectedItem
	_ = store
	return nil, nil
}
