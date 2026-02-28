package integrationqueue

import "time"

// Lane models the execution lane for an integration queue entry.
type Lane string

const (
	SafeLane Lane = "safe_lane"
	CodeLane Lane = "code_lane"
)

// ErrorCode represents error codes for integration queue entries.
type ErrorCode string

const (
	ErrorCodeSessionCommitFailed ErrorCode = "session_commit_failed"
)

// SchemaVersion is the current version of the integration queue schema.
const SchemaVersion = 1

// State models the lifecycle of an integration queue entry.
type State string

const (
	StateDraft         State = "draft"
	StateReady         State = "ready"
	StateIntegrating   State = "integrating"
	StateMerged        State = "merged"
	StateConflict      State = "conflict"
	StateFailedGates   State = "failed_gates"
	StateLaneViolation State = "lane_violation"
)

var validStates = map[State]struct{}{
	StateDraft:         {},
	StateReady:         {},
	StateIntegrating:   {},
	StateMerged:        {},
	StateConflict:      {},
	StateFailedGates:   {},
	StateLaneViolation: {},
}

// Entry represents a persisted integration queue record.
type Entry struct {
	Branch               string    `json:"branch"`
	SessionID            string    `json:"session_id"`
	OriginCommand        string    `json:"origin_command"`
	State                State     `json:"state"`
	Lane                 string    `json:"lane"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	AttemptCount         int       `json:"attempt_count"`
	RetryCount           int       `json:"retry_count"`
	FifoSeq              int       `json:"fifo_seq"`
	BaseRef              string    `json:"base_ref"`
	HeadSHA              string    `json:"head_sha"`
	ChangedFiles         []string  `json:"changed_files,omitempty"`
	ChangedFilesHash     string    `json:"changed_files_hash"`
	LastErrorCode        string    `json:"last_error_code"`
	LastErrorMessage     string    `json:"last_error_message"`
	LastTransitionReason string    `json:"last_transition_reason"`
}

// Valid returns true when the state is recognized.
func (s State) Valid() bool {
	if s == "" {
		return false
	}
	_, ok := validStates[s]
	return ok
}

// EntryOrdering captures deterministic ordering metadata for an entry.
type EntryOrdering struct {
	Sequence  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Ordering returns ordering metadata derived from the entry.
func (e Entry) Ordering() EntryOrdering {
	return EntryOrdering{
		Sequence:  e.FifoSeq,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}
