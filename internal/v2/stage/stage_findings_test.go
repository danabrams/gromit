package stage

import "testing"

func TestStageRequestFindingsAcceptsDefinedFinding(t *testing.T) {
	req := StageRequest{
		Findings: []Finding{
			{
				Severity:      SeverityCritical,
				Category:      CategoryBug,
				Scope:         ScopeSpec,
				Description:   "missing validation",
				AffectedFiles: []string{"internal/v2/loop/spec_loop.go"},
			},
		},
	}

	if len(req.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(req.Findings))
	}

	finding := req.Findings[0]
	if finding.Severity != SeverityCritical {
		t.Fatalf("expected severity to be %q, got %q", SeverityCritical, finding.Severity)
	}
	if finding.Category != CategoryBug {
		t.Fatalf("expected category to be %q, got %q", CategoryBug, finding.Category)
	}
	if finding.Scope != ScopeSpec {
		t.Fatalf("expected scope to be %q, got %q", ScopeSpec, finding.Scope)
	}
	if finding.Description != "missing validation" {
		t.Fatalf("unexpected description: %s", finding.Description)
	}
	if len(finding.AffectedFiles) != 1 || finding.AffectedFiles[0] != "internal/v2/loop/spec_loop.go" {
		t.Fatalf("unexpected affected files: %v", finding.AffectedFiles)
	}
}
