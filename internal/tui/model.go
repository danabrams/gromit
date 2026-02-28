package tui

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

// View constants for the TUI
const (
	ViewDashboard    = "dashboard"
	ViewQueue        = "queue"
	ViewConversation = "conversation"
)

// Model implements the bubbletea.Model interface for the Gromit TUI.
type Model struct {
	store        *Store
	currentView  string
	focusedPanel int
	scrollOffset int
	conversation *ConversationController
}

// NewModel creates a new TUI model with the given store.
func NewModel(store *Store) *Model {
	return &Model{
		store:       store,
		currentView: ViewDashboard,
	}
}

// SetConversationController attaches a controller whose view is shown in the conversation panel.
func (m *Model) SetConversationController(ctrl *ConversationController) {
	m.conversation = ctrl
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
	if m.currentView == ViewConversation && m.conversation != nil {
		return m.conversation.Init()
	}
	return nil
}

// Update handles messages and updates the model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// When in conversation view, forward messages to the conversation controller
	if m.currentView == ViewConversation && m.conversation != nil {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			model, cmd := m.conversation.Update(msg)
			if ctrl, ok := model.(*ConversationController); ok {
				m.conversation = ctrl
			}
			return m, cmd
		case conversationEventMsg:
			// Apply conversation event to store
			if m.store != nil {
				m.store.ApplyConversationEvent(msg.Event)
			}
			model, cmd := m.conversation.Update(msg)
			if ctrl, ok := model.(*ConversationController); ok {
				m.conversation = ctrl
			}
			return m, cmd
		}
	}

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
			if len(msg.Runes) > 0 {
				switch msg.Runes[0] {
				case 'q':
					return m, tea.Quit
				case '1':
					m.SwitchView(ViewDashboard)
					m.focusedPanel = 0 // Reset focus when switching views
				case '2':
					m.SwitchView(ViewQueue)
					m.focusedPanel = 0 // Reset focus when switching views
				case '3':
					m.SwitchView(ViewConversation)
					m.focusedPanel = 0 // Reset focus when switching views
					if m.conversation != nil {
						return m, m.conversation.Init()
					}
				}
			}
		}
	}
	return m, nil
}

// View renders the model to a string.
func (m *Model) View() string {
	switch m.currentView {
	case ViewQueue:
		return m.renderQueueView()
	case ViewConversation:
		return m.renderConversationView()
	default:
		return m.renderDashboardView()
	}
}

func (m *Model) renderConversationView() string {
	if m.conversation == nil {
		return "(no conversation data)\n"
	}
	return m.conversation.View()
}

func (m *Model) renderDashboardView() string {
	var output string

	// Panel 0: Progress panel
	progressFocus := ""
	if m.focusedPanel == 0 {
		progressFocus = " [FOCUSED]"
	}
	output += "=== Progress Panel" + progressFocus + " ===\n"
	output += "panel 0\n"
	if m.store != nil {
		if progress := m.store.RunProgressSnapshot(); progress != nil {
			output += "progress: " + strconv.Itoa(progress.CurrentIteration) + "/" + strconv.Itoa(progress.MaxIterations) + "\n"
		}
	}
	output += "\n"

	// Panel 1: Status panel
	statusFocus := ""
	if m.focusedPanel == 1 {
		statusFocus = " [FOCUSED]"
	}
	output += "=== Status Panel" + statusFocus + " ===\n"
	output += "panel 1\n"
	output += "Status information\n"

	return output
}

func (m *Model) renderQueueView() string {
	var output string

	// Panel 0: Ready beads
	readyFocus := ""
	if m.focusedPanel == 0 {
		readyFocus = " [FOCUSED]"
	}
	output += "=== Ready Beads" + readyFocus + " ===\n"
	output += "panel 0\n"
	output += "Queue beads\n"

	return output
}
