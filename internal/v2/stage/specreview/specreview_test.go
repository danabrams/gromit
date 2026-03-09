package specreview

import (
    "testing"

    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestVerdictFromFindings(t *testing.T) {
    cases := []struct {
        name     string
        findings []SpecReviewFinding
        want     string
    }{
        {
            name:     "issue present",
            findings: []SpecReviewFinding{{Verdict: "issue"}},
            want:     "issue",
        },
        {
            name: "all pass",
            findings: []SpecReviewFinding{
                {Verdict: "pass"},
                {Verdict: "pass"},
            },
            want: "pass",
        },
        {
            name: "no findings",
            want: "pass",
        },
    }

    for _, tt := range cases {
        t.Run(tt.name, func(t *testing.T) {
            got := verdictFromFindings(tt.findings)
            if got != tt.want {
                t.Fatalf("%s: verdict = %q, want %q", tt.name, got, tt.want)
            }
        })
    }
}

func TestParseSpecReviewOutput(t *testing.T) {
    source := `{
        "findings": [
            {
                "verdict": "issue",
                "severity": "critical",
                "category": "quality",
                "scope": "prompt rendering",
                "description": "Missing validation in spec review",
                "affected_files": ["cmd/gromit/review_spec_validation_acceptance_test.go"]
            },
            {
                "verdict": "pass",
                "severity": "low",
                "category": "test_coverage",
                "scope": "test coverage",
                "description": "Tests look good",
                "affected_files": []
            }
        ],
        "summary": " Spec-level review summary. "
    }`

    artifacts, err := parseSpecReviewOutput(source)
    if err != nil {
        t.Fatalf("parse spec review output: %v", err)
    }

    if artifacts.Summary != "Spec-level review summary." {
        t.Fatalf("summary = %q, want %q", artifacts.Summary, "Spec-level review summary.")
    }
    if artifacts.Verdict != "issue" {
        t.Fatalf("verdict = %q, want issue", artifacts.Verdict)
    }
    if got := len(artifacts.Findings); got != 2 {
        t.Fatalf("findings count = %d, want 2", got)
    }

    first := artifacts.Findings[0]
    if first.Title != "prompt rendering" {
        t.Fatalf("first finding title = %q, want %q", first.Title, "prompt rendering")
    }
    if first.Description != "Missing validation in spec review" {
        t.Fatalf("description = %q", first.Description)
    }
    if first.Severity != stagepkg.SpecFindingSeverityCritical {
        t.Fatalf("severity = %q, want %q", first.Severity, stagepkg.SpecFindingSeverityCritical)
    }
    if first.Category != stagepkg.SpecFindingCategoryQuality {
        t.Fatalf("category = %q, want %q", first.Category, stagepkg.SpecFindingCategoryQuality)
    }
    if len(first.AffectedFiles) != 1 || first.AffectedFiles[0] != "cmd/gromit/review_spec_validation_acceptance_test.go" {
        t.Fatalf("affected files = %v", first.AffectedFiles)
    }
}
