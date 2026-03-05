package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/conversation"
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

func TestModel_ArrowKeysNavigateTabs(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	if m.activeTab != TabRunLoop {
		t.Fatalf("expected initial active tab to be run loop, got %q", m.activeTab)
	}

	rightSequence := []struct {
		key  tea.KeyType
		want Tab
	}{
		{tea.KeyRight, TabBacklog},
		{tea.KeyRight, TabSpecs},
		{tea.KeyRight, TabPlans},
		{tea.KeyRight, TabQueue},
		{tea.KeyRight, TabRunLoop},
	}

	for _, tc := range rightSequence {
		msg := tea.KeyMsg{Type: tc.key}
		m.Update(msg)
		if m.activeTab != tc.want {
			t.Fatalf("after %s, expected active tab %q, got %q", tc.key, tc.want, m.activeTab)
		}
	}

	msg := tea.KeyMsg{Type: tea.KeyLeft}
	m.Update(msg)
	if m.activeTab != TabQueue {
		t.Fatalf("after left arrow, expected active tab queue, got %q", m.activeTab)
	}
}

func TestModel_ScrollHandlingUp(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	m.scrollOffset = 5

	list := &mockPipelineListModel{}
	m.registerPipelineListModel(TabBacklog, list)
	m.activeTab = TabBacklog

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

	list := &mockPipelineListModel{}
	m.registerPipelineListModel(TabBacklog, list)
	m.activeTab = TabBacklog

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

	list := &mockPipelineListModel{}
	m.registerPipelineListModel(TabBacklog, list)
	m.activeTab = TabBacklog

	// Send Up key - should not go below 0
	msg := tea.KeyMsg{Type: tea.KeyUp}
	model, _ := m.Update(msg)
	m = model.(*Model)

	// Scroll offset should stay at 0
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset to be 0 (clamped), got %d", m.scrollOffset)
	}
}

func TestModel_ScrollIgnoredWhenPipelineTabInactive(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	m.scrollOffset = 2

	if _, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp}); m.scrollOffset != 2 {
		t.Fatalf("expected scrollOffset to remain 2 when active tab %q is inactive, got %d", m.activeTab, m.scrollOffset)
	}

	m.scrollOffset = 2
	if _, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}); m.scrollOffset != 2 {
		t.Fatalf("expected scrollOffset to remain 2 when active tab %q is inactive, got %d", m.activeTab, m.scrollOffset)
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

func TestModel_ToggleDetailView(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	if m.detailView {
		t.Fatalf("expected detail view to start disabled")
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}}
	model, _ := m.Update(msg)
	m = model.(*Model)
	if !m.detailView {
		t.Fatalf("expected detail view to be enabled after 'v' key")
	}

	model, _ = m.Update(msg)
	m = model.(*Model)
	if m.detailView {
		t.Fatalf("expected detail view to be disabled after second 'v' key")
	}
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

	if !strings.Contains(view, "=== Progress Panel") {
		t.Error("expected dashboard view to contain progress panel header")
	}
	if !strings.Contains(view, "Queue depth:") {
		t.Error("expected dashboard view to describe queue depth")
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

func TestModel_ProgressDisplaysIterationNumbersAsDecimalStrings(t *testing.T) {
	store := &Store{
		Dashboard: DashboardState{
			RunProgress: &RunProgress{
				CurrentIteration: 5,
				MaxIterations:    10,
				Status:           "running",
			},
		},
	}
	m := NewModel(store)

	// Render the dashboard view
	view := m.View()

	// The view should contain "5/10" as a decimal string representation
	// not as Unicode control characters U+0005 and U+000A
	if !containsString(view, "5/10") {
		t.Errorf("expected dashboard view to contain '5/10', but got:\n%q", view)
	}
}

func TestModelConversationViewRendersSession(t *testing.T) {
	timeline := []conversation.FakeStep{
		{Event: conversation.Event{Type: conversation.EventTypeStream, Text: "hello"}},
		{Event: conversation.Event{Type: conversation.EventTypeDone}},
	}
	session := conversation.NewFakeSession(timeline)
	controller := NewConversationController(session)

	cmd := controller.Init()
	for cmd != nil {
		msg := cmd()
		model, next := controller.Update(msg)
		var ok bool
		controller, ok = model.(*ConversationController)
		if !ok {
			t.Fatalf("expected ConversationController, got %T", model)
		}
		cmd = next
	}

	m := NewModel(&Store{})
	m.SetConversationController(controller)
	m.SwitchView(ViewConversation)

	view := m.View()
	if !containsString(view, "- stream: hello") {
		t.Fatalf("expected conversation view to include streamed text, got %q", view)
	}
}

func TestModelStartsConversationControllerWhenSwitchingToConversationView(t *testing.T) {
	timeline := []conversation.FakeStep{
		{Event: conversation.Event{Type: conversation.EventTypeStream, Text: "hello"}},
		{Event: conversation.Event{Type: conversation.EventTypeDone}},
	}
	session := conversation.NewFakeSession(timeline)
	controller := NewConversationController(session)

	store := &Store{}
	m := NewModel(store)
	m.SetConversationController(controller)
	if m.currentView == ViewConversation {
		t.Fatalf("expected model to start in dashboard view, got %v", m.currentView)
	}

	focusMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}}
	model, cmd := m.Update(focusMsg)
	m = model.(*Model)
	if cmd == nil {
		t.Fatal("expected conversation controller init command when focusing view")
	}

	for cmd != nil {
		msg := cmd()
		model, next := m.Update(msg)
		var ok bool
		m, ok = model.(*Model)
		if !ok {
			t.Fatalf("expected Model, got %T", model)
		}
		cmd = next
	}

	if store.Conversation.EventCount == 0 {
		t.Fatal("expected conversation events to be recorded after switching views")
	}
}

