package reviewpacket

import (
	"strings"
	"testing"
)

func TestGenerateSummary(t *testing.T) {
	tests := []struct {
		name             string
		terminalState    string
		behaviorCardLen  int
		behaviorStatus   map[string]int // status -> count
		acceptancePassed int
		acceptanceTotal  int
		repairCycles     int
		degradedFlags    []string
		expectedSubstr   string
	}{
		{
			name:             "ready_for_review with all proven behaviors",
			terminalState:    "ready_for_review",
			behaviorCardLen:  6,
			behaviorStatus:   map[string]int{"proven": 6},
			acceptancePassed: 5,
			acceptanceTotal:  5,
			repairCycles:     0,
			degradedFlags:    []string{},
			expectedSubstr:   "6 behaviors verified",
		},
		{
			name:             "ready_for_review with mixed behaviors",
			terminalState:    "ready_for_review",
			behaviorCardLen:  4,
			behaviorStatus:   map[string]int{"proven": 2, "mixed": 2},
			acceptancePassed: 4,
			acceptanceTotal:  5,
			repairCycles:     1,
			degradedFlags:    []string{"diff_unavailable"},
			expectedSubstr:   "4 behaviors verified",
		},
		{
			name:             "blocked run diagnostic summary",
			terminalState:    "blocked",
			behaviorCardLen:  3,
			behaviorStatus:   map[string]int{"unclear": 3},
			acceptancePassed: 0,
			acceptanceTotal:  3,
			repairCycles:     3,
			degradedFlags:    []string{},
			expectedSubstr:   "Run blocked",
		},
		{
			name:             "needs_human run diagnostic summary",
			terminalState:    "needs_human",
			behaviorCardLen:  2,
			behaviorStatus:   map[string]int{"unclear": 2},
			acceptancePassed: 1,
			acceptanceTotal:  2,
			repairCycles:     2,
			degradedFlags:    []string{},
			expectedSubstr:   "Run needs human review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := GenerateSummary(GenerateSummaryInput{
				TerminalState:    tt.terminalState,
				BehaviorCardLen:  tt.behaviorCardLen,
				BehaviorStatus:   tt.behaviorStatus,
				AcceptancePassed: tt.acceptancePassed,
				AcceptanceTotal:  tt.acceptanceTotal,
				RepairCycles:     tt.repairCycles,
				DegradedFlags:    tt.degradedFlags,
			})

			if summary == "" {
				t.Errorf("GenerateSummary returned empty string")
			}

			if tt.expectedSubstr != "" && !strings.Contains(summary, tt.expectedSubstr) {
				t.Errorf("expected summary to contain %q, got %q", tt.expectedSubstr, summary)
			}
		})
	}
}
