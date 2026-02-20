package prompt

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/learnings"
)

func TestShapeATDDContextForBudget_Table(t *testing.T) {
	tests := []struct {
		name            string
		ctx             *Context
		cfg             ATDDPromptConfig
		wantSpec        string
		wantRules       string
		wantClaudeMD    string
		expectSpecExact bool
		wantRecent      int
		wantConfirmed   int
		wantTrimActions []string
		wantTruncSpec   bool
	}{
		{
			name: "toggles and confirmed cap applied before budget",
			ctx: &Context{
				Spec:               "spec-content",
				Rules:              "rules-content",
				ClaudeMD:           "claude-content",
				ConfirmedLearnings: []learnings.Learning{makeLearning("12345678901"), makeLearning("12345678901"), makeLearning("12345678901")},
				RecentLearnings:    []learnings.Learning{makeLearning("recent-one")},
			},
			cfg: ATDDPromptConfig{
				IncludeRules:              false,
				IncludeSpec:               false,
				IncludeClaudeMD:           false,
				MaxChars:                  10_000,
				MaxConfirmedLearningChars: 12,
			},
			wantSpec:        "",
			wantRules:       "",
			wantClaudeMD:    "",
			expectSpecExact: true,
			wantRecent:      1,
			wantConfirmed:   1,
			wantTrimActions: []string{
				trimDropATDDRules,
				trimDropATDDSpec,
				trimDropClaudeMD,
				trimCapATDDLearnings,
			},
		},
		{
			name: "drops recent, then confirms drop to budget then keeps ATDD rule subset",
			ctx: &Context{
				ClaudeMD: strings.Repeat("c", 10),
				Spec:     strings.Repeat("s", 20),
				Rules: "# Rules\n\n## Build <!-- phases: build -->\n" +
					strings.Repeat("r", 20) +
					"\n## Review <!-- phases: review -->\n" +
					strings.Repeat("r", 120),
				RecentLearnings: []learnings.Learning{makeLearning(strings.Repeat("r", 20))},
				ConfirmedLearnings: []learnings.Learning{
					makeLearning(strings.Repeat("l", 30)),
					makeLearning(strings.Repeat("l", 30)),
					makeLearning(strings.Repeat("l", 30)),
				},
			},
			cfg: ATDDPromptConfig{
				IncludeRules:              true,
				IncludeSpec:               true,
				IncludeClaudeMD:           true,
				MaxChars:                  120,
				MaxConfirmedLearningChars: 0,
			},
			wantRecent:      0,
			wantConfirmed:   0,
			wantClaudeMD:    strings.Repeat("c", 10),
			expectSpecExact: true,
			wantRules:       "# Rules\n\n## Build\n" + strings.Repeat("r", 20),
			wantSpec:        strings.Repeat("s", 20),
			wantTrimActions: []string{
				trimCapATDDLearnings,
				trimDropRecentLearnings,
				trimReplaceATDDRules,
			},
		},
		{
			name: "reduces confirmed learnings with deterministic action",
			ctx: &Context{
				ClaudeMD:        strings.Repeat("c", 10),
				Spec:            strings.Repeat("s", 40),
				Rules:           "#Rules\n## Build <!-- phases: build -->\n" + strings.Repeat("r", 10),
				RecentLearnings: []learnings.Learning{makeLearning(strings.Repeat("r", 20))},
				ConfirmedLearnings: []learnings.Learning{
					makeLearning(strings.Repeat("l", 10)),
					makeLearning(strings.Repeat("l", 10)),
					makeLearning(strings.Repeat("l", 10)),
				},
			},
			cfg: ATDDPromptConfig{
				IncludeRules:              true,
				IncludeSpec:               true,
				IncludeClaudeMD:           true,
				MaxChars:                  120,
				MaxConfirmedLearningChars: 200,
			},
			wantRecent:      0,
			wantConfirmed:   2,
			wantClaudeMD:    strings.Repeat("c", 10),
			expectSpecExact: true,
			wantSpec:        strings.Repeat("s", 40),
			wantRules:       "#Rules\n## Build <!-- phases: build -->\n" + strings.Repeat("r", 10),
			wantTrimActions: []string{
				trimDropRecentLearnings,
				trimCapConfirmedLearnings,
			},
		},
		{
			name: "truncates spec and drops rules last",
			ctx: &Context{
				ClaudeMD:           strings.Repeat("c", 10),
				Spec:               strings.Repeat("s", 500),
				Rules:              "Rules without build phases\n" + strings.Repeat("r", 500),
				RecentLearnings:    []learnings.Learning{makeLearning(strings.Repeat("r", 5))},
				ConfirmedLearnings: []learnings.Learning{makeLearning(strings.Repeat("l", 100))},
			},
			cfg: ATDDPromptConfig{
				IncludeRules:              true,
				IncludeSpec:               true,
				IncludeClaudeMD:           true,
				MaxChars:                  120,
				MaxConfirmedLearningChars: 500,
			},
			wantRecent:      0,
			wantConfirmed:   0,
			wantRules:       "",
			wantClaudeMD:    strings.Repeat("c", 10),
			expectSpecExact: false,
			wantTrimActions: []string{
				trimDropRecentLearnings,
				trimDropConfirmedLearnings,
				trimTruncateSpec,
				trimDropATDDRules,
			},
			wantTruncSpec: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.ctx.normalizeNilFields()
			shaped, report := ShapeATDDContextForBudget(tt.ctx, tt.cfg)

			if shaped == nil {
				t.Fatal("expected shaped context")
			}
			if tt.expectSpecExact && shaped.Spec != tt.wantSpec {
				t.Fatalf("Spec = %q, want %q", shaped.Spec, tt.wantSpec)
			}
			if shaped.Rules != tt.wantRules {
				t.Fatalf("Rules = %q, want %q", shaped.Rules, tt.wantRules)
			}
			if shaped.ClaudeMD != tt.wantClaudeMD {
				t.Fatalf("ClaudeMD = %q, want %q", shaped.ClaudeMD, tt.wantClaudeMD)
			}
			if len(shaped.RecentLearnings) != tt.wantRecent {
				t.Fatalf("RecentLearnings = %d, want %d", len(shaped.RecentLearnings), tt.wantRecent)
			}
			if len(shaped.ConfirmedLearnings) != tt.wantConfirmed {
				t.Fatalf("ConfirmedLearnings = %d, want %d", len(shaped.ConfirmedLearnings), tt.wantConfirmed)
			}

			if len(report.TrimActions) != len(tt.wantTrimActions) {
				t.Fatalf("TrimActions = %v, want %v", report.TrimActions, tt.wantTrimActions)
			}
			for i, wantAction := range tt.wantTrimActions {
				if report.TrimActions[i] != wantAction {
					t.Fatalf("TrimActions[%d] = %q, want %q", i, report.TrimActions[i], wantAction)
				}
			}

			if tt.wantTruncSpec && !strings.Contains(shaped.Spec, truncationMarker) {
				t.Fatalf("expected truncated marker in Spec, got %q", shaped.Spec)
			}
		})
	}
}
