package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danabrams/gromit/internal/conversation"
)

type conversationEventMsg struct {
	Event conversation.Event
}

type conversationDoneMsg struct{}

// ConversationController renders a Bubble Tea model for streaming conversation state.
type ConversationController struct {
	session           conversation.Session
	events            []conversation.Event
	waitingForTool    bool
	cancelled         bool
	ignoredLateEvents int
	followUpProvider  func() string
}

// ConversationControllerOption customizes controller behavior.
type ConversationControllerOption func(*ConversationController)

// WithFollowUpProvider overrides the prompt provider used when requesting follow-ups.
func WithFollowUpProvider(provider func() string) ConversationControllerOption {
	return func(c *ConversationController) {
		if provider != nil {
			c.followUpProvider = provider
		}
	}
}

// NewConversationController builds a controller wired to the provided session.
func NewConversationController(session conversation.Session, opts ...ConversationControllerOption) *ConversationController {
	ctrl := &ConversationController{
		session:          session,
		followUpProvider: defaultFollowUpProvider,
	}
	for _, opt := range opts {
		opt(ctrl)
	}
	return ctrl
}

func defaultFollowUpProvider() string { return "" }

func (c *ConversationController) Init() tea.Cmd {
	return c.watchSessionCmd()
}

func (c *ConversationController) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case conversationEventMsg:
		if c.cancelled {
			c.ignoredLateEvents++
			return c, c.watchSessionCmd()
		}
		c.events = append(c.events, msg.Event)
		c.waitingForTool = msg.Event.Type == conversation.EventTypeToolWait
		if msg.Event.Type == conversation.EventTypeToolResult || msg.Event.Type == conversation.EventTypeDone {
			c.waitingForTool = false
		}
		if msg.Event.Type == conversation.EventTypeDone {
			return c, nil
		}
		return c, c.watchSessionCmd()
	case conversationDoneMsg:
		return c, nil
	case tea.KeyMsg:
		c.handleKey(msg)
		return c, c.watchSessionCmd()
	}
	return c, nil
}

func (c *ConversationController) View() string {
	var b strings.Builder
	b.WriteString("Conversation\n")
	for _, ev := range c.events {
		b.WriteString("- ")
		b.WriteString(ev.Type.String())
		if ev.Text != "" {
			b.WriteString(": ")
			b.WriteString(ev.Text)
		}
		if ev.ToolName != "" {
			b.WriteString(" [")
			b.WriteString(ev.ToolName)
			b.WriteString("]")
		}
		b.WriteRune('\n')
	}
	if c.waitingForTool {
		b.WriteString("[waiting for tool]\n")
	}
	if c.ignoredLateEvents > 0 {
		plural := "events"
		if c.ignoredLateEvents == 1 {
			plural = "event"
		}
		b.WriteString(fmt.Sprintf("[ignored %d late %s]\n", c.ignoredLateEvents, plural))
	}
	if c.cancelled {
		b.WriteString("[cancelled]\n")
	}
	return b.String()
}

func (c *ConversationController) watchSessionCmd() tea.Cmd {
	return func() tea.Msg {
		if c == nil || c.session == nil {
			return conversationDoneMsg{}
		}
		ev, ok := <-c.session.Events()
		if !ok {
			return conversationDoneMsg{}
		}
		return MapConversationEventToMsg(ev)
	}
}

func (c *ConversationController) handleKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "ctrl+c", "q":
		c.cancelSession()
	case "f":
		c.requestFollowUp()
	}
}

func (c *ConversationController) cancelSession() {
	if c.session != nil {
		c.session.Cancel()
	}
	c.cancelled = true
}

func (c *ConversationController) requestFollowUp() {
	if !c.waitingForTool || c.session == nil {
		return
	}
	prompt := c.followUpProvider()
	c.session.FollowUp(prompt)
}
