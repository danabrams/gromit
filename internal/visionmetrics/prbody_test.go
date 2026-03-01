package visionmetrics

import (
	"strings"
	"testing"
)

func TestParseFromPRBody_VisionMetricsBlockExtraction(t *testing.T) {
	const knownGood = `
# Vision Metrics

spec_id: spec-2026-011
cycle_start_trigger_at: 2026-02-24T10:00:00Z
cycle_end_presented_at: 2026-02-27T14:00:00Z
review_outcome: rework_vision_change
review_rationale: Product owner shifted direction after the first presentation.
human_tactical_intervention: yes
human_debugging_intervention: no
escaped_regression_within_7d: pending

Some follow-up notes.
`

	cases := []struct {
		name          string
		body          string
		wantErr       bool
		errContains   string
		wantSpec      string
		wantOutcome   ReviewOutcome
		wantRationale string
		wantEscaped   YesNo
	}{
		{
			name:        "missing Vision Metrics block",
			body:        "spec_id: spec-2026-777\ncycle_start_trigger_at: 2026-02-25T10:00:00Z\n",
			wantErr:     true,
			errContains: "Vision Metrics block",
		},
		{
			name: "malformed Vision Metrics block",
			body: `
# Vision Metrics

spec_id spec-2026-001
cycle_start_trigger_at: 2026-02-24T10:00:00Z
`,
			wantErr:     true,
			errContains: "malformed Vision Metrics block",
		},
		{
			name:          "known-good Vision Metrics example",
			body:          knownGood,
			wantErr:       false,
			wantSpec:      "spec-2026-011",
			wantOutcome:   ReviewOutcomeVisionChange,
			wantRationale: "Product owner shifted direction after the first presentation.",
			wantEscaped:   EscapedRegressionPending,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rec, err := ParseFromPRBody(tc.body)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %s", tc.name)
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("expected error to mention %q, got %v", tc.errContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantSpec != "" && rec.SpecID != tc.wantSpec {
				t.Fatalf("spec mismatch: got %q", rec.SpecID)
			}
			if tc.wantOutcome != "" && rec.ReviewOutcome != tc.wantOutcome {
				t.Fatalf("review_outcome mismatch: got %v", rec.ReviewOutcome)
			}
			if tc.wantRationale != "" && rec.ReviewRationale != tc.wantRationale {
				t.Fatalf("review_rationale mismatch: got %q", rec.ReviewRationale)
			}
			if tc.wantEscaped != "" && rec.EscapedRegressionWithin7D != tc.wantEscaped {
				t.Fatalf("escaped_regression status mismatch: got %v", rec.EscapedRegressionWithin7D)
			}
		})
	}
}
