package prompt

import (
	"testing"
)

func TestRulesPhaseCharBudgets_CoversMethodologyPhases(t *testing.T) {
	budgets := rulesPhaseBudgetMatrix()
	seen := make(map[string]bool, len(budgets))
	for _, budget := range budgets {
		seen[budget.phase] = true
	}

	for _, phase := range []string{"red", "green", "refactor"} {
		if !seen[phase] {
			t.Fatalf("rules phase budget table missing %q phase", phase)
		}
	}
}

func TestRulesPhaseCharBudgets(t *testing.T) {
	r := &Renderer{
		rulesPath: "../../.gromit/RULES.md",
		gromitDir: "../../.gromit",
	}

	tests := []struct {
		phase    string
		maxChars int
	}{
		{phase: "build", maxChars: 8000},
		{phase: "review", maxChars: 6000},
		{phase: "plan", maxChars: 2000},
		{phase: "refine", maxChars: 2000},
		{phase: "retro", maxChars: 2000},
		{phase: "validate", maxChars: 2000},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.phase, func(t *testing.T) {
			result, err := r.LoadRulesForPhase(tt.phase)
			if err != nil {
				t.Fatalf("LoadRulesForPhase(%q): %v", tt.phase, err)
			}
			if len(result) > tt.maxChars {
				t.Errorf("phase %q: %d chars exceeds budget of %d", tt.phase, len(result), tt.maxChars)
			}
		})
	}
}
