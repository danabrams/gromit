package stages

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/provider"
)

// neverCalledInvoker panics if Invoke is called — asserts that the LLM is never
// reached when the file-read check fails first and returns fail-safe.
type neverCalledInvoker struct{ t *testing.T }

func (n *neverCalledInvoker) Invoke(_ context.Context, _ string) (*provider.Result, error) {
	n.t.Fatal("invoker must not be called when file is unreadable")
	return nil, nil
}

func (n *neverCalledInvoker) InvokeInDir(_ context.Context, _ string, _ string) (*provider.Result, error) {
	n.t.Fatal("invoker must not be called when file is unreadable")
	return nil, nil
}

// TestScenario_ReviewStage_FileUnreadable_FailSafeRetainsFinding verifies that when
// LLMFindingVerifier cannot read the file referenced by a blocking finding, it returns
// DispositionConfirmed ("file unreadable") so the finding is retained and the stage
// triggers a replan — identical behaviour to having no verifier at all.
func TestScenario_ReviewStage_FileUnreadable_FailSafeRetainsFinding(t *testing.T) {
	// Seed: one blocking error finding whose File does not exist on disk
	// and is absent from the diff.
	eventLog := runstore.NewEventLog(t.TempDir() + "/events.jsonl")
	workDir := t.TempDir() // empty: cmd/gromit-next/nonexistent.go is not here

	blockingFinding := review.Finding{
		Facet:       "test_facet",
		Severity:    review.SeverityError,
		File:        "cmd/gromit-next/nonexistent.go",
		Line:        10,
		Description: "blocking error in nonexistent file",
		Cycle:       1,
	}

	runner := &mockReviewRunner{
		result: &review.RunResult{
			AllFindings:         []review.Finding{blockingFinding},
			BlockingFindings:    []review.Finding{blockingFinding},
			HasBlockingFindings: true,
			FindingsByFacet: map[string][]review.Finding{
				"test_facet": {blockingFinding},
			},
		},
	}

	// LLMFindingVerifier backed by neverCalledInvoker: the LLM must not be
	// reached because the unreadable-file path short-circuits before invocation.
	verifier := review.NewLLMFindingVerifier(&neverCalledInvoker{t: t})

	stage := NewReviewStage(runner, ReviewStageConfig{
		Verifier:     verifier,
		DiffProvider: &fakeDiffProvider{diff: ""}, // empty diff → finding is out-of-diff
		WorkDir:      workDir,
	}, eventLog)

	rs := runstore.NewRunState("test-spec", "test-project")
	rs.Cycle = 1

	// Invoke
	action, err := stage.Run(context.Background(), rs)

	// Assert: no error
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Assert: stage triggers ReplanFrom — finding retained (fail-safe confirmed)
	if action.Kind != specloop.ReplanFrom {
		t.Errorf("expected ReplanFrom, got %v", action.Kind)
	}

	// Assert: blocking finding is present in the action context with original severity
	if action.Context == nil {
		t.Fatal("expected non-nil action context")
	}
	if len(action.Context.Failures) == 0 {
		t.Error("expected blocking finding to be retained in action context failures")
	}
}
