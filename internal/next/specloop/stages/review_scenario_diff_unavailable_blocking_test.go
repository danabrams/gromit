package stages

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestScenario_DiffUnavailable_ReviewFindsBlockingIssues_ReplansFromFindings(t *testing.T) {
	// --- Seed ---
	// A run where DiffProvider.Diff() errors, but the LLM reviewer
	// identifies real blocking code quality issues from task results.
	evidenceDir := filepath.Join(t.TempDir(), "evidence")
	eventLogPath := filepath.Join(t.TempDir(), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings: []review.Finding{
				{Facet: "spec_alignment", Severity: review.SeverityError, File: "handler.go", Line: 55, Description: "acceptance criterion 'idempotent retries' not met: no dedup key checked"},
				{Facet: "code_quality", Severity: review.SeverityError, File: "service.go", Line: 12, Description: "missing error propagation from downstream call"},
				{Facet: "code_quality", Severity: review.SeverityInfo, File: "service.go", Line: 30, Description: "consider extracting helper"},
			},
			BlockingFindings: []review.Finding{
				{Facet: "spec_alignment", Severity: review.SeverityError, File: "handler.go", Line: 55, Description: "acceptance criterion 'idempotent retries' not met: no dedup key checked"},
				{Facet: "code_quality", Severity: review.SeverityError, File: "service.go", Line: 12, Description: "missing error propagation from downstream call"},
			},
			FindingsByFacet: map[string][]review.Finding{
				"spec_alignment": {
					{Facet: "spec_alignment", Severity: review.SeverityError, File: "handler.go", Line: 55, Description: "acceptance criterion 'idempotent retries' not met: no dedup key checked"},
				},
				"code_quality": {
					{Facet: "code_quality", Severity: review.SeverityError, File: "service.go", Line: 12, Description: "missing error propagation from downstream call"},
					{Facet: "code_quality", Severity: review.SeverityInfo, File: "service.go", Line: 30, Description: "consider extracting helper"},
				},
			},
			HasBlockingFindings: true,
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{
		DiffProvider: &fakeDiffProvider{err: errors.New("worktree has no commits yet")},
		EvidenceDir:  evidenceDir,
		SpecContent:  "spec: idempotent retries required",
		BaseBranch:   "main",
	}, eventLog)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	// --- Invoke ---
	action, err := stage.Run(context.Background(), rs)

	// --- Assert ---

	// No error returned — diff failure is gracefully degraded, not propagated.
	if err != nil {
		t.Fatalf("expected no error (graceful degradation), got: %v", err)
	}

	// Action must be ReplanFrom, driven by real review findings.
	if action.Kind != specloop.ReplanFrom {
		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
	}

	// FailureContext must contain the actual code quality / spec alignment issues.
	if action.Context == nil {
		t.Fatal("expected non-nil FailureContext")
	}
	if len(action.Context.Failures) == 0 {
		t.Fatal("expected failure messages from blocking findings")
	}

	// Failures should reference the real issues, NOT the diff error.
	failureText := strings.Join(action.Context.Failures, "\n")
	if strings.Contains(failureText, "worktree has no commits yet") {
		t.Error("failure context should not contain the diff error message — it should reflect review findings")
	}
	if !strings.Contains(failureText, "idempotent retries") && !strings.Contains(failureText, "missing error propagation") {
		t.Errorf("failure context should reference actual blocking findings, got:\n%s", failureText)
	}

	// RunState: FinalReviewPassed must be false.
	if rs.FinalReviewPassed {
		t.Error("expected FinalReviewPassed = false")
	}

	// RunState: ReviewFindings should contain only blocking findings (ReplanFrom path).
	if len(rs.ReviewFindings) != 2 {
		t.Errorf("expected 2 blocking findings in ReviewFindings, got %d", len(rs.ReviewFindings))
	}
	// ReviewFindings should not mention the diff error.
	for _, f := range rs.ReviewFindings {
		if strings.Contains(f, "diff unavailable") || strings.Contains(f, "worktree has no commits") {
			t.Errorf("ReviewFindings should not contain diff error, got: %q", f)
		}
	}

	// Event log: should contain both diff_unavailable and review_result events.
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	var hasDiffUnavailable, hasReviewResult bool
	for _, ev := range events {
		switch ev.EventType() {
		case "diff_unavailable":
			hasDiffUnavailable = true
			due := ev.(*runstore.DiffUnavailableEvent)
			if !strings.Contains(due.Reason, "worktree has no commits yet") {
				t.Errorf("DiffUnavailableEvent.Reason = %q, want it to contain the original error", due.Reason)
			}
		case "review_result":
			hasReviewResult = true
			rev := ev.(*runstore.ReviewResultEvent)
			if rev.BlockingFindings != 2 {
				t.Errorf("ReviewResultEvent.BlockingFindings = %d, want 2", rev.BlockingFindings)
			}
			if rev.TotalFindings != 3 {
				t.Errorf("ReviewResultEvent.TotalFindings = %d, want 3", rev.TotalFindings)
			}
		}
	}
	if !hasDiffUnavailable {
		t.Error("expected diff_unavailable event in log")
	}
	if !hasReviewResult {
		t.Error("expected review_result event in log")
	}

	// Evidence: review.json should exist with diff_unavailable=true and findings present.
	reviewJSON, err := os.ReadFile(filepath.Join(evidenceDir, "review.json"))
	if err != nil {
		t.Fatalf("read review.json: %v", err)
	}
	content := string(reviewJSON)
	if !strings.Contains(content, `"diff_unavailable": true`) {
		t.Error("review.json should have diff_unavailable: true")
	}
	if !strings.Contains(content, "spec_alignment") {
		t.Error("review.json should contain spec_alignment findings")
	}
	if !strings.Contains(content, "code_quality") {
		t.Error("review.json should contain code_quality findings")
	}
}
