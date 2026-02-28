package pipeline

import "github.com/danabrams/gromit/internal/conversation"

// CollectConversation drains events from a conversation session while honoring follow-up prompts.
func CollectConversation(session conversation.Session, followUpProvider func() string, cancel <-chan struct{}) ([]conversation.Event, int) {
    if session == nil {
        return nil, 0
    }

    events := make([]conversation.Event, 0)
    ignored := 0
    cancelled := false

    for {
        select {
        case <-cancel:
            cancelled = true
        case ev, ok := <-session.Events():
            if !ok {
                return events, ignored
            }
            if cancelled {
                ignored++
                continue
            }
            events = append(events, ev)
            if ev.Type == conversation.EventTypeToolWait && followUpProvider != nil {
                if prompt := followUpProvider(); prompt != "" {
                    session.FollowUp(prompt)
                }
            }
            if ev.Type == conversation.EventTypeDone {
                return events, ignored
            }
        }
    }
}
