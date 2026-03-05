package presentation

import (
    "strings"
    "testing"

    v2review "github.com/danabrams/gromit/internal/v2/review"
)

func TestRenderPRBody_SuccessIncludesAcceptanceAndOutOfScope(t *testing.T) {
    t.Parallel()

    summary := PresentationSummary{
        Success: true,
        AcceptanceResults: []AcceptanceResult{
            {
                Title:       "Acceptance tests",
                Description: "All validation checks passed",
            },
        },
        OutOfScopeFindings: []v2review.Finding{
            {
                Title:         "Document audit drift",
                Description:   "Docs are out of date relative to the process",
                InScope:       false,
                AffectedFiles: []string{"docs/audit.md"},
            },
        },
    }

    body := RenderPRBody(summary)
    if !strings.Contains(body, "Acceptance tests") {
        t.Fatalf("expected acceptance details in body: %s", body)
    }
    if !strings.Contains(body, "Document audit drift") {
        t.Fatalf("expected out-of-scope title in body: %s", body)
    }
    if !strings.Contains(body, "docs/audit.md") {
        t.Fatalf("expected affected file list in body: %s", body)
    }
}
