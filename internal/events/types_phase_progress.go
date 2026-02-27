package events

import "time"

// BuildStartEvent is emitted before invoking Claude for the build phase.
type BuildStartEvent struct {
	BeadID     string
	Model      string
	Attempt    int
	MaxAttempts int
	Time       time.Time
}

func (e *BuildStartEvent) EventType() string {
	return "build_start"
}

func (e *BuildStartEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// BuildCompleteEvent is emitted after Claude returns from the build phase.
type BuildCompleteEvent struct {
	BeadID    string
	Success   bool
	Duration  time.Duration
	Cost      float64
	TokensIn  int
	TokensOut int
	Time      time.Time
}

func (e *BuildCompleteEvent) EventType() string {
	return "build_complete"
}

func (e *BuildCompleteEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// ValidationStartEvent is emitted before running validation.
type ValidationStartEvent struct {
	BeadID   string
	Commands []string
	Time     time.Time
}

func (e *ValidationStartEvent) EventType() string {
	return "validation_start"
}

func (e *ValidationStartEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// ValidationPassEvent is emitted when validation succeeds.
type ValidationPassEvent struct {
	BeadID   string
	Duration time.Duration
	Time     time.Time
}

func (e *ValidationPassEvent) EventType() string {
	return "validation_pass"
}

func (e *ValidationPassEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// ValidationFailEvent is emitted when validation fails.
type ValidationFailEvent struct {
	BeadID   string
	Output   string
	Duration time.Duration
	Time     time.Time
}

func (e *ValidationFailEvent) EventType() string {
	return "validation_fail"
}

func (e *ValidationFailEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// ReviewStartEvent is emitted before running post-iteration review.
type ReviewStartEvent struct {
	BeadID    string
	Model     string
	Thorough  bool
	Time      time.Time
}

func (e *ReviewStartEvent) EventType() string {
	return "review_start"
}

func (e *ReviewStartEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// ReviewCompleteEvent is emitted after review finishes.
type ReviewCompleteEvent struct {
	BeadID string
	Verdict string
	Issues []string
	Time   time.Time
}

func (e *ReviewCompleteEvent) EventType() string {
	return "review_complete"
}

func (e *ReviewCompleteEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// AnalysisStartEvent is emitted before failure analysis.
type AnalysisStartEvent struct {
	BeadID string
	Time   time.Time
}

func (e *AnalysisStartEvent) EventType() string {
	return "analysis_start"
}

func (e *AnalysisStartEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// AnalysisCompleteEvent is emitted after failure analysis.
type AnalysisCompleteEvent struct {
	BeadID      string
	Category    string
	Recoverable bool
	Suggestion  string
	Time        time.Time
}

func (e *AnalysisCompleteEvent) EventType() string {
	return "analysis_complete"
}

func (e *AnalysisCompleteEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// RetroStartEvent is emitted before retrospective.
type RetroStartEvent struct {
	BeadID string
	Time   time.Time
}

func (e *RetroStartEvent) EventType() string {
	return "retro_start"
}

func (e *RetroStartEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// RetroCompleteEvent is emitted after retrospective.
type RetroCompleteEvent struct {
	BeadID               string
	ProvisionalLearnings int
	RulesUpdated         bool
	Time                 time.Time
}

func (e *RetroCompleteEvent) EventType() string {
	return "retro_complete"
}

func (e *RetroCompleteEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// HeartbeatEvent is emitted periodically during Claude invocations.
type HeartbeatEvent struct {
	Elapsed           time.Duration
	ToolCalls         int
	FilesModified     int
	RateLimitHits     int
	WaitingForResponse bool
	Time              time.Time
}

func (e *HeartbeatEvent) EventType() string {
	return "heartbeat"
}

func (e *HeartbeatEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// ModelSelectedEvent is emitted when a model is chosen.
type ModelSelectedEvent struct {
	Model  string
	Reason string
	Time   time.Time
}

func (e *ModelSelectedEvent) EventType() string {
	return "model_selected"
}

func (e *ModelSelectedEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// EscalationEvent is emitted when retrying with a higher-tier model.
type EscalationEvent struct {
	FromModel string
	ToModel   string
	Attempt   int
	Reason    string
	Time      time.Time
}

func (e *EscalationEvent) EventType() string {
	return "escalation"
}

func (e *EscalationEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// StallDetectedEvent is emitted when a stall timeout fires.
type StallDetectedEvent struct {
	Elapsed   time.Duration
	Threshold time.Duration
	Time      time.Time
}

func (e *StallDetectedEvent) EventType() string {
	return "stall_detected"
}

func (e *StallDetectedEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}

// ScopeCheckEvent is emitted after scope check completes.
type ScopeCheckEvent struct {
	BeadID    string
	Complexity string
	Approved  bool
	Reason    string
	Time      time.Time
}

func (e *ScopeCheckEvent) EventType() string {
	return "scope_check"
}

func (e *ScopeCheckEvent) EventTime() time.Time {
	if e.Time.IsZero() {
		return time.Now()
	}
	return e.Time
}