func TestModel_FocusConversationKeySwitchesToConversationView(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	// Start in Dashboard view
	if m.currentView != ViewDashboard {
		t.Errorf("expected initial view to be Dashboard, got %v", m.currentView)
	}

	// Send FocusConversation key ('3')
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}}
	model, _ := m.Update(msg)
	m = model.(*Model)

	// Should switch to Conversation view
	if m.currentView != ViewConversation {
		t.Errorf("expected view to be Conversation after '3' key, got %v", m.currentView)
	}
}

func TestModel_NumberKeysOnlyRunLoopActive(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	m.SwitchView(ViewDashboard)

	m.activeTab = TabBacklog
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}
	m.Update(msg)
	if m.currentView != ViewDashboard {
		t.Fatalf("expected view to stay Dashboard when active tab is %q, got %q", m.activeTab, m.currentView)
	}

	m.activeTab = TabRunLoop
	m.Update(msg)
	if m.currentView != ViewQueue {
		t.Fatalf("expected view to switch to Queue when run loop active, got %q", m.currentView)
	}
}

func TestModel_UsesKeymapForConversationActions(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	timeline := []conversation.FakeStep{
		{Event: conversation.Event{Type: conversation.EventTypeStream, Text: "test"}},
		{Event: conversation.Event{Type: conversation.EventTypeDone}},
	}
	session := conversation.NewFakeSession(timeline)
	controller := NewConversationController(session)
	m.SetConversationController(controller)
	m.SwitchView(ViewConversation)

	// Get the keymap
	keymap := DefaultKeymap()

	// Test that CancelSession key (from keymap) is actually 'c'
	if keymap.CancelSession != "c" {
		t.Errorf("expected CancelSession to be 'c', got %q", keymap.CancelSession)
	}

	// Test that FocusConversation key (from keymap) is actually '3'
	if keymap.FocusConversation != "3" {
		t.Errorf("expected FocusConversation to be '3', got %q", keymap.FocusConversation)
	}

	// Test that model recognizes these keys
	m.SwitchView(ViewDashboard)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(keymap.FocusConversation[0])}}
	model, _ := m.Update(msg)
	m = model.(*Model)

	if m.currentView != ViewConversation {
		t.Errorf("expected FocusConversation key to switch to conversation view, got %v", m.currentView)
	}
}

