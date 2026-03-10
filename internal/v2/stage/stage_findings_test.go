package stage

import "testing"

func TestStageRequestFindingsAcceptsDefinedFinding(t *testing.T) {
	cases := []struct {
		name    string
		finding Finding
	}{
		{
			name: "spec finding",
			finding: Finding{
				Severity:      SeverityCritical,
				Category:      CategoryBug,
				Scope:         ScopeSpec,
				Description:   "missing validation",
				AffectedFiles: []string{"internal/v2/loop/spec_loop.go"},
			},
		},
		{
			name: "general suggestion",
			finding: Finding{
				Severity:      SeveritySuggestion,
				Category:      CategoryQuality,
				Scope:         ScopeGeneral,
				Description:   "consider simplifying",
				AffectedFiles: []string{"internal/v2/loop/spec_loop_test.go"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := StageRequest{Findings: []Finding{tc.finding}}
			if len(req.Findings) != 1 {
				t.Fatalf("expected 1 finding, got %d", len(req.Findings))
			}
			got := req.Findings[0]
			if got.Severity != tc.finding.Severity {
				t.Fatalf("severity = %q, want %q", got.Severity, tc.finding.Severity)
			}
			if got.Category != tc.finding.Category {
				t.Fatalf("category = %q, want %q", got.Category, tc.finding.Category)
			}
			if got.Scope != tc.finding.Scope {
				t.Fatalf("scope = %q, want %q", got.Scope, tc.finding.Scope)
			}
			if got.Description != tc.finding.Description {
				t.Fatalf("description = %q, want %q", got.Description, tc.finding.Description)
			}
			if len(got.AffectedFiles) != len(tc.finding.AffectedFiles) {
				t.Fatalf("affected files len = %d, want %d", len(got.AffectedFiles), len(tc.finding.AffectedFiles))
			}
			for idx, file := range got.AffectedFiles {
				if file != tc.finding.AffectedFiles[idx] {
					t.Fatalf("affected files[%d] = %q, want %q", idx, file, tc.finding.AffectedFiles[idx])
				}
			}
		})
	}
}
