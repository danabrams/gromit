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

func TestScenario_DiffUnavailable_NoBlockingFindings_Passes(t *testing.T) {
	// Seed: a run where DiffProvider errors but the reviewer finds no blocking issues
	tmp := t.TempDir()
	evidenceDir := filepath.Join(tmp, "evidence")
	eventLogPath := filepath.Join(tmp, "events.jsonl")
	eventLog := runstore.NewEventLog(eventLogPath)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	// The reviewer returns no blocking findings — all criteria met from available evidence
	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings:         []review.Finding{},
			BlockingFindings:    []review.Finding{},
			HasBlockingFindings: false,
			FindingsByFacet:     map[string][]review.Finding{},
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{
		DiffProvider: &fakeDiffProvider{err: errors.New("worktree has no commits yet")},
		EvidenceDir:  evidenceDir,
	}, eventLog)

	// Invoke
	action, err := stage.Run(context.Background(), rs)

	// Assert: no error — graceful degradation
	if err != nil {
		t.Fatalf("expected no error (graceful degradation), got: %v", err)
	}

	// Assert: stage returns Continue (previously this would block)
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue, got %v", action.Kind)
	}

	// Assert: FinalReviewPassed is true
	if !rs.FinalReviewPassed {
		t.Error("expected FinalReviewPassed = true when diff unavailable but no blocking findings")
	}

	// Assert: diff_unavailable event was emitted
	events, err := eventLog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	var foundDiffUnavailable bool
	var foundReviewResult bool
	for _, ev := range events {
		switch ev.EventType() {
		case "diff_unavailable":
			foundDiffUnavailable = true
			due, ok := ev.(*runstore.DiffUnavailableEvent)
			if !ok {
				t.Fatal("expected *DiffUnavailableEvent")
			}
			if !strings.Contains(due.Reason, "worktree has no commits yet") {
				t.Errorf("DiffUnavailableEvent.Reason = %q, want it to contain error message", due.Reason)
			}
		case "review_result":
			foundReviewResult = true
			rre, ok := ev.(*runstore.ReviewResultEvent)
			if !ok {
				t.Fatal("expected *ReviewResultEvent")
			}
			if rre.BlockingFindings != 0 {
				t.Errorf("expected 0 blocking findings in event, got %d", rre.BlockingFindings)
			}
		}
	}
	if !foundDiffUnavailable {
		t.Error("expected diff_unavailable event in log")
	}
	if !foundReviewResult {
		t.Error("expected review_result event in log")
	}

	// Assert: review.json records diff_unavailable = true
	reviewJSONPath := filepath.Join(evidenceDir, "review.json")
	data, err := os.ReadFile(reviewJSONPath)
	if err != nil {
		t.Fatalf("read review.json: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse review.json: %v", err)
	}
	diffUnavail, ok := parsed["diff_unavailable"]
	if !ok {
		t.Fatal("review.json should contain diff_unavailable field")
	}
	if diffUnavail != true {
		t.Errorf("diff_unavailable = %v, want true", diffUnavail)
	}
}

func TestScenario_DiffUnavailable_WithInfoFindings_StillPasses(t *testing.T) {
	// Seed: diff errors, reviewer returns info-only findings (non-blocking)
	tmp := t.TempDir()
	evidenceDir := filepath.Join(tmp, "evidence")
	eventLog := runstore.NewEventLog(filepath.Join(tmp, "events.jsonl"))

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings: []review.Finding{
				{Facet: "code_quality", Severity: review.SeverityInfo, File: "handler.go", Description: "consider extracting helper"},
			},
			BlockingFindings:    []review.Finding{},
			HasBlockingFindings: false,
			FindingsByFacet: map[string][]review.Finding{
				"code_quality": {
					{Facet: "code_quality", Severity: review.SeverityInfo, File: "handler.go", Description: "consider extracting helper"},
				},
			},
		},
	}

	stage := NewReviewStage(runner, ReviewStageConfig{
		DiffProvider: &fakeDiffProvider{err: errors.New("no merge base found")},
		EvidenceDir:  evidenceDir,
	}, eventLog)

	// Invoke
	action, err := stage.Run(context.Background(), rs)

	// Assert: passes despite diff unavailable + info findings
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Errorf("expected Continue with info-only findings and diff unavailable, got %v", action.Kind)
	}
	if !rs.FinalReviewPassed {
		t.Error("expected FinalReviewPassed = true")
	}

	// Assert: info findings still recorded in RunState for evidence
	if len(rs.ReviewFindings) == 0 {
		t.Error("expected info findings to be recorded in ReviewFindings for evidence")
	}

	// Assert: review.json marks diff as unavailable
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
	// Info findings should still appear in evidence
	if _, ok := parsed["code_quality"]; !ok {
		t.Error("expected code_quality findings in review.json despite diff unavailable")
	}
}