func TestModel_RunLoopSubViewKeysOnlyHandledWhenRunLoopActive(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	// When active tab is not the run loop, the key handler should not report the key as handled.
	m.activeTab = TabBacklog
	cmd, handled := m.handleRunLoopNavigationKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if handled {
		t.Fatalf("expected run loop sub-view keys not to be handled when active tab is %q", m.activeTab)
	}
	if cmd != nil {
		t.Fatalf("expected no command when key is not handled, got %T", cmd)
	}

	// When run loop tab is active, the handler should intercept the key.
	m.activeTab = TabRunLoop
	cmd, handled = m.handleRunLoopNavigationKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if !handled {
		t.Fatalf("expected run loop sub-view keys to be handled when active tab is run loop")
	}
	if cmd != nil {
		t.Fatalf("expected no command from run loop sub-view key, got %T", cmd)
	}
}

func TestModel_RunLoopKeysSkipPipelineActionsWhenInactive(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	list := &countingPipelineList{}
	m.registerPipelineListModel(TabBacklog, list)
	m.activeTab = TabBacklog

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}
	m.Update(msg)

	if list.selectedCalls != 0 {
		t.Fatalf("expected pipeline list not to be consulted for run loop key when inactive, got %d calls", list.selectedCalls)
	}
}

type countingPipelineList struct {
	selectedCalls int
}

func (c *countingPipelineList) SetItems([]ListItem) {}

func (c *countingPipelineList) Selected() ListItem {
	c.selectedCalls++
	return nil
}

func (c *countingPipelineList) CursorUp()   {}
func (c *countingPipelineList) CursorDown() {}

func TestModel_AppliesConversationEventsToStore(t *testing.T) {
	timeline := []conversation.FakeStep{
		{Event: conversation.Event{Type: conversation.EventTypeStream, Text: "streamed text"}},
		{Event: conversation.Event{Type: conversation.EventTypeDone}},
	}
	session := conversation.NewFakeSession(timeline)
	controller := NewConversationController(session)

	store := &Store{}
	m := NewModel(store)
	m.SetConversationController(controller)
	m.SwitchView(ViewConversation)

	// Pump events through the model's Update method
	cmd := m.Init()
	for cmd != nil {
		msg := cmd()
		model, next := m.Update(msg)
		var ok bool
		m, ok = model.(*Model)
		if !ok {
			t.Fatalf("expected Model, got %T", model)
		}
		cmd = next
	}

	// Check that store has recorded conversation events
	if store.Conversation.EventCount == 0 {
		t.Error("expected store to record conversation events")
	}
	if store.Conversation.LastEvent == nil {
		t.Error("expected LastEvent to be recorded")
	}
}

func TestModel_ForwardsKeysToConversationControllerWhenInConversationView(t *testing.T) {
	timeline := []conversation.FakeStep{
		{Event: conversation.Event{Type: conversation.EventTypeStream, Text: "response"}},
		{Event: conversation.Event{Type: conversation.EventTypeDone}},
	}
	session := conversation.NewFakeSession(timeline)
	controller := NewConversationController(session)

	cmd := controller.Init()
	for cmd != nil {
		msg := cmd()
		model, next := controller.Update(msg)
		var ok bool
		controller, ok = model.(*ConversationController)
		if !ok {
			t.Fatalf("expected ConversationController, got %T", model)
		}
		cmd = next
	}

	store := &Store{}
	m := NewModel(store)
	m.SetConversationController(controller)
	m.SwitchView(ViewConversation)

	// Send ctrl+c key to the model while in conversation view
	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	model, _ := m.Update(keyMsg)
	m = model.(*Model)

	// The conversation view should contain "[cancelled]" after ctrl+c key
	view := m.View()
	if !containsString(view, "[cancelled]") {
		t.Errorf("expected conversation view to contain '[cancelled]', got: %q", view)
	}
}

func TestModel_ArrowKeysNavigateTabsDuringConversationView(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	timeline := []conversation.FakeStep{
		{Event: conversation.Event{Type: conversation.EventTypeDone}},
	}
	session := conversation.NewFakeSession(timeline)
	controller := NewConversationController(session)
	m.SetConversationController(controller)
	m.SwitchView(ViewConversation)

	if m.activeTab != TabRunLoop {
		t.Fatalf("expected active tab run loop before navigation, got %q", m.activeTab)
	}

	if _, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}); m.activeTab != TabBacklog {
		t.Fatalf("expected active tab to advance to backlog while in conversation view, got %q", m.activeTab)
	}

	if _, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft}); m.activeTab != TabRunLoop {
		t.Fatalf("expected active tab to move back to run loop after left arrow, got %q", m.activeTab)
	}
}

