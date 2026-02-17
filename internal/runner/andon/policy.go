package andon

import (
	"path/filepath"
	"strings"
	"time"
)

const workflowEscalationAttemptThreshold = 1
const defaultFailureClass = FailureClassWorkflow

// DefaultThresholds returns the default policy bounds from the Andon spec.
func DefaultThresholds() AndonThresholds {
	return DefaultThresholdDefinition()
}

// PolicyInput is the deterministic policy input without command or clock dependencies.
type PolicyInput struct {
	State      RecoveryState
	Thresholds AndonThresholds
	L1Elapsed  time.Duration
	L2Elapsed  time.Duration
}

// ClassifyFailure maps an observed signal to an Andon failure class.
func ClassifyFailure(signal FailureSignal) FailureClass {
	class, _ := classifyFailure(signal)
	return class
}

func classifyFailure(signal FailureSignal) (FailureClass, bool) {
	switch signal.Kind {
	case FailureKindTimeout:
		return FailureClassTransient, false
	case FailureKindWorkflow:
		return FailureClassWorkflow, false
	case FailureKindQualityGate:
		return FailureClassQuality, false
	case FailureKindAmbiguousIntent:
		return FailureClassIntent, false
	case FailureKindIntegrity:
		return FailureClassData, false
	case FailureKindHardStopBulkDelete:
		if isBulkDeleteAllowlisted(signal.HardStop) {
			return FailureClassTransient, false
		}
		return FailureClassData, false
	case FailureKindHardStopIrreversibleMigration:
		return FailureClassData, false
	case FailureKindHardStopCredentialChange:
		return FailureClassData, false
	default:
		// Unknown signals route through the workflow class so policy output remains canonical.
		return defaultFailureClass, true
	}
}

// ClassifyFailureEntry classifies a failure signal for policy-level decisioning.
func ClassifyFailureEntry(signal FailureSignal) PolicyClassification {
	class, fallback := classifyFailure(signal)
	classification := PolicyClassification{
		Class:                  class,
		IsWorkflowFallbackKind: fallback,
	}
	switch signal.Kind {
	case FailureKindHardStopIrreversibleMigration, FailureKindHardStopCredentialChange:
		classification.IsHardStopAction = true
	case FailureKindHardStopBulkDelete:
		if !isBulkDeleteAllowlisted(signal.HardStop) {
			classification.IsHardStopAction = true
			classification.IsBulkDeleteOutsideAllowlist = hasExplicitBulkDeleteAllowlistContext(signal.HardStop)
		}
	}

	return classification
}

// ChooseNextAction computes the next bounded recovery step for a failure state.
func ChooseNextAction(state RecoveryState, thresholds AndonThresholds, now time.Time) PolicyDecision {
	return ChooseNextActionPure(PolicyInput{
		State:      state,
		Thresholds: thresholds,
		L1Elapsed:  elapsed(state.L1Started, now),
		L2Elapsed:  elapsed(state.L2Started, now),
	})
}

// EvaluateFailure classifies a failure signal and selects the next action.
func EvaluateFailure(signal FailureSignal, state RecoveryState, thresholds AndonThresholds, now time.Time) PolicyEvaluation {
	classification := ClassifyFailureEntry(signal)
	return EvaluateClassifiedFailure(classification, state, thresholds, now)
}

// EvaluateFailureWithTrace classifies a failure and returns trace metadata for policy decisioning.
func EvaluateFailureWithTrace(signal FailureSignal, state RecoveryState, thresholds AndonThresholds, now time.Time) PolicyDecisionTrace {
	classification := ClassifyFailureEntry(signal)
	evaluation := EvaluateClassifiedFailure(classification, state, thresholds, now)

	return PolicyDecisionTrace{
		Classification:      classification,
		Decision:            evaluation.Decision,
		Path:                evaluation.Path,
		DecisionInputSource: DecisionInputSourceTraceClassifier,
		BoundaryTransition:  detectBoundaryTransition(state.Level, evaluation.Decision.NextLevel),
	}
}

// EvaluateClassifiedFailure selects policy action from a pre-classified failure.
func EvaluateClassifiedFailure(classification PolicyClassification, state RecoveryState, thresholds AndonThresholds, now time.Time) PolicyEvaluation {
	return evaluateFailureClass(classification, state, thresholds, now)
}

// EvaluateClassifiedFailureWithTrace evaluates a pre-classified failure and returns trace metadata.
func EvaluateClassifiedFailureWithTrace(classification PolicyClassification, state RecoveryState, thresholds AndonThresholds, now time.Time) PolicyDecisionTrace {
	evaluation := EvaluateClassifiedFailure(classification, state, thresholds, now)

	return PolicyDecisionTrace{
		Classification:      classification,
		Decision:            evaluation.Decision,
		Path:                evaluation.Path,
		DecisionInputSource: DecisionInputSourceClassifiedEntry,
		BoundaryTransition:  detectBoundaryTransition(state.Level, evaluation.Decision.NextLevel),
	}
}

