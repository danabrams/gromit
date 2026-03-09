package event

import "time"

// SchemaVersion describes the current event schema.
const SchemaVersion = 1

// TypedEvent defines the interface every event implements.
type TypedEvent interface {
	EventType() string
}

// Event captures the common metadata every typed event exposes.
type Event struct {
	SchemaVersion int       `json:"schema_version"`
	Timestamp     time.Time `json:"timestamp"`
	Type          string    `json:"type"`
}

const (
	EventTypeSpecStarted             = "spec.started"
	EventTypeSpecCompleted           = "spec.completed"
	EventTypeSpecFailed              = "spec.failed"
	EventTypeBeadStarted             = "bead.started"
	EventTypeBeadCompleted           = "bead.completed"
	EventTypeStageStarted            = "stage.started"
	EventTypeStageCompleted          = "stage.completed"
	EventTypeStageFailed             = "stage.failed"
	EventTypeStageRetrying           = "stage.retrying"
	EventTypeValidation              = "validation"
	EventTypeReview                  = "review"
	EventTypeSpecReviewCompleted     = "spec_review.completed"
	EventTypeScope                   = "scope"
	EventTypeTelemetry               = "telemetry"
	EventTypeGenerationCapReached    = "generation_cap_reached"
	EventTypeTriageStarted           = "triage.started"
	EventTypeTriageCompleted         = "triage.completed"
	EventTypeBuildInvocationStart    = "build_invocation.start"
	EventTypeBuildInvocationComplete = "build_invocation.complete"
	EventTypeModelSelected           = "model.selected"
)

// SpecStartedEvent marks the beginning of a spec execution.
type SpecStartedEvent struct {
	Event
	SpecID   string `json:"spec_id,omitempty"`
	Worktree string `json:"worktree,omitempty"`
}

func (SpecStartedEvent) EventType() string { return EventTypeSpecStarted }

// SpecCompletedEvent marks the end of a spec execution.
type SpecCompletedEvent struct {
	Event
	SpecID        string `json:"spec_id,omitempty"`
	Worktree      string `json:"worktree,omitempty"`
	Success       bool   `json:"success"`
	FailureReason string `json:"failure_reason,omitempty"`
}

func (SpecCompletedEvent) EventType() string { return EventTypeSpecCompleted }