func TestModel_TabNavigationKeepsCursorAndRendersPipelineData(t *testing.T) {
	store, ready := newPipelineStore(t)
	m := NewModel(store)
	m.focusedPanel = 1

	queueList := &mockPipelineListModel{}
	m.registerPipelineListModel(TabQueue, queueList)

	view := m.View()
	if !strings.Contains(view, "Iteration 2/5") {
		t.Fatalf("expected dashboard view to show iteration progress, got %q", view)
	}
	if !strings.Contains(view, "Queue depth: ready=1 blocked=1 stuck=1") {
		t.Fatalf("expected dashboard view to include queue depth, got %q", view)
	}

	queueMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}
	model, _ := m.Update(queueMsg)
	m = model.(*Model)
	queueView := m.View()
	if !strings.Contains(queueView, ready.Title) {
		t.Fatalf("expected queue view to render ready bead title, got %q", queueView)
	}

	m.activeTab = TabQueue

	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	model, _ = m.Update(downMsg)
	m = model.(*Model)
	if m.scrollOffset != 1 {
		t.Fatalf("expected scroll offset to increment, got %d", m.scrollOffset)
	}

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = model.(*Model)
	if m.focusedPanel != 1 {
		t.Fatalf("expected focus to remain on status panel, got panel %d", m.focusedPanel)
	}

	model, _ = m.Update(queueMsg)
	m = model.(*Model)
	if m.scrollOffset != 1 {
		t.Fatalf("expected scroll offset to persist after switching tabs, got %d", m.scrollOffset)
	}
	queueView = m.View()
	if !strings.Contains(queueView, ready.Title) {
		t.Fatalf("expected queue view to still render ready bead title, got %q", queueView)
	}
}

func TestModel_ActionDispatchConversationViewUsesPipelineStore(t *testing.T) {
	store, ready := newPipelineStore(t)
	timeline := []conversation.FakeStep{
		{Event: conversation.Event{Type: conversation.EventTypeStream, Text: "hello"}},
		{Event: conversation.Event{Type: conversation.EventTypeDone}},
	}
	session := conversation.NewFakeSession(timeline)
	controller := NewConversationController(session)

	m := NewModel(store)
	m.SetConversationController(controller)

	focusMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}}
	model, cmd := m.Update(focusMsg)
	m = model.(*Model)
	if cmd == nil {
		t.Fatalf("expected conversation init command, got nil")
	}
	for cmd != nil {
		msg := cmd()
		model, next := m.Update(msg)
		var ok bool
		m, ok = model.(*Model)
		if !ok {
			t.Fatalf("expected Model, got %T", model)
		}
		cmd = next
	}

	if store.Conversation.EventCount == 0 {
		t.Fatal("expected conversation events to be recorded")
	}
	if !strings.Contains(m.View(), "hello") {
		t.Fatalf("expected conversation view to include transcript text, got %q", m.View())
	}

	m.SwitchView(ViewQueue)
	queueView := m.View()
	if !strings.Contains(queueView, ready.Title) {
		t.Fatalf("expected pipeline queue view still to show ready bead title, got %q", queueView)
	}
}

func TestModel_RunLoopSubViewKeysWorkFromConversationView(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	timeline := []conversation.FakeStep{
		{Event: conversation.Event{Type: conversation.EventTypeDone}},
	}
	session := conversation.NewFakeSession(timeline)
	controller := NewConversationController(session)
	m.SetConversationController(controller)
	m.SwitchView(ViewConversation)

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = model.(*Model)
	if m.currentView != ViewDashboard {
		t.Fatalf("expected dashboard view after pressing '1' in conversation view, got %v", m.currentView)
	}
}

