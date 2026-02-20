package prompt

import (
	"testing"
)

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
