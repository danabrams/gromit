package andon

import (
	"testing"
	"time"
)

func setupPolicyTest(t *testing.T) (time.Time, AndonThresholds) {
	t.Helper()
	return time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC), DefaultThresholds()
}

func assertDecision(t *testing.T, got PolicyDecision, wantLevel AndonLevel, wantAction Decision) {
	t.Helper()
	if got.NextLevel != wantLevel {
		t.Fatalf("NextLevel = %q, want %q", got.NextLevel, wantLevel)
	}
	if got.Action != wantAction {
		t.Fatalf("Action = %q, want %q", got.Action, wantAction)
	}
}

func assertFailureClass(t *testing.T, got FailureClass, want FailureClass) {
	t.Helper()
	if got != want {
		t.Fatalf("Class = %q, want %q", got, want)
	}
}

// TestDefaultThresholds_SpecAligned verifies the default Andon bounds used by the
// policy for L1/L2 autonomy.
func TestDefaultThresholds_SpecAligned(t *testing.T) {
	thresholds := DefaultThresholds()

	if thresholds.L1MaxRetries != defaultL1MaxRetries {
		t.Fatalf("L1MaxRetries = %d, want %d", thresholds.L1MaxRetries, defaultL1MaxRetries)
	}
	if thresholds.L1MaxDuration != defaultL1MaxDuration {
		t.Fatalf("L1MaxDuration = %v, want %v", thresholds.L1MaxDuration, defaultL1MaxDuration)
	}
	if thresholds.L2MaxDuration != defaultL2MaxDuration {
		t.Fatalf("L2MaxDuration = %v, want %v", thresholds.L2MaxDuration, defaultL2MaxDuration)
	}
	if thresholds.MaxAssumptions != defaultMaxAssumptions {
		t.Fatalf("MaxAssumptions = %d, want %d", thresholds.MaxAssumptions, defaultMaxAssumptions)
	}
}

// TestLevels_IncludeL1ToL4 verifies all Andon levels are represented.
func TestLevels_IncludeL1ToL4(t *testing.T) {
	levels := []AndonLevel{LevelL1, LevelL2, LevelL3, LevelL4}

	if len(levels) != 4 {
		t.Fatalf("levels length = %d, want 4", len(levels))
	}

	for i, level := range levels {
		if level == "" {
			t.Fatalf("levels[%d] is empty", i)
		}
	}
}

