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