func detectBoundaryTransition(from, to AndonLevel) BoundaryTransitionType {
	if from == LevelL1 && to == LevelL2 {
		return BoundaryL1ToL2
	}

	return ""
}

func evaluateFailureClass(classification PolicyClassification, state RecoveryState, thresholds AndonThresholds, now time.Time) PolicyEvaluation {
	// Apply classification to a local copy so decisioning stays pure from the caller's perspective.
	classifiedState := state
	classifiedState.Class = classification.Class
	decision, path := chooseDecisionForClass(classification, classifiedState, thresholds, now)

	return PolicyEvaluation{
		Class:    classification.Class,
		Decision: decision,
		Path:     path,
	}
}

func chooseDecisionForClass(classification PolicyClassification, state RecoveryState, thresholds AndonThresholds, now time.Time) (PolicyDecision, DecisionPath) {
	if classification.IsHardStopAction {
		if classification.IsBulkDeleteOutsideAllowlist {
			return PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate}, DecisionPathHardStopBulkDeleteOutsideAllowlist
		}
		return PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate}, DecisionPathHardStopRequiresApproval
	}

	decision := ChooseNextAction(state, thresholds, now)

	if classification.IsWorkflowFallbackKind {
		return decision, DecisionPathWorkflowFallbackForUnknownKind
	}

	return decision, DecisionPathForClass(classification.Class)
}

func isBulkDeleteAllowlisted(hardStop HardStopContext) bool {
	command := strings.TrimSpace(hardStop.Command)
	if command == "" || len(hardStop.BulkDeleteAllowlist) == 0 {
		return false
	}

	targetPath := extractDeleteTargetPath(command)
	if targetPath == "" {
		return false
	}
	cleanTarget := filepath.Clean(targetPath)
	sep := string(filepath.Separator)

	for _, allowed := range hardStop.BulkDeleteAllowlist {
		cleanAllowed := filepath.Clean(strings.TrimSpace(allowed))
		if cleanAllowed == "" {
			continue
		}
		if cleanTarget == cleanAllowed || strings.HasPrefix(cleanTarget, cleanAllowed+sep) {
			return true
		}
	}

	return false
}

func hasExplicitBulkDeleteAllowlistContext(hardStop HardStopContext) bool {
	return strings.TrimSpace(hardStop.Command) != "" && len(hardStop.BulkDeleteAllowlist) > 0
}

func extractDeleteTargetPath(command string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}

	target := parts[len(parts)-1]
	if strings.HasPrefix(target, "-") {
		return ""
	}

	return target
}

// DecisionPathForClass returns the canonical decision path branch for a failure class.
func DecisionPathForClass(class FailureClass) DecisionPath {
	switch class {
	case FailureClassTransient:
		return DecisionPathTransientL1Retry
	case FailureClassWorkflow:
		return DecisionPathWorkflowEscalateAfterDeterministicAttempt
	case FailureClassQuality:
		return DecisionPathQualityStopLineAfterTimebox
	case FailureClassIntent:
		return DecisionPathIntentEscalateAfterAssumptionBudget
	case FailureClassData:
		return DecisionPathDataImmediateStopLine
	default:
		return DecisionPathWorkflowFallbackForUnknownKind
	}
}

// ChooseNextActionPure computes the next bounded recovery step for a failure state.
func ChooseNextActionPure(input PolicyInput) PolicyDecision {
	thresholds := normalizeThresholds(input.Thresholds)
	state := input.State

	if state.Class == FailureClassData {
		return PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine}
	}

	if state.Class == FailureClassIntent && state.AssumptionsUsed >= thresholds.MaxAssumptions {
		return PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate}
	}

	if state.Class == FailureClassWorkflow &&
		state.Level == LevelL1 &&
		state.L1Attempts >= workflowEscalationAttemptThreshold {
		return PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate}
	}

	if state.Level == LevelL2 {
		if input.L2Elapsed >= thresholds.L2MaxDuration {
			return PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine}
		}

		return PolicyDecision{NextLevel: LevelL2, Action: DecisionRetry}
	}

	if state.L1Attempts >= thresholds.L1MaxRetries || input.L1Elapsed >= thresholds.L1MaxDuration {
		return PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate}
	}

	return PolicyDecision{NextLevel: LevelL1, Action: DecisionRetry}
}

func elapsed(start, now time.Time) time.Duration {
	if start.IsZero() || now.Before(start) {
		return 0
	}

	return now.Sub(start)
}

func normalizeThresholds(thresholds AndonThresholds) AndonThresholds {
	defaults := DefaultThresholdDefinition()

	if thresholds.L1MaxRetries <= 0 {
		thresholds.L1MaxRetries = defaults.L1MaxRetries
	}
	if thresholds.L1MaxDuration <= 0 {
		thresholds.L1MaxDuration = defaults.L1MaxDuration
	}
	if thresholds.L2MaxDuration <= 0 {
		thresholds.L2MaxDuration = defaults.L2MaxDuration
	}
	if thresholds.MaxAssumptions <= 0 {
		thresholds.MaxAssumptions = defaults.MaxAssumptions
	}

	return thresholds
}