// TestClassifyFailure_SupportsAllAndonClasses verifies class routing entry-point
// coverage for all Andon failure classes in the spec.
func TestClassifyFailure_SupportsAllAndonClasses(t *testing.T) {
	tests := []struct {
		name   string
		signal FailureSignal
		want   FailureClass
	}{
		{
			name: "transient class",
			signal: FailureSignal{
				Kind:   FailureKindTimeout,
				Output: "context deadline exceeded",
			},
			want: FailureClassTransient,
		},
		{
			name: "workflow class",
			signal: FailureSignal{
				Kind:   FailureKindWorkflow,
				Output: "bd sync required before close",
			},
			want: FailureClassWorkflow,
		},
		{
			name: "quality class",
			signal: FailureSignal{
				Kind:   FailureKindQualityGate,
				Output: "go test ./... failed",
			},
			want: FailureClassQuality,
		},
		{
			name: "intent class",
			signal: FailureSignal{
				Kind:   FailureKindAmbiguousIntent,
				Output: "spec leaves behavior undefined",
			},
			want: FailureClassIntent,
		},
		{
			name: "data class",
			signal: FailureSignal{
				Kind:   FailureKindIntegrity,
				Output: "state divergence detected",
			},
			want: FailureClassData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyFailure(tt.signal)
			if got != tt.want {
				t.Fatalf("ClassifyFailure() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestChooseNextAction_EnforcesL1AndL2Bounds verifies L1 and L2 thresholds are
// represented and enforced in policy decisions.
func TestChooseNextAction_EnforcesL1AndL2Bounds(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	t.Run("L1 within retry/time bounds stays in L1", func(t *testing.T) {
		state := RecoveryState{
			Class:      FailureClassTransient,
			Level:      LevelL1,
			L1Attempts: 1,
			L1Started:  now.Add(-1 * time.Minute),
		}

		decision := ChooseNextAction(state, thresholds, now)
		assertDecision(t, decision, LevelL1, DecisionRetry)
	})

	t.Run("L1 retry cap escalates to L2", func(t *testing.T) {
		state := RecoveryState{
			Class:      FailureClassTransient,
			Level:      LevelL1,
			L1Attempts: thresholds.L1MaxRetries,
			L1Started:  now.Add(-1 * time.Minute),
		}

		decision := ChooseNextAction(state, thresholds, now)
		assertDecision(t, decision, LevelL2, DecisionEscalate)
	})

	t.Run("L2 timebox exhaustion escalates to L3", func(t *testing.T) {
		state := RecoveryState{
			Class:     FailureClassQuality,
			Level:     LevelL2,
			L2Started: now.Add(-16 * time.Minute),
		}

		decision := ChooseNextAction(state, thresholds, now)
		assertDecision(t, decision, LevelL3, DecisionStopLine)
	})
}

// TestChooseNextAction_HasDecisionPathPerFailureClass verifies at least one policy
// decision path for each Andon failure class.
func TestChooseNextAction_HasDecisionPathPerFailureClass(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	tests := []struct {
		name  string
		state RecoveryState
		want  PolicyDecision
	}{
		{
			name: "Transient retries in L1 when below limits",
			state: RecoveryState{
				Class:      FailureClassTransient,
				Level:      LevelL1,
				L1Attempts: 0,
				L1Started:  now,
			},
			want: PolicyDecision{NextLevel: LevelL1, Action: DecisionRetry},
		},
		{
			name: "Workflow escalates from L1 to L2 after deterministic repair",
			state: RecoveryState{
				Class:      FailureClassWorkflow,
				Level:      LevelL1,
				L1Attempts: 1,
				L1Started:  now,
			},
			want: PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate},
		},
		{
			name: "Quality in L2 beyond 15 minutes escalates to L3",
			state: RecoveryState{
				Class:     FailureClassQuality,
				Level:     LevelL2,
				L2Started: now.Add(-16 * time.Minute),
			},
			want: PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine},
		},
		{
			name: "Intent unresolved after assumption budget escalates to L3",
			state: RecoveryState{
				Class:           FailureClassIntent,
				Level:           LevelL1,
				AssumptionsUsed: thresholds.MaxAssumptions,
			},
			want: PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate},
		},
		{
			name: "Data integrity risk triggers immediate stop-line",
			state: RecoveryState{
				Class: FailureClassData,
				Level: LevelL1,
			},
			want: PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := ChooseNextAction(tt.state, thresholds, now)
			assertDecision(t, decision, tt.want.NextLevel, tt.want.Action)
		})
	}
}

func TestEvaluateFailure_ClassifiesTransientAndChoosesDecision(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	got := EvaluateFailure(
		FailureSignal{
			Kind:   FailureKindTimeout,
			Output: "context deadline exceeded",
		},
		RecoveryState{
			Level:      LevelL1,
			L1Attempts: 0,
			L1Started:  now,
		},
		thresholds,
		now,
	)

	assertFailureClass(t, got.Class, FailureClassTransient)
	assertDecision(t, got.Decision, LevelL1, DecisionRetry)
}

func TestEvaluateFailure_ClassifiesWorkflowAndEscalatesFromL1(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	got := EvaluateFailure(
		FailureSignal{
			Kind:   FailureKindWorkflow,
			Output: "bd sync required before close",
		},
		RecoveryState{
			Level:      LevelL1,
			L1Attempts: 1,
			L1Started:  now,
		},
		thresholds,
		now,
	)

	assertFailureClass(t, got.Class, FailureClassWorkflow)
	assertDecision(t, got.Decision, LevelL2, DecisionEscalate)
}

func TestEvaluateFailure_ClassifiesQualityAndStopsLineFromL2(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	got := EvaluateFailure(
		FailureSignal{
			Kind:   FailureKindQualityGate,
			Output: "go test ./... failed",
		},
		RecoveryState{
			Level:     LevelL2,
			L2Started: now.Add(-16 * time.Minute),
		},
		thresholds,
		now,
	)

	assertFailureClass(t, got.Class, FailureClassQuality)
	assertDecision(t, got.Decision, LevelL3, DecisionStopLine)
}

func TestEvaluateFailure_ClassifiesIntentAndEscalatesWhenAssumptionsExhausted(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	got := EvaluateFailure(
		FailureSignal{
			Kind:   FailureKindAmbiguousIntent,
			Output: "spec leaves behavior undefined",
		},
		RecoveryState{
			Level:           LevelL1,
			AssumptionsUsed: thresholds.MaxAssumptions,
		},
		thresholds,
		now,
	)

	assertFailureClass(t, got.Class, FailureClassIntent)
	assertDecision(t, got.Decision, LevelL3, DecisionEscalate)
}

func TestEvaluateFailure_ClassifiesDataAndStopsLineImmediately(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	got := EvaluateFailure(
		FailureSignal{
			Kind:   FailureKindIntegrity,
			Output: "state divergence detected",
		},
		RecoveryState{
			Level: LevelL1,
		},
		thresholds,
		now,
	)

	assertFailureClass(t, got.Class, FailureClassData)
	assertDecision(t, got.Decision, LevelL3, DecisionStopLine)
}

func TestEvaluateFailure_EnforcesL1ToL2BoundaryAtRetryCap(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	got := EvaluateFailure(
		FailureSignal{
			Kind:   FailureKindTimeout,
			Output: "context deadline exceeded",
		},
		RecoveryState{
			Level:      LevelL1,
			L1Attempts: thresholds.L1MaxRetries,
			L1Started:  now,
		},
		thresholds,
		now,
	)

	assertFailureClass(t, got.Class, FailureClassTransient)
	assertDecision(t, got.Decision, LevelL2, DecisionEscalate)
}

func TestEvaluateFailure_UsesWorkflowDecisionPath(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	got := EvaluateFailure(
		FailureSignal{
			Kind:   FailureKindWorkflow,
			Output: "missing bd sync before close",
		},
		RecoveryState{
			Level:      LevelL1,
			L1Attempts: 1,
			L1Started:  now,
		},
		thresholds,
		now,
	)

	if got.Path != DecisionPathWorkflowEscalateAfterDeterministicAttempt {
		t.Fatalf("Path = %q, want %q", got.Path, DecisionPathWorkflowEscalateAfterDeterministicAttempt)
	}
}

func TestEvaluateClassifiedFailure_HasExplicitPathPerClass(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	tests := []struct {
		name           string
		classification PolicyClassification
		state          RecoveryState
		wantPath       DecisionPath
	}{
		{
			name:           "transient",
			classification: PolicyClassification{Class: FailureClassTransient},
			state:          RecoveryState{Level: LevelL1, L1Attempts: 0, L1Started: now},
			wantPath:       DecisionPathTransientL1Retry,
		},
		{
			name:           "workflow",
			classification: PolicyClassification{Class: FailureClassWorkflow},
			state:          RecoveryState{Level: LevelL1, L1Attempts: 1, L1Started: now},
			wantPath:       DecisionPathWorkflowEscalateAfterDeterministicAttempt,
		},
		{
			name:           "quality",
			classification: PolicyClassification{Class: FailureClassQuality},
			state:          RecoveryState{Level: LevelL2, L2Started: now.Add(-16 * time.Minute)},
			wantPath:       DecisionPathQualityStopLineAfterTimebox,
		},
		{
			name:           "intent",
			classification: PolicyClassification{Class: FailureClassIntent},
			state:          RecoveryState{Level: LevelL1, AssumptionsUsed: thresholds.MaxAssumptions},
			wantPath:       DecisionPathIntentEscalateAfterAssumptionBudget,
		},
		{
			name:           "data",
			classification: PolicyClassification{Class: FailureClassData},
			state:          RecoveryState{Level: LevelL1},
			wantPath:       DecisionPathDataImmediateStopLine,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateClassifiedFailure(tt.classification, tt.state, thresholds, now)
			if got.Path != tt.wantPath {
				t.Fatalf("Path = %q, want %q", got.Path, tt.wantPath)
			}
		})
	}
}

