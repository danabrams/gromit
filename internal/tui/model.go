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
	Selected() ListItem
	pipelineListNavigator
}

// Model implements the bubbletea.Model interface for the Gromit TUI.
type Model struct {
	store              *Store
	currentView        string
	activeTab          Tab
	runLoopSubView     string
	focusedPanel       int
	scrollOffset       int
	conversation       *ConversationController
	pendingAction      *PendingAction
	detailView         bool
	confirmDelete      bool
	pipelineListModels map[Tab]pipelineListModel
}

// NewModel creates a new TUI model with the given store.
func NewModel(store *Store) *Model {
	return &Model{
		store:          store,
		currentView:    ViewDashboard,
		activeTab:      TabRunLoop,
		runLoopSubView: ViewDashboard,
	}
}

// SetConversationController attaches a controller whose view is shown in the conversation panel.
func (m *Model) SetConversationController(ctrl *ConversationController) {
	m.conversation = ctrl
}

// SwitchView switches the current view to the specified view.
func (m *Model) SwitchView(view string) {
	m.currentView = view
	m.runLoopSubView = view
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

func (m *Model) registerPipelineListModel(tab Tab, list pipelineListModel) {
	if m == nil || list == nil {
		return
	}
	if m.pipelineListModels == nil {
		m.pipelineListModels = make(map[Tab]pipelineListModel)
	}
	m.pipelineListModels[tab] = list
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
		if list == nil {
			continue
		}
		fn(list)
	}
}

func (m *Model) activePipelineListModel() pipelineListModel {
	if m == nil || m.pipelineListModels == nil {
		return nil
	}
	list, ok := m.pipelineListModels[m.activeTab]
	if !ok {
		return nil
	}
	return list
}

func (m *Model) activePipelineNavigator() pipelineListNavigator {
	return m.activePipelineListModel()
}

func (m *Model) shouldRoutePipelineActions() bool {
	if m == nil {
		return false
	}
	return isPipelineTab(m.activeTab)
}

func (m *Model) handlePipelineActionRune(key rune) (tea.Model, tea.Cmd, bool) {
	if !m.shouldRoutePipelineActions() {
		return m, nil, false
	}
	var selected ListItem
	if list := m.activePipelineListModel(); list != nil {
		selected = list.Selected()
	}
	updatedModel, cmd := handleAction(m, string(key), m.activeTab, selected, m.store)
	if updated, ok := updatedModel.(*Model); ok && updated != nil {
		m = updated
	}
	return m, cmd, true
}

func (m *Model) handleRunLoopNavigationKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyLeft:
		m.PrevTab()
		return nil, true
	case tea.KeyRight:
		m.NextTab()
		return nil, true
	case tea.KeyRunes:
		if len(msg.Runes) == 0 {
			return nil, false
		}
		key := msg.Runes[0]
		switch key {
		case '1', '2', '3':
			if m.activeTab != TabRunLoop {
				return nil, true
			}
			switch key {
			case '1':
				m.SwitchView(ViewDashboard)
				return nil, true
			case '2':
				m.SwitchView(ViewQueue)
				return nil, true
			case '3':
				m.SwitchView(ViewConversation)
				if m.conversation != nil {
					return m.conversation.Init(), true
				}
				return nil, true
			}
		}
	}
	return nil, false
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
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if cmd, handled := m.handleRunLoopNavigationKey(keyMsg); handled {
			return m, cmd
		}
	}

	if m.currentView == ViewConversation && m.conversation != nil {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			model, cmd := m.conversation.Update(msg)
			if ctrl, ok := model.(*ConversationController); ok {
				m.conversation = ctrl
			}
			return m, cmd
		case conversationEventMsg:
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
			if nav := m.activePipelineNavigator(); nav != nil {
				nav.CursorUp()
			}
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case tea.KeyDown:
			if nav := m.activePipelineNavigator(); nav != nil {
				nav.CursorDown()
			}
			m.scrollOffset++
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			if m.detailView {
				m.detailView = false
				return m, nil
			}
			if m.confirmDelete {
				m.confirmDelete = false
				return m, nil
			}
		case tea.KeyRunes:
			if len(msg.Runes) == 0 {
				break
			}
			key := msg.Runes[0]
			switch key {
			case 'q':
				return m, tea.Quit
			case 'v':
				m.detailView = !m.detailView
			default:
				if updatedModel, cmd, handled := m.handlePipelineActionRune(key); handled {
					return updatedModel, cmd
				}
			}
		}
	case pipelineRefreshedMsg:
		if m.pipelineListModels == nil {
			break
		}
		requestedTab := msg.RequestedTab
		if requestedTab == "" {
			for tab, list := range m.pipelineListModels {
				if list == nil {
					continue
				}
				list.SetItems(m.pipelineListItemsForTab(tab))
			}
			break
		}
		if list := m.pipelineListModels[requestedTab]; list != nil {
			list.SetItems(m.pipelineListItemsForTab(requestedTab))
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
	if m == nil || m.store == nil {
		return nil
	}
	items := m.store.GetPipelineItems()
	switch tab {
	case TabBacklog:
		list := make([]ListItem, 0, len(items.BacklogIdeas))
		for i := range items.BacklogIdeas {
			list = append(list, &IdeaListItem{idea: &items.BacklogIdeas[i]})
		}
		return list
	case TabSpecs:
		list := make([]ListItem, 0, len(items.UnplannedSpecs))
		for i := range items.UnplannedSpecs {
			list = append(list, &SpecListItem{path: items.UnplannedSpecs[i]})
		}
		return list
	case TabPlans:
		list := make([]ListItem, 0, len(items.UndecomposedPlans))
		for i := range items.UndecomposedPlans {
			list = append(list, &PlanListItem{path: items.UndecomposedPlans[i]})
		}
		return list
	case TabQueue:
		list := make([]ListItem, 0, len(items.Beads))
		for i := range items.Beads {
			list = append(list, &BeadListItem{bead: &items.Beads[i]})
		}
		return list
	default:
		return nil
	}
}

func isPipelineTab(tab Tab) bool {
	switch tab {
	case TabBacklog, TabSpecs, TabPlans, TabQueue:
		return true
	default:
		return false
	}
}
