package events

import "time"

// BuildStartEvent is emitted before invoking Claude for the build phase.
type BuildStartEvent struct {
	BeadID      string
	Model       string
	Attempt     int
	MaxAttempts int
	TimeMixin
}

func (e *BuildStartEvent) EventType() string {
	return "build_start"
}

// BuildCompleteEvent is emitted after Claude returns from the build phase.
type BuildCompleteEvent struct {
	BeadID    string
	Success   bool
	Duration  time.Duration
	Cost      float64
	TokensIn  int
	TokensOut int
	TimeMixin
}

func (e *BuildCompleteEvent) EventType() string {
	return "build_complete"
}

// ValidationStartEvent is emitted before running validation.
type ValidationStartEvent struct {
	BeadID   string
	Model    string
	Commands []string
	TimeMixin
}

func (e *ValidationStartEvent) EventType() string {
	return "validation_start"
}

// ValidationPassEvent is emitted when validation succeeds.
type ValidationPassEvent struct {
	BeadID   string
	Duration time.Duration
	TimeMixin
}

func (e *ValidationPassEvent) EventType() string {
	return "validation_pass"
}

// ValidationFailEvent is emitted when validation fails.
type ValidationFailEvent struct {
	BeadID   string
	Output   string
	Duration time.Duration
	TimeMixin
}

func (e *ValidationFailEvent) EventType() string {
	return "validation_fail"
}

// ReviewStartEvent is emitted before running post-iteration review.
type ReviewStartEvent struct {
	BeadID   string
	Model    string
	Thorough bool
	TimeMixin
}

func (e *ReviewStartEvent) EventType() string {
	return "review_start"
}

// ReviewCompleteEvent is emitted after review finishes.
type ReviewCompleteEvent struct {
	BeadID  string
	Verdict string
	Issues  []string
	TimeMixin
}

func (e *ReviewCompleteEvent) EventType() string {
	return "review_complete"
}

// AnalysisStartEvent is emitted before failure analysis.
type AnalysisStartEvent struct {
	BeadID string
	TimeMixin
}

func (e *AnalysisStartEvent) EventType() string {
	return "analysis_start"
}

// AnalysisCompleteEvent is emitted after failure analysis.
type AnalysisCompleteEvent struct {
	BeadID      string
	Category    string
	Recoverable bool
	Suggestion  string
	TimeMixin
}

func (e *AnalysisCompleteEvent) EventType() string {
	return "analysis_complete"
}

// RetroStartEvent is emitted before retrospective.
type RetroStartEvent struct {
	BeadID string
	TimeMixin
}

func (e *RetroStartEvent) EventType() string {
	return "retro_start"
}

// RetroCompleteEvent is emitted after retrospective.
type RetroCompleteEvent struct {
	BeadID               string
	ProvisionalLearnings int
	RulesUpdated         bool
	TimeMixin
}

func (e *RetroCompleteEvent) EventType() string {
	return "retro_complete"
}

// HeartbeatEvent is emitted periodically during Claude invocations.
type HeartbeatEvent struct {
	Elapsed            time.Duration
	ToolCalls          int
	FilesModified      int
	RateLimitHits      int
	WaitingForResponse bool
	TimeMixin
}

func (e *HeartbeatEvent) EventType() string {
	return "heartbeat"
}

// ModelSelectedEvent is emitted when a model is chosen.
type ModelSelectedEvent struct {
	Model  string
	Reason string
	TimeMixin
}

func (e *ModelSelectedEvent) EventType() string {
	return "model_selected"
}

// EscalationEvent is emitted when retrying with a higher-tier model.
type EscalationEvent struct {
	FromModel string
	ToModel   string
	Attempt   int
	Reason    string
	TimeMixin
}

func (e *EscalationEvent) EventType() string {
	return "escalation"
}

// StallDetectedEvent is emitted when a stall timeout fires.
type StallDetectedEvent struct {
	Elapsed   time.Duration
	Threshold time.Duration
	TimeMixin
}

func (e *StallDetectedEvent) EventType() string {
	return "stall_detected"
}

// ScopeCheckEvent is emitted after scope check completes.
type ScopeCheckEvent struct {
	BeadID     string
	Complexity string
	Approved   bool
	Reason     string
	TimeMixin
}

func (e *ScopeCheckEvent) EventType() string {
	return "scope_check"
}