func TestEvaluateFailure_UnknownKindUsesDeterministicWorkflowFallbackPath(t *testing.T) {
	now, thresholds := setupPolicyTest(t)
	signal := FailureSignal{
		Kind:   FailureKind("new_kind_not_yet_classified"),
		Output: "unrecognized envelope",
	}
	state := RecoveryState{
		Level:      LevelL1,
		L1Attempts: 1,
		L1Started:  now,
	}

	first := EvaluateFailure(signal, state, thresholds, now)
	second := EvaluateFailure(signal, state, thresholds, now)

	if first != second {
		t.Fatalf("EvaluateFailure must be deterministic for unknown kind: first=%+v second=%+v", first, second)
	}
	if first.Class != FailureClassWorkflow {
		t.Fatalf("Class = %q, want %q", first.Class, FailureClassWorkflow)
	}
	if first.Path != DecisionPathWorkflowFallbackForUnknownKind {
		t.Fatalf("Path = %q, want %q", first.Path, DecisionPathWorkflowFallbackForUnknownKind)
	}
	assertDecision(t, first.Decision, LevelL2, DecisionEscalate)
}

func TestDecisionPathForClass_ReturnsCanonicalPathPerClass(t *testing.T) {
	tests := []struct {
		name  string
		class FailureClass
		want  DecisionPath
	}{
		{name: "transient", class: FailureClassTransient, want: DecisionPathTransientL1Retry},
		{name: "workflow", class: FailureClassWorkflow, want: DecisionPathWorkflowEscalateAfterDeterministicAttempt},
		{name: "quality", class: FailureClassQuality, want: DecisionPathQualityStopLineAfterTimebox},
		{name: "intent", class: FailureClassIntent, want: DecisionPathIntentEscalateAfterAssumptionBudget},
		{name: "data", class: FailureClassData, want: DecisionPathDataImmediateStopLine},
		{name: "unknown", class: FailureClass("unknown"), want: DecisionPathWorkflowFallbackForUnknownKind},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecisionPathForClass(tt.class)
			if got != tt.want {
				t.Fatalf("DecisionPathForClass(%q) = %q, want %q", tt.class, got, tt.want)
			}
		})
	}
}

