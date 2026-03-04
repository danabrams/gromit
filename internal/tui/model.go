package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// View constants for the TUI
const (
	ViewDashboard    = "dashboard"
	ViewQueue        = "queue"
	ViewConversation = "conversation"
)

type pipelineListModel interface {
	SetItems([]ListItem)
}

// Model implements the bubbletea.Model interface for the Gromit TUI.
type Model struct {
	store              *Store
	currentView        string
	activeTab          Tab
	focusedPanel       int
	scrollOffset       int
	conversation       *ConversationController
	pendingAction      *PendingAction
	detailView         bool
	confirmDelete      bool
	pipelineListModels []pipelineListModel
}

// NewModel creates a new TUI model with the given store.
func NewModel(store *Store) *Model {
	return &Model{
		store:       store,
		currentView: ViewDashboard,
		activeTab:   TabRunLoop,
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

// NextTab advances the active top-level tab, wrapping around the tab list.
func (m *Model) NextTab() {
	if m == nil {
		return
	}
	m.activeTab = nextTab(m.activeTab)
}

// PrevTab moves the active top-level tab backward, wrapping around the tab list.
func (m *Model) PrevTab() {
	if m == nil {
		return
	}
	m.activeTab = prevTab(m.activeTab)
}

// PendingAction returns the action that should be executed after the TUI exits.
func (m *Model) PendingAction() *PendingAction {
	if m == nil {
		return nil
	}
	return m.pendingAction
}

func (m *Model) registerPipelineListModel(list pipelineListModel) {
	if m == nil || list == nil {
		return
	}
	m.pipelineListModels = append(m.pipelineListModels, list)
}

type pipelineListNavigator interface {
	CursorUp()
	CursorDown()
}

func (m *Model) forEachPipelineNavigator(fn func(pipelineListNavigator)) {
	if m == nil || fn == nil {
		return
	}
	for _, list := range m.pipelineListModels {
		if nav, ok := list.(pipelineListNavigator); ok && nav != nil {
			fn(nav)
		}
	}
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
		case tea.KeyLeft:
			m.PrevTab()
		case tea.KeyRight:
			m.NextTab()
		case tea.KeyUp:
			m.forEachPipelineNavigator(func(nav pipelineListNavigator) {
				nav.CursorUp()
			})
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case tea.KeyDown:
			m.forEachPipelineNavigator(func(nav pipelineListNavigator) {
				nav.CursorDown()
			})
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
				case '2':
					m.SwitchView(ViewQueue)
				case '3':
					m.SwitchView(ViewConversation)
					if m.conversation != nil {
						return m, m.conversation.Init()
					}
				case 'v':
					m.detailView = !m.detailView
				}
			}
		}
	case pipelineRefreshedMsg:
		items := m.pipelineListItemsForTab(msg.RequestedTab)
		for _, list := range m.pipelineListModels {
			list.SetItems(items)
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
	return RenderDashboardView(m.store, m.focusedPanel)
}

func (m *Model) renderQueueView() string {
	return RenderQueueView(m.store, m.focusedPanel)
}

func (m *Model) pipelineListItemsForTab(tab Tab) []ListItem {
	return nil
}
