package stages

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestScenario_DiffAvailable_ReviewProceedsNormally(t *testing.T) {
	// Seed: a 50-line diff that succeeds, a runner that returns clean findings
	fiftyLineDiff := generateDiff(50)

	var capturedInput review.RunInput
	runner := &capturingReviewRunner{
		resultFn: func() *review.RunResult {
			return &review.RunResult{
				AllFindings: []review.Finding{
					{Facet: "code_quality", Severity: review.SeverityInfo, File: "handler.go", Description: "consider extracting helper"},
				},
				BlockingFindings:    []review.Finding{},
				HasBlockingFindings: false,
				FindingsByFacet: map[string][]review.Finding{
					"code_quality": {{Facet: "code_quality", Severity: review.SeverityInfo, File: "handler.go", Description: "consider extracting helper"}},
				},
			}
		},
		capture: func(input review.RunInput) {
			capturedInput = input
		},
	}

	evidenceDir := filepath.Join(t.TempDir(), "evidence")
	eventLogPath := filepath.Join(t.TempDir(), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	stage := NewReviewStage(runner, ReviewStageConfig{
		DiffProvider: &fakeDiffProvider{diff: fiftyLineDiff},
		EvidenceDir:  evidenceDir,
	}, eventLog)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	// Invoke
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Assert: review proceeds normally — Continue action
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue, got %v", action.Kind)
	}
	if !rs.FinalReviewPassed {
		t.Error("expected FinalReviewPassed = true")
	}

	// Assert: diff was passed to the LLM reviewer
	if capturedInput.DiffSummary == "" {
		t.Fatal("expected DiffSummary to be populated")
	}
	if !strings.Contains(capturedInput.DiffSummary, "handler.go") {
		t.Error("expected DiffSummary to contain diff content")
	}

	// Assert: no DiffUnavailableEvent emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	for _, ev := range events {
		if ev.EventType() == "diff_unavailable" {
			t.Error("DiffUnavailableEvent should NOT be emitted when diff succeeds")
		}
	}

	// Assert: review_result event IS emitted (normal flow)
	foundReviewResult := false
	for _, ev := range events {
		if ev.EventType() == "review_result" {
			foundReviewResult = true
		}
	}
	if !foundReviewResult {
		t.Error("expected review_result event to be emitted")
	}

	// Assert: diff_unavailable is false in evidence review.json
	reviewJSONPath := filepath.Join(evidenceDir, "review.json")
	data, err := os.ReadFile(reviewJSONPath)
	if err != nil {
		t.Fatalf("read review.json: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("review.json should be valid JSON: %v", err)
	}

	diffUnavailable, ok := parsed["diff_unavailable"]
	if !ok {
		t.Fatal("diff_unavailable field should be present in review.json")
	}
	if diffUnavailable != false {
		t.Errorf("diff_unavailable should be false, got %v", diffUnavailable)
	}

	// Assert: findings are stored in RunState for evidence
	if len(rs.ReviewFindings) == 0 {
		t.Error("expected ReviewFindings to be populated")
	}
}

// generateDiff creates a synthetic diff with the given number of lines.
func generateDiff(lines int) string {
	var b strings.Builder
	b.WriteString("diff --git a/handler.go b/handler.go\n")
	b.WriteString("--- a/handler.go\n")
	b.WriteString("+++ b/handler.go\n")
	b.WriteString("@@ -1,10 +1,60 @@\n")
	for i := 0; i < lines-4; i++ {
		b.WriteString("+// line ")
		b.WriteString(strings.Repeat("x", 10))
		b.WriteString("\n")
	}
	return b.String()
}
