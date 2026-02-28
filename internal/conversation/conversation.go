package conversation

// EventType describes the kind of update delivered by a conversation session.
type EventType int

const (
    EventTypeStream EventType = iota
    EventTypeToolWait
    EventTypeToolResult
    EventTypeDone
)

func (t EventType) String() string {
    switch t {
    case EventTypeStream:
        return "stream"
    case EventTypeToolWait:
        return "tool wait"
    case EventTypeToolResult:
        return "tool result"
    case EventTypeDone:
        return "done"
    default:
        return "unknown"
    }
}

// Event represents a discrete update emitted by a conversation session.
type Event struct {
    Type     EventType
    Text     string
    ToolName string
}

// Session abstracts the streaming contract shared between pipeline and UI components.
type Session interface {
    Events() <-chan Event
    Cancel()
    FollowUp(prompt string)
}
