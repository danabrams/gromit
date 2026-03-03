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

type identifierProvider interface {
	Identifier() string
}

type pipelineRefreshedMsg struct {
	RequestedTab Tab
}

// handleAction routes pipeline tab keystrokes to their respective behaviors.
func handleAction(m *Model, key string, activeTab Tab, selectedItem ListItem, store *Store) (tea.Model, tea.Cmd) {
	if m == nil {
		m = &Model{}
	}

	switch key {
	case "r":
		m.pendingAction = buildPendingAction("refine", extractIdentifier(selectedItem))
		return m, tea.Quit
	case "p":
		m.pendingAction = buildPendingAction("plan", extractIdentifier(selectedItem))
		return m, tea.Quit
	case "d":
		m.pendingAction = buildPendingAction("decompose", extractIdentifier(selectedItem))
		return m, tea.Quit
	case "R":
		return m, refreshPipelineCmd(activeTab)
	default:
		return m, nil
	}
}

func extractIdentifier(item ListItem) string {
	if item == nil {
		return ""
	}
	if provider, ok := item.(identifierProvider); ok {
		return provider.Identifier()
	}
	return ""
}

func buildPendingAction(command, target string) *PendingAction {
	args := []string{}
	if target != "" {
		args = append(args, target)
	}
	return &PendingAction{
		Command: command,
		Args:    args,
	}
}

func refreshPipelineCmd(active Tab) tea.Cmd {
	return func() tea.Msg {
		return pipelineRefreshedMsg{
			RequestedTab: active,
		}
	}
}
