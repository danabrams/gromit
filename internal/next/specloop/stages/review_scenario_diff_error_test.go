package stages

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

func TestScenario_DiffProviderError_ReviewProceedsWithoutDiff(t *testing.T) {
	// Seed: set up event log, evidence dir, and a RunState at cycle 1
	tmp := t.TempDir()
	eventLogPath := filepath.Join(tmp, "events.jsonl")
	evidenceDir := filepath.Join(tmp, "evidence")
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

	diffErr := errors.New("fatal: not a git repository")
	stage := NewReviewStage(runner, ReviewStageConfig{
		DiffProvider: &fakeDiffProvider{err: diffErr},
		EvidenceDir:  evidenceDir,
		SpecContent:  "spec content here",
	}, eventLog)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	// Invoke
	action, err := stage.Run(context.Background(), rs)

	// Assert: Run() does not return an error
	if err != nil {
		t.Fatalf("expected no error from Run(), got: %v", err)
	}

	// Assert: action is Continue (no blocker from diff error)
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue, got %v", action.Kind)
	}

	// Assert: DiffSummary sent to reviewer contains placeholder with error
	if !strings.Contains(capturedInput.DiffSummary, "[diff unavailable:") {
		t.Errorf("expected DiffSummary to contain '[diff unavailable:', got: %q", capturedInput.DiffSummary)
	}
	if !strings.Contains(capturedInput.DiffSummary, "fatal: not a git repository") {
		t.Errorf("expected DiffSummary to contain error message, got: %q", capturedInput.DiffSummary)
	}

	// Assert: DiffUnavailableEvent emitted in event log
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	var diffUnavailableFound bool
	for _, ev := range events {
		if ev.EventType() == "diff_unavailable" {
			due, ok := ev.(*runstore.DiffUnavailableEvent)
			if !ok {
				t.Fatalf("expected *DiffUnavailableEvent, got %T", ev)
			}
			if !strings.Contains(due.Reason, "fatal: not a git repository") {
				t.Errorf("DiffUnavailableEvent.Reason = %q, want it to contain error text", due.Reason)
			}
			diffUnavailableFound = true
		}
	}
	if !diffUnavailableFound {
		t.Error("expected DiffUnavailableEvent in event log, not found")
	}

	// Assert: review_result event also emitted (review proceeded)
	var reviewResultFound bool
	for _, ev := range events {
		if ev.EventType() == "review_result" {
			reviewResultFound = true
		}
	}
	if !reviewResultFound {
		t.Error("expected review_result event in event log (review should proceed despite diff error)")
	}

	// Assert: diff_unavailable is true in evidence/review.json
	reviewJSONPath := filepath.Join(evidenceDir, "review.json")
	data, err := os.ReadFile(reviewJSONPath)
	if err != nil {
		t.Fatalf("read review.json: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse review.json: %v", err)
	}
	diffUnavailableVal, ok := parsed["diff_unavailable"]
	if !ok {
		t.Fatal("diff_unavailable field missing from review.json")
	}
	if diffUnavailableVal != true {
		t.Errorf("diff_unavailable = %v, want true", diffUnavailableVal)
	}

	// Assert: FinalReviewPassed is true (no blocking findings)
	if !rs.FinalReviewPassed {
		t.Error("expected FinalReviewPassed = true when no blocking findings")
	}
}

func TestScenario_DiffProviderError_ExitStatus128_ReviewProceedsWithoutDiff(t *testing.T) {
	// Seed: same shape but with "exit status 128" error text
	tmp := t.TempDir()
	eventLogPath := filepath.Join(tmp, "events.jsonl")
	evidenceDir := filepath.Join(tmp, "evidence")
	eventLog := runstore.NewEventLog(eventLogPath)

	var capturedInput review.RunInput
	runner := &capturingReviewRunner{
		resultFn: func() *review.RunResult {
			return &review.RunResult{
				AllFindings: []review.Finding{
					{Facet: "spec_alignment", Severity: review.SeverityInfo, File: "main.go", Description: "acceptable"},
				},
				BlockingFindings:    []review.Finding{},
				HasBlockingFindings: false,
				FindingsByFacet: map[string][]review.Finding{
					"spec_alignment": {{Facet: "spec_alignment", Severity: review.SeverityInfo, File: "main.go", Description: "acceptable"}},
				},
			}
		},
		capture: func(input review.RunInput) {
			capturedInput = input
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{
		DiffProvider: &fakeDiffProvider{err: errors.New("exit status 128")},
		EvidenceDir:  evidenceDir,
	}, eventLog)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	// Invoke
	action, err := stage.Run(context.Background(), rs)

	// Assert: no error, continues
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue, got %v", action.Kind)
	}

	// Assert: placeholder sent to reviewer
	if !strings.Contains(capturedInput.DiffSummary, "exit status 128") {
		t.Errorf("DiffSummary should contain error text, got: %q", capturedInput.DiffSummary)
	}

	// Assert: reviewer still received task/acceptance data and produced findings
	if len(rs.ReviewFindings) == 0 {
		t.Error("expected ReviewFindings to be populated (reviewer evaluated acceptance criteria)")
	}

	// Assert: evidence marks diff_unavailable = true
	data, err := os.ReadFile(filepath.Join(evidenceDir, "review.json"))
	if err != nil {
		t.Fatalf("read review.json: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse review.json: %v", err)
	}
	if parsed["diff_unavailable"] != true {
		t.Errorf("diff_unavailable = %v, want true", parsed["diff_unavailable"])
	}

	// Assert: findings still present alongside diff_unavailable
	if _, ok := parsed["spec_alignment"]; !ok {
		t.Error("expected spec_alignment findings in review.json despite diff unavailability")
	}
}
