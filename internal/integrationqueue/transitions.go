package integrationqueue

import "time"

// TransitionRecord captures metadata about a state transition.
type TransitionRecord struct {
	FromState string
	ToState   string
	Reason    string
	ErrorCode string
	Timestamp time.Time
}

// CanTransition checks if a transition from one state to another is allowed.
func CanTransition(from, to string) bool {
	if from == "draft" && to == "ready" {
		return true
	}
	if from == "ready" && to == "integrating" {
		return true
	}
	if from == "integrating" && to == "merged" {
		return true
	}
	if from == "integrating" && to == "conflict" {
		return true
	}
	if from == "integrating" && to == "failed_gates" {
		return true
	}
	if from == "integrating" && to == "lane_violation" {
		return true
	}
	if from == "conflict" && to == "ready" {
		return true
	}
	if from == "failed_gates" && to == "ready" {
		return true
	}
	if from == "lane_violation" && to == "ready" {
		return true
	}
	return false
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
