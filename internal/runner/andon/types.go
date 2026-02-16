package andon

import "time"

// AndonLevel identifies the escalation depth.
type AndonLevel string

const (
	LevelL1 AndonLevel = "L1"
	LevelL2 AndonLevel = "L2"
	LevelL3 AndonLevel = "L3"
	LevelL4 AndonLevel = "L4"
)

// AndonThresholds defines policy limits for autonomous recovery.
type AndonThresholds struct {
	L1MaxRetries   int
	L1MaxDuration  time.Duration
	L2MaxDuration  time.Duration
	MaxAssumptions int
}

// FailureClass groups failures into Andon policy categories.
type FailureClass string

const (
	FailureClassTransient FailureClass = "transient"
	FailureClassWorkflow  FailureClass = "workflow"
	FailureClassQuality   FailureClass = "quality"
	FailureClassIntent    FailureClass = "intent"
	FailureClassData      FailureClass = "data"
)

// FailureKind captures the source pattern for classification.
type FailureKind string

const (
	FailureKindTimeout         FailureKind = "timeout"
	FailureKindWorkflow        FailureKind = "workflow"
	FailureKindQualityGate     FailureKind = "quality_gate"
	FailureKindAmbiguousIntent FailureKind = "ambiguous_intent"
	FailureKindIntegrity       FailureKind = "integrity"
)

// FailureSignal is the policy input for failure classification.
type FailureSignal struct {
	Kind   FailureKind
	Output string
}

// Decision is the next operation selected by Andon policy.
type Decision string

const (
	DecisionRetry    Decision = "retry"
	DecisionEscalate Decision = "escalate"
	DecisionStopLine Decision = "stop_line"
)

// PolicyDecision is the chosen action and level transition.
type PolicyDecision struct {
	NextLevel AndonLevel
	Action    Decision
}

// RecoveryState tracks bounded recovery progress for a failure.
type RecoveryState struct {
	Class           FailureClass
	Level           AndonLevel
	L1Attempts      int
	L1Started       time.Time
	L2Started       time.Time
	AssumptionsUsed int
}
