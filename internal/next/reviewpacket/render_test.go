package reviewpacket

import (
	"strings"
	"testing"
)

func TestRenderProductReview(t *testing.T) {
	tests := []struct {
		name     string
		review   ProductReview
		contains []string
		notEmpty bool
	}{
		{
			name: "renders spec title and summary",
			review: ProductReview{
				RunID:      "run-123",
				SpecTitle:  "Add user authentication",
				Summary:    "Successfully implemented OAuth flow",
				TerminalState: "success",
			},
			contains: []string{
				"Add user authentication",
				"Successfully implemented OAuth flow",
			},
			notEmpty: true,
		},
		{
			name: "leads with behavior cards",
			review: ProductReview{
				SpecTitle: "Test spec",
				Summary:   "Test summary",
				BehaviorCards: []BehaviorCard{
					{
						ID:              "card-1",
						Title:           "Login flow works",
						Given:           "User at login page",
						When:            "User enters credentials",
						Then:            "User is authenticated",
						AutomaticStatus: "pass",
					},
					{
						ID:              "card-2",
						Title:           "Invalid credentials rejected",
						When:            "User enters wrong password",
						Then:            "Error message shown",
						AutomaticStatus: "fail",
					},
				},
			},
			contains: []string{
				"Login flow works",
				"Invalid credentials rejected",
				"card-1",
				"card-2",
			},
			notEmpty: true,
		},
		{
			name: "includes behavior card details",
			review: ProductReview{
				SpecTitle: "Test spec",
				Summary:   "Test summary",
				BehaviorCards: []BehaviorCard{
					{
						ID:              "card-1",
						Title:           "Test behavior",
						Given:           "Initial state",
						When:            "Action taken",
						Then:            "Expectation met",
						AutomaticStatus: "pass",
						EvidenceFiles:   []string{"test.log", "metrics.json"},
						Notes:           "All assertions passed",
					},
				},
			},
			contains: []string{
				"Initial state",
				"Action taken",
				"Expectation met",
				"pass",
				"test.log",
				"metrics.json",
				"All assertions passed",
			},
			notEmpty: true,
		},
		{
			name: "includes surprises when present",
			review: ProductReview{
				SpecTitle: "Test spec",
				Summary:   "Test summary",
				Surprises: []string{
					"Build time increased by 30%",
					"New warnings in linter",
				},
			},
			contains: []string{
				"Build time increased by 30%",
				"New warnings in linter",
			},
			notEmpty: true,
		},
		{
			name: "marks diagnostic reviews",
			review: ProductReview{
				SpecTitle: "Test spec",
				Summary:   "Test summary",
				IsDiagnostic: true,
			},
			contains: []string{"diagnostic"},
			notEmpty: true,
		},
		{
			name: "includes blocker and recommended action",
			review: ProductReview{
				SpecTitle:          "Test spec",
				Summary:            "Test summary",
				BlockerSummary:     "Database migration failed",
				RecommendedNextAction: "Rollback and investigate schema changes",
			},
			contains: []string{
				"Database migration failed",
				"Rollback and investigate schema changes",
			},
			notEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderProductReview(tt.review)

			if tt.notEmpty && result == "" {
				t.Error("expected non-empty output, got empty string")
			}

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected output to contain %q, got:\n%s", expected, result)
				}
			}
		})
	}
}

func TestRenderProcessReview(t *testing.T) {
	tests := []struct {
		name     string
		review   ProcessReview
		contains []string
		notEmpty bool
	}{
		{
			name: "shows trust level and key fields",
			review: ProcessReview{
				TrustLevel:          "high",
				AutomaticProof:      "All tests passed",
				MachineReview:       "Code quality acceptable",
				Acceptance:          "Ready for production",
				RecommendedPosture:  "Deploy",
			},
			contains: []string{
				"high",
				"All tests passed",
				"Code quality acceptable",
				"Ready for production",
				"Deploy",
			},
			notEmpty: true,
		},
		{
			name: "includes degraded flags",
			review: ProcessReview{
				TrustLevel:         "medium",
				DegradedFlags:      []string{"slow_tests", "missing_coverage"},
				RecommendedPosture: "Deploy with caution",
			},
			contains: []string{
				"medium",
				"slow_tests",
				"missing_coverage",
				"Deploy with caution",
			},
			notEmpty: true,
		},
		{
			name: "includes repair cycles and repeated failures",
			review: ProcessReview{
				TrustLevel:          "low",
				RepairCycles:        3,
				RepeatedFailureFlag: true,
				RecommendedPosture:  "Hold for investigation",
			},
			contains: []string{
				"low",
				"3",
				"repeated",
				"Hold for investigation",
			},
			notEmpty: true,
		},
		{
			name: "handles empty degraded flags",
			review: ProcessReview{
				TrustLevel:         "high",
				AutomaticProof:     "All checks passed",
				MachineReview:      "No issues",
				Acceptance:         "Approved",
				DegradedFlags:      []string{},
				RepairCycles:       0,
				RepeatedFailureFlag: false,
				RecommendedPosture: "Deploy",
			},
			contains: []string{
				"high",
				"All checks passed",
			},
			notEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderProcessReview(tt.review)

			if tt.notEmpty && result == "" {
				t.Error("expected non-empty output, got empty string")
			}

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected output to contain %q, got:\n%s", expected, result)
				}
			}
		})
	}
}