func TestEvaluateFailureWithTrace_CoversDecisionPathForEachClass(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	tests := []struct {
		name     string
		signal   FailureSignal
		state    RecoveryState
		wantPath DecisionPath
		want     PolicyDecision
	}{
		{
			name:     "Transient path",
			signal:   FailureSignal{Kind: FailureKindTimeout, Output: "context deadline exceeded"},
			state:    RecoveryState{Level: LevelL1, L1Attempts: 0, L1Started: now},
			wantPath: DecisionPathTransientL1Retry,
			want:     PolicyDecision{NextLevel: LevelL1, Action: DecisionRetry},
		},
		{
			name:     "Workflow path",
			signal:   FailureSignal{Kind: FailureKindWorkflow, Output: "missing bd sync before close"},
			state:    RecoveryState{Level: LevelL1, L1Attempts: 1, L1Started: now},
			wantPath: DecisionPathWorkflowEscalateAfterDeterministicAttempt,
			want:     PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate},
		},
		{
			name:     "Quality path",
			signal:   FailureSignal{Kind: FailureKindQualityGate, Output: "go test ./... failed"},
			state:    RecoveryState{Level: LevelL2, L2Started: now.Add(-16 * time.Minute)},
			wantPath: DecisionPathQualityStopLineAfterTimebox,
			want:     PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine},
		},
		{
			name:     "Intent path",
			signal:   FailureSignal{Kind: FailureKindAmbiguousIntent, Output: "requirements conflict"},
			state:    RecoveryState{Level: LevelL1, AssumptionsUsed: thresholds.MaxAssumptions},
			wantPath: DecisionPathIntentEscalateAfterAssumptionBudget,
			want:     PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate},
		},
		{
			name:     "Data path",
			signal:   FailureSignal{Kind: FailureKindIntegrity, Output: "state divergence detected"},
			state:    RecoveryState{Level: LevelL1},
			wantPath: DecisionPathDataImmediateStopLine,
			want:     PolicyDecision{NextLevel: LevelL3, Action: DecisionStopLine},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := EvaluateFailureWithTrace(tt.signal, tt.state, thresholds, now)
			if trace.Decision != tt.want {
				t.Fatalf("EvaluateFailureWithTrace(...).Decision = %+v, want %+v", trace.Decision, tt.want)
			}
			if trace.Path != tt.wantPath {
				t.Fatalf("EvaluateFailureWithTrace(...).Path = %q, want %q", trace.Path, tt.wantPath)
			}
			if trace.DecisionInputSource != DecisionInputSourceClassifier {
				t.Fatalf("EvaluateFailureWithTrace(...).DecisionInputSource = %q, want %q", trace.DecisionInputSource, DecisionInputSourceClassifier)
			}
		})
	}
}

