package tui

import (
    "strings"

    tea "github.com/charmbracelet/bubbletea"
)

// ConversationEventType describes the kind of data emitted by a conversation session.
type ConversationEventType int

const (
    ConversationEventTypeStream ConversationEventType = iota
    ConversationEventTypeToolWait
    ConversationEventTypeToolResult
    ConversationEventTypeDone
)

func (t ConversationEventType) String() string {
    switch t {
    case ConversationEventTypeStream:
        return "stream"
    case ConversationEventTypeToolWait:
        return "tool wait"
    case ConversationEventTypeToolResult:
        return "tool result"
    case ConversationEventTypeDone:
        return "done"
    default:
        return "unknown"
    }
}

// ConversationEvent represents a discrete update emitted by a conversation session.
type ConversationEvent struct {
    Type     ConversationEventType
    Text     string
    ToolName string
}

// ConversationSession abstracts the streaming interface used by the UI.
type ConversationSession interface {
    Events() <-chan ConversationEvent
    Cancel()
    FollowUp(prompt string)
}

type conversationEventMsg struct {
    Event ConversationEvent
}

type conversationDoneMsg struct{}

// ConversationController renders a Bubble Tea model for streaming conversation state.
type ConversationController struct {
    session          ConversationSession
    events           []ConversationEvent
    waitingForTool   bool
    cancelled        bool
    followUpProvider func() string
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
func NewConversationController(session ConversationSession, opts ...ConversationControllerOption) *ConversationController {
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
            return c, c.watchSessionCmd()
        }
        c.events = append(c.events, msg.Event)
        c.waitingForTool = msg.Event.Type == ConversationEventTypeToolWait
        if msg.Event.Type == ConversationEventTypeToolResult || msg.Event.Type == ConversationEventTypeDone {
            c.waitingForTool = false
        }
        if msg.Event.Type == ConversationEventTypeDone {
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
        return conversationEventMsg{Event: ev}
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
    c.waitingForTool = false
}
