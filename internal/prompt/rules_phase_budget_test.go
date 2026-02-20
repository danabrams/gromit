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
		{"build", 8000},
		{"review", 6000},
		{"plan", 2000},
		{"refine", 2000},
		{"retro", 2000},
		{"validate", 2000},
	}

	for _, tt := range tests {
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
