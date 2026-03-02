package integrationqueue

import (
	"errors"
	"time"
)

// ErrInvalidTransition is returned when a state transition is not allowed.
var ErrInvalidTransition = errors.New("invalid_transition")

// TransitionRecord captures metadata about a state transition.
type TransitionRecord struct {
	FromState string
	ToState   string
	Reason    string
	ErrorCode string
	Timestamp time.Time
}

// allowedTransitions is the table-driven transition matrix.
var allowedTransitions = map[string]map[string]bool{
	"draft": {
		"ready": true,
	},
	"ready": {
		"integrating": true,
	},
	"integrating": {
		"merged":         true,
		"conflict":       true,
		"failed_gates":   true,
		"lane_violation": true,
		"push_failure":   true,
		"ready":          true,
	},
	"push_failure": {
		"ready": true,
	},
	"conflict": {
		"ready": true,
	},
	"failed_gates": {
		"ready": true,
	},
	"lane_violation": {
		"ready": true,
	},
}

// CanTransition checks if a transition from one state to another is allowed.
func CanTransition(from, to string) bool {
	fromTransitions, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	return fromTransitions[to]
}

// CheckTransition checks if a transition from one state to another is allowed,
// returning ErrInvalidTransition if not.
func CheckTransition(from, to string) error {
	if !CanTransition(from, to) {
		return ErrInvalidTransition
	}
	return nil
}

// ApplyTransition validates the transition and updates the entry's state,
// updated_at, and last_transition_reason on success. Returns ErrInvalidTransition if not allowed.
func ApplyTransition(entry *Entry, toState string, reason string) error {
	if err := CheckTransition(string(entry.State), toState); err != nil {
		return err
	}
	entry.State = State(toState)
	entry.LastTransitionReason = reason
	entry.UpdatedAt = time.Now()
	return nil
}

// RecordTransition creates a TransitionRecord with the given parameters and current timestamp.
func RecordTransition(from, to, reason, errorCode string) *TransitionRecord {
	return &TransitionRecord{
		FromState: from,
		ToState:   to,
		Reason:    reason,
		ErrorCode: errorCode,
		Timestamp: time.Now(),
	}
}

// NextAllowedStates returns a list of all allowed next states from the given state.
func NextAllowedStates(from string) []string {
	fromTransitions, ok := allowedTransitions[from]
	if !ok {
		return nil
	}

	states := make([]string, 0, len(fromTransitions))
	for state := range fromTransitions {
		states = append(states, state)
	}

	return states
}

// IsTerminalState returns true if the state is a terminal state (has no transitions out).
// A state is terminal if it has no entry in the allowedTransitions map.
func IsTerminalState(state string) bool {
	_, ok := allowedTransitions[state]
	return !ok
}

// IsBlockedState returns true if the state is a blocked state.
// A state is blocked if it has exactly one outgoing transition: to "ready".
func IsBlockedState(state string) bool {
	transitions, ok := allowedTransitions[state]
	if !ok {
		return false
	}
	// A blocked state has exactly one transition: to "ready"
	if len(transitions) != 1 {
		return false
	}
	return transitions["ready"]
}