func newPipelineStore(t *testing.T) (*Store, *bead.Bead) {
	t.Helper()

	ready := &bead.Bead{
		ID:    "ready-1",
		Title: "Ready Bead",
	}
	blocked := &bead.Bead{
		ID:     "blocked-1",
		Title:  "Blocked Bead",
		Parent: "parent-1",
	}
	stuck := &bead.Bead{
		ID:    "stuck-1",
		Title: "Stuck Bead",
	}

	store := &Store{
		Dashboard: DashboardState{
			RunProgress: &RunProgress{
				CurrentIteration: 2,
				MaxIterations:    5,
				Status:           "running",
			},
		},
		Queue: QueueState{
			Snapshot: &QueueSnapshot{
				Ready:   []*bead.Bead{ready},
				Blocked: []*bead.Bead{blocked},
				Stuck:   []*bead.Bead{stuck},
				All:     []*bead.Bead{ready, blocked, stuck},
			},
		},
	}

	return store, ready
}

type mockPipelineListModel struct {
	called          int
	items           []ListItem
	cursorUpCalls   int
	cursorDownCalls int
	selected        ListItem
}

func (m *mockPipelineListModel) SetItems(items []ListItem) {
	m.called++
	m.items = items
}

func (m *mockPipelineListModel) CursorUp() {
	m.cursorUpCalls++
}

func (m *mockPipelineListModel) CursorDown() {
	m.cursorDownCalls++
}

func (m *mockPipelineListModel) Selected() ListItem {
	return m.selected
}

type testActionListItem struct {
	id string
}

func (t *testActionListItem) Title() string {
	return "action item"
}

func (t *testActionListItem) Summary() string {
	return "selected for action"
}

func (t *testActionListItem) Identifier() string {
	return t.id
}

func TestModel_PipelineRefreshUpdatesRequestedTab(t *testing.T) {
	store := &Store{}
	ideas := []backlog.Idea{
		{ID: "idea-alpha", Text: "Alpha idea"},
	}
	specs := []string{"spec-alpha", "spec-beta"}
	store.SetPipelineItems(PipelineItems{
		BacklogIdeas:   ideas,
		UnplannedSpecs: specs,
	})

	model := NewModel(store)
	backlog := &mockPipelineListModel{}
	specList := &mockPipelineListModel{}
	model.registerPipelineListModel(TabBacklog, backlog)
	model.registerPipelineListModel(TabSpecs, specList)

	model.Update(pipelineRefreshedMsg{RequestedTab: TabSpecs})

	if backlog.called != 0 {
		t.Fatalf("expected backlog list not to be updated when specs refresh requested, got %d", backlog.called)
	}
	if specList.called != 1 {
		t.Fatalf("expected specs list to be updated once, got %d", specList.called)
	}
	if len(backlog.items) != 0 {
		t.Fatalf("expected backlog items to remain unset, got %d", len(backlog.items))
	}
	if len(specList.items) != len(specs) {
		t.Fatalf("expected %d spec items, got %d", len(specs), len(specList.items))
	}
	for i, item := range specList.items {
		specItem, ok := item.(*SpecListItem)
		if !ok {
			t.Fatalf("expected SpecListItem, got %T", item)
		}
		if specItem.path != specs[i] {
			t.Fatalf("expected spec path %q, got %q", specs[i], specItem.path)
		}
	}
}

func TestModel_PipelineListNavigationRoutesToActiveList(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	list := &mockPipelineListModel{}
	m.registerPipelineListModel(TabBacklog, list)
	m.activeTab = TabBacklog

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyUp}); cmd != nil {
		t.Fatalf("unexpected command for up key: %v", cmd)
	}
	if list.cursorUpCalls != 1 {
		t.Fatalf("expected CursorUp to be called once, got %d", list.cursorUpCalls)
	}

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown}); cmd != nil {
		t.Fatalf("unexpected command for down key: %v", cmd)
	}
	if list.cursorDownCalls != 1 {
		t.Fatalf("expected CursorDown to be called once, got %d", list.cursorDownCalls)
	}
}

