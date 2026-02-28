package prepare

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestCheckCriteriaPresence(t *testing.T) {
	tests := []struct {
		name        string
		bead        *bead.Bead
		wantOutcome ReadinessOutcome
		wantReason  string
	}{
		{
			name:        "missing criteria",
			bead:        &bead.Bead{},
			wantOutcome: ReadinessOutcomeNotReadyCriteria,
			wantReason:  ReasonCriteriaMissing,
		},
		{
			name: "expected outputs satisfy criteria",
			bead: &bead.Bead{
				ExpectedOutputs: []string{"artifact"},
			},
			wantOutcome: ReadinessOutcomeReady,
		},
		{
			name: "acceptance criteria fallback",
			bead: &bead.Bead{
				AcceptanceCriteria: "line one\nline two",
			},
			wantOutcome: ReadinessOutcomeReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome, reason := CheckCriteriaPresence(tt.bead)
			if outcome != tt.wantOutcome {
				t.Fatalf("outcome = %v, want %v", outcome, tt.wantOutcome)
			}
			if reason != tt.wantReason {
				t.Fatalf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}

func TestCheckCriteriaCount(t *testing.T) {
	tests := []struct {
		name        string
		bead        *bead.Bead
		wantOutcome ReadinessOutcome
		wantReason  string
	}{
		{
			name: "too many outputs",
			bead: &bead.Bead{
				ExpectedOutputs: []string{"a", "b", "c", "d"},
			},
			wantOutcome: ReadinessOutcomeNotReadyCriteria,
			wantReason:  ReasonCriteriaAmbiguous,
		},
		{
			name: "within limit",
			bead: &bead.Bead{
				ExpectedOutputs: []string{"alpha", "beta", "gamma"},
			},
			wantOutcome: ReadinessOutcomeReady,
		},
		{
			name: "acceptance criteria fallback",
			bead: &bead.Bead{
				AcceptanceCriteria: "one\ntwo\nthree\nfour",
			},
			wantOutcome: ReadinessOutcomeNotReadyCriteria,
			wantReason:  ReasonCriteriaAmbiguous,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome, reason := CheckCriteriaCount(tt.bead)
			if outcome != tt.wantOutcome {
				t.Fatalf("outcome = %v, want %v", outcome, tt.wantOutcome)
			}
			if reason != tt.wantReason {
				t.Fatalf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}

func TestCheckExpectedOutputs(t *testing.T) {
	tests := []struct {
		name        string
		bead        *bead.Bead
		wantOutcome ReadinessOutcome
		wantReason  string
	}{
		{
			name: "within scope limits",
			bead: &bead.Bead{
				ExpectedOutputs: []string{"o1", "o2", "o3", "o4", "o5"},
			},
			wantOutcome: ReadinessOutcomeReady,
		},
		{
			name: "exceeds scope limit",
			bead: &bead.Bead{
				ExpectedOutputs: []string{"a", "b", "c", "d", "e", "f"},
			},
			wantOutcome: ReadinessOutcomeNotReadyScope,
			wantReason:  ReasonScopeTooBroad,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome, reason := CheckExpectedOutputs(tt.bead)
			if outcome != tt.wantOutcome {
				t.Fatalf("outcome = %v, want %v", outcome, tt.wantOutcome)
			}
			if reason != tt.wantReason {
				t.Fatalf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}
