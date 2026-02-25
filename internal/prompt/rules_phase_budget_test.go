package prompt

import (
	"testing"
)

type rulesPhaseBudget struct {
	phase    string
	maxChars int
}

func rulesPhaseBudgetMatrix() []rulesPhaseBudget {
	return []rulesPhaseBudget{
		{phase: "red", maxChars: 5600},
		{phase: "build", maxChars: 9500},
		{phase: "green", maxChars: 5600},
		{phase: "refactor", maxChars: 5600},
		{phase: "review", maxChars: 6000},
		{phase: "plan", maxChars: 2000},
		{phase: "refine", maxChars: 2000},
		{phase: "retro", maxChars: 2000},
		{phase: "validate", maxChars: 2000},
	}
}

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

	for _, budget := range rulesPhaseBudgetMatrix() {
		budget := budget
		t.Run(budget.phase, func(t *testing.T) {
			result, err := r.LoadRulesForPhase(budget.phase)
			if err != nil {
				t.Fatalf("LoadRulesForPhase(%q): %v", budget.phase, err)
			}
			if len(result) > budget.maxChars {
				t.Errorf("phase %q: %d chars exceeds budget of %d", budget.phase, len(result), budget.maxChars)
			}
		})
	}
}