func TestModel_PipelineListNavigationTargetsActiveTab(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	backlog := &mockPipelineListModel{}
	specs := &mockPipelineListModel{}
	m.registerPipelineListModel(TabBacklog, backlog)
	m.registerPipelineListModel(TabSpecs, specs)

	m.activeTab = TabBacklog
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyUp}); cmd != nil {
		t.Fatalf("unexpected command for up key: %v", cmd)
	}
	if backlog.cursorUpCalls != 1 {
		t.Fatalf("expected backlog CursorUp to be called once, got %d", backlog.cursorUpCalls)
	}
	if specs.cursorUpCalls != 0 {
		t.Fatalf("expected specs CursorUp to be ignored, got %d", specs.cursorUpCalls)
	}

	m.activeTab = TabSpecs
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown}); cmd != nil {
		t.Fatalf("unexpected command for down key: %v", cmd)
	}
	if specs.cursorDownCalls != 1 {
		t.Fatalf("expected specs CursorDown to be called once, got %d", specs.cursorDownCalls)
	}
	if backlog.cursorDownCalls != 0 {
		t.Fatalf("expected backlog CursorDown to remain zero, got %d", backlog.cursorDownCalls)
	}
}

func TestModel_PipelineActionKeysInvokeHandleAction(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	item := &testActionListItem{id: "idea-9"}
	list := &mockPipelineListModel{selected: item}
	m.registerPipelineListModel(TabBacklog, list)
	m.activeTab = TabBacklog

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = model.(*Model)

	if cmd == nil {
		t.Fatalf("expected non-nil command from handleAction")
	}
	if m.pendingAction == nil {
		t.Fatalf("expected pending action to be set after handleAction")
	}
	if m.pendingAction.Command != "refine" {
		t.Fatalf("expected pending action command \"refine\", got %q", m.pendingAction.Command)
	}
	if len(m.pendingAction.Args) != 1 || m.pendingAction.Args[0] != item.Identifier() {
		t.Fatalf("expected pending action args [%q], got %+v", item.Identifier(), m.pendingAction.Args)
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected handleAction to return tea.QuitMsg, got %T", msg)
	}
}

func TestModel_PipelineActionKeysHandleOutsideDashboardView(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	m.currentView = ViewQueue

	item := &testActionListItem{id: "idea-42"}
	list := &mockPipelineListModel{selected: item}
	m.registerPipelineListModel(TabBacklog, list)
	m.activeTab = TabBacklog

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = model.(*Model)

	if cmd == nil {
		t.Fatalf("expected non-nil command from handleAction")
	}
	if m.pendingAction == nil {
		t.Fatalf("expected pending action to be set")
	}
	if m.pendingAction.Command != "refine" {
		t.Fatalf("expected pending action command \"refine\", got %q", m.pendingAction.Command)
	}
	if len(m.pendingAction.Args) != 1 || m.pendingAction.Args[0] != item.Identifier() {
		t.Fatalf("expected pending action args [%q], got %+v", item.Identifier(), m.pendingAction.Args)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected handleAction to emit tea.QuitMsg, got %T", cmd())
	}
}

func TestModel_EscapeClosesDetailView(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	m.detailView = true
	m.activeTab = TabBacklog

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(*Model)

	if m.detailView {
		t.Fatalf("expected detail view to close after Escape")
	}
}

func TestModel_EscapeCancelsConfirmDelete(t *testing.T) {
	store := &Store{}
	m := NewModel(store)
	m.activeTab = TabBacklog
	m.confirmDelete = true

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(*Model)

	if m.confirmDelete {
		t.Fatalf("expected confirmDelete to be cleared after Escape")
	}
}

func TestModel_RunLoopSubViewInitializesToDashboard(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	// runLoopSubView should be initialized to ViewDashboard
	if m.runLoopSubView != ViewDashboard {
		t.Errorf("expected initial runLoopSubView to be Dashboard, got %v", m.runLoopSubView)
	}
}

func TestModel_SwitchViewUpdatesRunLoopSubView(t *testing.T) {
	store := &Store{}
	m := NewModel(store)

	// Switch to Queue view
	m.SwitchView(ViewQueue)
	if m.runLoopSubView != ViewQueue {
		t.Errorf("expected runLoopSubView to be Queue after SwitchView, got %v", m.runLoopSubView)
	}

	// Switch to Conversation view
	m.SwitchView(ViewConversation)
	if m.runLoopSubView != ViewConversation {
		t.Errorf("expected runLoopSubView to be Conversation after SwitchView, got %v", m.runLoopSubView)
	}
}