func TestEvaluateFailureWithTrace_EnforcesL1ToL2ThresholdBoundaryAtPublicEntryPoint(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	trace := EvaluateFailureWithTrace(
		FailureSignal{Kind: FailureKindTimeout, Output: "transient timeout"},
		RecoveryState{
			Level:      LevelL1,
			L1Attempts: thresholds.L1MaxRetries,
			L1Started:  now,
		},
		thresholds,
		now,
	)

	if trace.Classification.Class != FailureClassTransient {
		t.Fatalf("EvaluateFailureWithTrace(...).Classification.Class = %q, want %q", trace.Classification.Class, FailureClassTransient)
	}
	if trace.Decision.NextLevel != LevelL2 {
		t.Fatalf("EvaluateFailureWithTrace(...).Decision.NextLevel = %q, want %q", trace.Decision.NextLevel, LevelL2)
	}
	if trace.Decision.Action != DecisionEscalate {
		t.Fatalf("EvaluateFailureWithTrace(...).Decision.Action = %q, want %q", trace.Decision.Action, DecisionEscalate)
	}
	if trace.DecisionInputSource != DecisionInputSourceClassifier {
		t.Fatalf("EvaluateFailureWithTrace(...).DecisionInputSource = %q, want %q", trace.DecisionInputSource, DecisionInputSourceClassifier)
	}
}

func TestEvaluateClassifiedFailureWithTrace_UsesProvidedClassWithoutBypass(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	trace := EvaluateClassifiedFailureWithTrace(
		PolicyClassification{Class: FailureClassWorkflow},
		RecoveryState{
			Class:      FailureClassTransient,
			Level:      LevelL1,
			L1Attempts: 1,
			L1Started:  now,
		},
		thresholds,
		now,
	)

	var _ PolicyDecisionTrace

	if trace.Classification.Class != FailureClassWorkflow {
		t.Fatalf("EvaluateClassifiedFailureWithTrace(...).Classification.Class = %q, want %q", trace.Classification.Class, FailureClassWorkflow)
	}
	if trace.Decision != (PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate}) {
		t.Fatalf("EvaluateClassifiedFailureWithTrace(...).Decision = %+v, want workflow escalation", trace.Decision)
	}
}

func TestEvaluateClassifiedFailureWithTrace_SetsClassifiedEntryInputSource(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	trace := EvaluateClassifiedFailureWithTrace(
		PolicyClassification{Class: FailureClassWorkflow},
		RecoveryState{
			Level:      LevelL1,
			L1Attempts: 1,
			L1Started:  now,
		},
		thresholds,
		now,
	)

	if trace.DecisionInputSource != DecisionInputSourceClassifiedEntry {
		t.Fatalf("EvaluateClassifiedFailureWithTrace(...).DecisionInputSource = %q, want %q", trace.DecisionInputSource, DecisionInputSourceClassifiedEntry)
	}
}

func TestEvaluateFailureWithTrace_IrreversibleMigrationRequiresApproval(t *testing.T) {
	now, thresholds := setupPolicyTest(t)

	trace := EvaluateFailureWithTrace(
		FailureSignal{
			Kind:   FailureKindHardStopIrreversibleMigration,
			Output: "alembic upgrade head --irreversible",
		},
		RecoveryState{
			Level:      LevelL1,
			L1Attempts: 0,
			L1Started:  now,
		},
		thresholds,
		now,
	)

	if trace.Decision != (PolicyDecision{NextLevel: LevelL3, Action: DecisionEscalate}) {
		t.Fatalf("EvaluateFailureWithTrace(...).Decision = %+v, want L3 escalation for irreversible migration", trace.Decision)
	}
	if trace.Path != DecisionPathHardStopRequiresApproval {
		t.Fatalf("EvaluateFailureWithTrace(...).Path = %q, want %q", trace.Path, DecisionPathHardStopRequiresApproval)
	}
}
