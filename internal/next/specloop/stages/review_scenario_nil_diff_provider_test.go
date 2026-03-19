package stages

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestScenario_NilDiffProvider_ReviewProceedsWithEmptyDiff(t *testing.T) {
	// Seed: configure ReviewStage with nil DiffProvider, an event log, and an evidence dir.
	evidenceDir := filepath.Join(t.TempDir(), "evidence")
	eventLogPath := filepath.Join(t.TempDir(), "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	var capturedInput review.RunInput
	runner := &capturingReviewRunner{
		resultFn: func() *review.RunResult {
			return &review.RunResult{
				AllFindings:         []review.Finding{},
				BlockingFindings:    []review.Finding{},
				HasBlockingFindings: false,
				FindingsByFacet:     map[string][]review.Finding{},
			}
		},
		capture: func(input review.RunInput) {
			capturedInput = input
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{
		DiffProvider: nil, // explicitly nil
		EvidenceDir:  evidenceDir,
	}, eventLog)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	// Invoke
	action, err := stage.Run(context.Background(), rs)

	// Assert: no error, returns Continue
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue, got %v", action.Kind)
	}

	// Assert: empty diff passed to runner
	if capturedInput.DiffSummary != "" {
		t.Errorf("expected empty DiffSummary, got %q", capturedInput.DiffSummary)
	}

	// Assert: no DiffUnavailableEvent emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	for _, ev := range events {
		if ev.EventType() == "diff_unavailable" {
			t.Error("expected no DiffUnavailableEvent, but one was emitted")
		}
	}

	// Assert: review.json has diff_unavailable = false
	reviewJSONPath := filepath.Join(evidenceDir, "review.json")
	data, err := os.ReadFile(reviewJSONPath)
	if err != nil {
		t.Fatalf("read review.json: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse review.json: %v", err)
	}
	diffUnavailable, ok := parsed["diff_unavailable"]
	if !ok {
		t.Fatal("diff_unavailable field should be present in review.json")
	}
	if diffUnavailable != false {
		t.Errorf("diff_unavailable should be false, got %v", diffUnavailable)
	}

	// Assert: FinalReviewPassed is set
	if !rs.FinalReviewPassed {
		t.Error("expected FinalReviewPassed = true")
	}
}
