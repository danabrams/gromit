package pipeline

import (
	"fmt"

	"github.com/danabrams/gromit/internal/conversation"
)

// ValidateConversationEventSequence ensures event ordering rules are followed.
// Returns an error if any terminal state (Completed, Failed, Cancelled) is followed by additional events.
func ValidateConversationEventSequence(events []ConversationEvent) error {
	var terminalIdx int = -1

	for i, ev := range events {
		// Check if current event is a terminal state
		if isConversationTerminal(ev.State) {
			// If we've already seen a terminal state, that's an error
			if terminalIdx != -1 {
				return fmt.Errorf("terminal state at index %d followed by another event at index %d", terminalIdx, i)
			}
			terminalIdx = i
		} else if terminalIdx != -1 {
			// We have a non-terminal event after seeing a terminal state
			return fmt.Errorf("event at index %d follows terminal state at index %d", i, terminalIdx)
		}
	}

	return nil
}

func isConversationTerminal(state ConversationLifecycleState) bool {
	return state == ConversationStateCompleted ||
		state == ConversationStateFailed ||
		state == ConversationStateCancelled
}

// IsTerminalState reports whether a state is a terminal state.
// Terminal states (Completed, Failed, Cancelled) indicate end of conversation.
func IsTerminalState(state ConversationLifecycleState) bool {
	return isConversationTerminal(state)
}

// CollectConversation drains events from a conversation session while honoring follow-up prompts.
func CollectConversation(session conversation.Session, followUpProvider func() string, cancel <-chan struct{}) ([]conversation.Event, int) {
    if session == nil {
        return nil, 0
    }

    events := make([]conversation.Event, 0)
    ignored := 0
    cancelled := false

    for {
        // Check cancel first (non-blocking) to avoid select non-determinism
        // when both cancel and events channels are ready simultaneously.
        select {
        case <-cancel:
            cancelled = true
            session.Cancel()
        default:
        }

        select {
        case <-cancel:
            cancelled = true
            session.Cancel()
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
