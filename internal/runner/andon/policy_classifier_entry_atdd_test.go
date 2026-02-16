package andon

import (
	"testing"
	"time"
)

func setupPolicyClassifierEntryATDD(t *testing.T) (time.Time, AndonThresholds) {
	t.Helper()
	return time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC), DefaultThresholdDefinition()
}

// Expected failure: PolicyClassification and ClassifyFailureEntry do not exist yet.
func TestClassifyFailureEntry_ReturnsOnlyCanonicalClasses(t *testing.T) {
	allowed := map[FailureClass]struct{}{
		FailureClassTransient: {},
		FailureClassWorkflow:  {},
		FailureClassQuality:   {},
		FailureClassIntent:    {},
		FailureClassData:      {},
	}

	cases := []FailureSignal{
		{Kind: FailureKindTimeout, Output: "context deadline exceeded"},
		{Kind: FailureKindWorkflow, Output: "missing bd sync before close"},
		{Kind: FailureKindQualityGate, Output: "go test ./... failed"},
		{Kind: FailureKindAmbiguousIntent, Output: "spec leaves behavior undefined"},
		{Kind: FailureKindIntegrity, Output: "state divergence detected"},
	}

	for _, signal := range cases {
		classified := ClassifyFailureEntry(signal)
		if _, ok := allowed[classified.Class]; !ok {
			t.Fatalf("ClassifyFailureEntry(%+v).Class = %q, want one of canonical classes", signal, classified.Class)
		}
	}

	var _ PolicyClassification
}

// Expected failure: ClassifyFailureEntry does not exist yet and representative mapping behavior is not implemented at the entry point.
func TestClassifyFailureEntry_MapsRepresentativeSignals(t *testing.T) {
	tests := []struct {
		name   string
		signal FailureSignal
		want   FailureClass
	}{
		{
			name:   "timeout maps to Transient",
			signal: FailureSignal{Kind: FailureKindTimeout, Output: "command timed out"},
			want:   FailureClassTransient,
		},
		{
			name:   "workflow issue maps to Workflow",
			signal: FailureSignal{Kind: FailureKindWorkflow, Output: "needs manual branch cleanup"},
			want:   FailureClassWorkflow,
		},
		{
			name:   "quality gate failure maps to Quality",
			signal: FailureSignal{Kind: FailureKindQualityGate, Output: "go vet ./... failed"},
			want:   FailureClassQuality,
		},
		{
			name:   "ambiguous intent maps to Intent",
			signal: FailureSignal{Kind: FailureKindAmbiguousIntent, Output: "requirements conflict"},
			want:   FailureClassIntent,
		},
		{
			name:   "data integrity issue maps to Data",
			signal: FailureSignal{Kind: FailureKindIntegrity, Output: "status file deserialization mismatch"},
			want:   FailureClassData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classified := ClassifyFailureEntry(tt.signal)
			if classified.Class != tt.want {
				t.Fatalf("ClassifyFailureEntry(%+v).Class = %q, want %q", tt.signal, classified.Class, tt.want)
			}
		})
	}
}

// Expected failure: EvaluateClassifiedFailure does not exist yet, so classified output is not yet wired into downstream decision selection.
func TestEvaluateClassifiedFailure_UsesClassificationForDecisionSelection(t *testing.T) {
	now, thresholds := setupPolicyClassifierEntryATDD(t)

	classification := PolicyClassification{Class: FailureClassWorkflow}
	state := RecoveryState{
		Level:      LevelL1,
		L1Attempts: 1,
		L1Started:  now,
	}

	got := EvaluateClassifiedFailure(classification, state, thresholds, now)
	want := PolicyDecision{NextLevel: LevelL2, Action: DecisionEscalate}
	if got.Decision != want {
		t.Fatalf("EvaluateClassifiedFailure(%+v, %+v, %+v, %v).Decision = %+v, want %+v", classification, state, thresholds, now, got.Decision, want)
	}
	if got.Class != FailureClassWorkflow {
		t.Fatalf("EvaluateClassifiedFailure(...).Class = %q, want %q", got.Class, FailureClassWorkflow)
	}
}
