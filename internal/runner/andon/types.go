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

var andonLevels = []AndonLevel{
	LevelL1,
	LevelL2,
	LevelL3,
	LevelL4,
}

// AllAndonLevels returns the canonical level catalog in escalation order.
func AllAndonLevels() []AndonLevel {
	return append([]AndonLevel(nil), andonLevels...)
}

// AndonThresholds defines policy limits for autonomous recovery.
type AndonThresholds struct {
	L1MaxRetries   int
	L1MaxDuration  time.Duration
	L2MaxDuration  time.Duration
	MaxAssumptions int
}

const (
	defaultL1MaxRetries   = 2
	defaultL1MaxDuration  = 2 * time.Minute
	defaultL2MaxDuration  = 15 * time.Minute
	defaultMaxAssumptions = 2
)

// DefaultThresholdDefinition returns the default policy bounds from the Andon spec.
func DefaultThresholdDefinition() AndonThresholds {
	return AndonThresholds{
		L1MaxRetries:   defaultL1MaxRetries,
		L1MaxDuration:  defaultL1MaxDuration,
		L2MaxDuration:  defaultL2MaxDuration,
		MaxAssumptions: defaultMaxAssumptions,
	}
}

// FailureClass groups failures into Andon policy categories.
type FailureClass string

const (
	FailureClassTransient FailureClass = "Transient"
	FailureClassWorkflow  FailureClass = "Workflow"
	FailureClassQuality   FailureClass = "Quality"
	FailureClassIntent    FailureClass = "Intent"
	FailureClassData      FailureClass = "Data"
)

var failureClasses = []FailureClass{
	FailureClassTransient,
	FailureClassWorkflow,
	FailureClassQuality,
	FailureClassIntent,
	FailureClassData,
}

// AllFailureClasses returns the canonical class catalog in spec order.
func AllFailureClasses() []FailureClass {
	return append([]FailureClass(nil), failureClasses...)
}

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

// DecisionPath identifies the class-aware policy branch selected for a decision.
type DecisionPath string

const (
	DecisionPathTransientL1Retry                            DecisionPath = "transient_l1_retry"
	DecisionPathWorkflowEscalateAfterDeterministicAttempt DecisionPath = "workflow_escalate_after_deterministic_attempt"
	DecisionPathQualityStopLineAfterTimebox                DecisionPath = "quality_stop_line_after_timebox"
	DecisionPathIntentEscalateAfterAssumptionBudget        DecisionPath = "intent_escalate_after_assumption_budget"
	DecisionPathDataImmediateStopLine                      DecisionPath = "data_immediate_stop_line"
	DecisionPathWorkflowFallbackForUnknownKind             DecisionPath = "workflow_fallback_for_unknown_kind"
)

// PolicyEvaluation is the classification plus selected decision.
type PolicyEvaluation struct {
	Class    FailureClass
	Decision PolicyDecision
	Path     DecisionPath
}

// PolicyClassification is the classifier output consumed by policy decisions.
type PolicyClassification struct {
	Class                  FailureClass
	IsWorkflowFallbackKind bool
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
