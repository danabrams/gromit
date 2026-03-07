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

func TestRenderPRBody_FailureIncludesSummaryAndRemainingWork(t *testing.T) {
	t.Parallel()

	summary := PresentationSummary{
		Success:        false,
		FailureSummary: "validation commands still failing",
		RemainingWork: []string{
			"Fix acceptance command 1",
			"Investigate flaky integration test",
		},
		SpecName: "spec-failure",
	}

	body := RenderPRBody(summary)
	if !strings.Contains(body, "validation commands still failing") {
		t.Fatalf("expected failure summary in body: %s", body)
	}
	if !strings.Contains(body, "Fix acceptance command 1") {
		t.Fatalf("expected remaining work entry in body: %s", body)
	}
	if !strings.Contains(body, "## Failure Summary") || !strings.Contains(body, "## Remaining Work") {
		t.Fatalf("expected headings in failure body: %s", body)
	}
	if !strings.Contains(body, "Spec: spec-failure") {
		t.Fatalf("expected spec context in failure body: %s", body)
	}
}