// SpecFailedEvent reports a spec execution that could not be remediated.
type SpecFailedEvent struct {
	Event
	SpecID        string `json:"spec_id,omitempty"`
	Worktree      string `json:"worktree,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
}

func (SpecFailedEvent) EventType() string { return EventTypeSpecFailed }

// BeadStartedEvent records when a bead execution begins.
type BeadStartedEvent struct {
	Event
	BeadID    string `json:"bead_id,omitempty"`
	BeadTitle string `json:"bead_title,omitempty"`
	Iteration int    `json:"iteration,omitempty"`
}

func (BeadStartedEvent) EventType() string { return EventTypeBeadStarted }

// BeadCompletedEvent captures the outcome of a bead.
type BeadCompletedEvent struct {
	Event
	BeadID       string        `json:"bead_id,omitempty"`
	BeadTitle    string        `json:"bead_title,omitempty"`
	Iteration    int           `json:"iteration,omitempty"`
	Success      bool          `json:"success"`
	RetryAttempt int           `json:"retry_attempt,omitempty"`
	Model        string        `json:"model,omitempty"`
	Provider     string        `json:"provider,omitempty"`
	CostUSD      float64       `json:"cost_usd,omitempty"`
	InputTokens  int           `json:"input_tokens,omitempty"`
	OutputTokens int           `json:"output_tokens,omitempty"`
	Duration     time.Duration `json:"duration,omitempty"`
}

func (BeadCompletedEvent) EventType() string { return EventTypeBeadCompleted }

// StageStartedEvent marks the start of a stage execution.
type StageStartedEvent struct {
	Event
	StageName string `json:"stage_name,omitempty"`
	BeadID    string `json:"bead_id,omitempty"`
	Iteration int    `json:"iteration,omitempty"`
}

func (StageStartedEvent) EventType() string { return EventTypeStageStarted }

// StageCompletedEvent reports the completion of a stage.
type StageCompletedEvent struct {
	Event
	StageName string        `json:"stage_name,omitempty"`
	BeadID    string        `json:"bead_id,omitempty"`
	Iteration int           `json:"iteration,omitempty"`
	Success   bool          `json:"success"`
	Duration  time.Duration `json:"duration,omitempty"`
}

func (StageCompletedEvent) EventType() string { return EventTypeStageCompleted }

// StageFailedEvent reports a stage failure.
type StageFailedEvent struct {
	Event
	StageName string `json:"stage_name,omitempty"`
	BeadID    string `json:"bead_id,omitempty"`
	Iteration int    `json:"iteration,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (StageFailedEvent) EventType() string { return EventTypeStageFailed }

// StageRetryingEvent records when a stage is retried.
type StageRetryingEvent struct {
	Event
	StageName string `json:"stage_name,omitempty"`
	BeadID    string `json:"bead_id,omitempty"`
	Attempt   int    `json:"attempt,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

func (StageRetryingEvent) EventType() string { return EventTypeStageRetrying }

// ValidationEvent summarizes a validation execution.
type ValidationEvent struct {
	Event
	BeadID        string        `json:"bead_id,omitempty"`
	StageName     string        `json:"stage_name,omitempty"`
	Commands      []string      `json:"commands,omitempty"`
	FailedCommand string        `json:"failed_command,omitempty"`
	Succeeded     bool          `json:"succeeded"`
	Duration      time.Duration `json:"duration,omitempty"`
	Details       string        `json:"details,omitempty"`
}

func (ValidationEvent) EventType() string { return EventTypeValidation }

// ReviewEvent records the outcome of a review invocation.
type ReviewEvent struct {
	Event
	BeadID     string   `json:"bead_id,omitempty"`
	Verdict    string   `json:"verdict,omitempty"`
	Issues     []string `json:"issues,omitempty"`
	OutOfScope []string `json:"out_of_scope,omitempty"`
	Notes      string   `json:"notes,omitempty"`
}

func (ReviewEvent) EventType() string { return EventTypeReview }

// SpecReviewCompletedEvent records the spec-level review verdict.
type SpecReviewCompletedEvent struct {
	Event
	SpecID           string `json:"spec_id,omitempty"`
	Worktree         string `json:"worktree,omitempty"`
	Verdict          string `json:"verdict,omitempty"`
	FindingCount     int    `json:"finding_count,omitempty"`
	CriticalFindings int    `json:"critical_findings,omitempty"`
	Success          bool   `json:"success"`
}

func (SpecReviewCompletedEvent) EventType() string { return EventTypeSpecReviewCompleted }

// ScopeEvent captures the result of a scope check.
type ScopeEvent struct {
	Event
	BeadID     string `json:"bead_id,omitempty"`
	Complexity string `json:"complexity,omitempty"`
	Approved   bool   `json:"approved"`
	Reason     string `json:"reason,omitempty"`
}

func (ScopeEvent) EventType() string { return EventTypeScope }

// TelemetryEvent holds aggregated telemetry for a stage execution.
type TelemetryEvent struct {
	Event
	BeadID       string        `json:"bead_id,omitempty"`
	StageName    string        `json:"stage_name,omitempty"`
	Model        string        `json:"model,omitempty"`
	Duration     time.Duration `json:"duration,omitempty"`
	InputTokens  int           `json:"input_tokens,omitempty"`
	OutputTokens int           `json:"output_tokens,omitempty"`
	CostUSD      float64       `json:"cost_usd,omitempty"`
	Category     string        `json:"category,omitempty"`
}

func (TelemetryEvent) EventType() string { return EventTypeTelemetry }

// GenerationCapReachedEvent is emitted when the bead loop stops because the generation cap is exceeded.
type GenerationCapReachedEvent struct {
	Event
	GenerationCap     int `json:"generation_cap,omitempty"`
	HighestGeneration int `json:"highest_generation,omitempty"`
}

func (GenerationCapReachedEvent) EventType() string { return EventTypeGenerationCapReached }

// TriageStartedEvent marks the beginning of a triage classification.
type TriageStartedEvent struct {
	Event
	BeadID    string `json:"bead_id,omitempty"`
	BeadTitle string `json:"bead_title,omitempty"`
	Iteration int    `json:"iteration,omitempty"`
}

func (TriageStartedEvent) EventType() string { return EventTypeTriageStarted }

// TriageCompletedEvent records the outcome of a triage classification.
type TriageCompletedEvent struct {
	Event
	BeadID    string `json:"bead_id,omitempty"`
	BeadTitle string `json:"bead_title,omitempty"`
	Iteration int    `json:"iteration,omitempty"`
	Category  string `json:"category,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`
}

func (TriageCompletedEvent) EventType() string { return EventTypeTriageCompleted }

// BuildInvocationStartEvent is emitted before an LLM invocation begins.
type BuildInvocationStartEvent struct {
	Event
	BeadID      string `json:"bead_id,omitempty"`
	Model       string `json:"model,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Tier        string `json:"tier,omitempty"`
	Attempt     int    `json:"attempt,omitempty"`
	MaxAttempts int    `json:"max_attempts,omitempty"`
}

func (BuildInvocationStartEvent) EventType() string { return EventTypeBuildInvocationStart }

// BuildInvocationCompleteEvent is emitted after an LLM invocation completes.
type BuildInvocationCompleteEvent struct {
	Event
	BeadID       string        `json:"bead_id,omitempty"`
	Model        string        `json:"model,omitempty"`
	Provider     string        `json:"provider,omitempty"`
	Success      bool          `json:"success"`
	Duration     time.Duration `json:"duration,omitempty"`
	CostUSD      float64       `json:"cost_usd,omitempty"`
	InputTokens  int           `json:"input_tokens,omitempty"`
	OutputTokens int           `json:"output_tokens,omitempty"`
	PromptSize   int           `json:"prompt_size,omitempty"`
}

func (BuildInvocationCompleteEvent) EventType() string { return EventTypeBuildInvocationComplete }

// ModelSelectedEvent is emitted when the router selects a provider and model.
type ModelSelectedEvent struct {
	Event
	BeadID   string `json:"bead_id,omitempty"`
	Model    string `json:"model,omitempty"`
	Provider string `json:"provider,omitempty"`
	Tier     string `json:"tier,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

func (ModelSelectedEvent) EventType() string { return EventTypeModelSelected }
