//go:build acceptance

package acceptance_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner"
)

// TestSpecPrReviewGateLifecycle exercises the spec PR gate wiring by faking the
// SpecMergeController dependency. It verifies that the orchestrator detects spec
// completion, triggers the merge pipeline once, and continues processing other
// beads while the PR is marked ready for human review.
func TestSpecPrReviewGateLifecycle(t *testing.T) {
	t.Parallel()

	specName := "auth"
	beads := []*bead.Bead{
		{ID: "spec-auth-1", Labels: []string{"spec:" + specName}},
		{ID: "spec-auth-2", Labels: []string{"spec:" + specName}},
		{ID: "storybook-1", Labels: []string{}},
	}

	closed := make(map[string]struct{})
	specBeads := map[string]struct{}{
		"spec-auth-1": {},
		"spec-auth-2": {},
	}
	closedSpecCount := 0

	mockBeads := &mockBeadClient{
		ReadyFn: func(ctx context.Context) (*bead.Bead, error) {
			for _, candidate := range beads {
				if _, ok := closed[candidate.ID]; ok {
					continue
				}
				return candidate, nil
			}
			return nil, nil
		},
		CloseFn: func(ctx context.Context, id string) error {
			if _, ok := closed[id]; ok {
				return nil
			}
			closed[id] = struct{}{}
			if _, ok := specBeads[id]; ok {
				closedSpecCount++
			}
			return nil
		},
	}

	executed := make([]string, 0, len(beads))
	buildStage := &testStage{
		fn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			executed = append(executed, in.Bead.ID)
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		},
	}

	specController := &fakeSpecMergeController{
		isCompleteFn: func(_ string) (bool, error) {
			return closedSpecCount == len(specBeads), nil
		},
	}

	cfg := &config.Config{
		Methodology: config.MethodologyConfig{
			Granularity: config.MethodologyGranularitySpec,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	orchOutput := &bytes.Buffer{}
	orch := runner.NewOrchestrator(runner.OrchestratorConfig{
		Gate:     &testStage{},
		Build:    buildStage,
		Validate: &testStage{},
		Epilogue: &testEpilogueStage{fn: func(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
			_ = mockBeads.Close(ctx, in.Bead.ID)
			return pipeline.Output{Decision: pipeline.Proceed}, nil
		}},
		GetBead:             func(ctx context.Context) (*bead.Bead, error) { return mockBeads.Ready(ctx) },
		Config:              cfg,
		SpecMergeController: specController,
		Output:              orchOutput,
	})

	if err := orch.Run(context.Background(), len(beads)*2, time.Time{}, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(executed) != len(beads) {
		t.Fatalf("expected %d beads to execute, got %d", len(beads), len(executed))
	}

	if len(specController.triggerCalls) != 1 {
		t.Fatalf("spec merge pipeline triggered %d times, want 1", len(specController.triggerCalls))
	}
	if got := specController.triggerCalls[0]; got != specName {
		t.Fatalf("trigger called for %q, want %q", got, specName)
	}

	readyMessage := fmt.Sprintf("Spec %q ready for human review", specName)
	if !strings.Contains(orchOutput.String(), readyMessage) {
		t.Fatalf("expected orchestrator output to mention %q", readyMessage)
	}

	if executed[len(executed)-1] != beads[len(beads)-1].ID {
		t.Fatalf("expected non-spec bead %q to run last, got %q", beads[len(beads)-1].ID, executed[len(executed)-1])
	}

	if len(specController.isCompleteCalls) < len(specBeads) {
		t.Fatalf("IsSpecComplete called %d times, want at least %d", len(specController.isCompleteCalls), len(specBeads))
	}
	for _, call := range specController.isCompleteCalls {
		if call != specName {
			t.Fatalf("IsSpecComplete called with %q, want %q", call, specName)
		}
	}
}

// fakeSpecMergeController is a test double that records spec completion and trigger calls.
type fakeSpecMergeController struct {
	isCompleteFn func(string) (bool, error)
	triggerFn    func(context.Context, string) error

	isCompleteCalls []string
	triggerCalls    []string
}

func (f *fakeSpecMergeController) IsSpecComplete(specName string) (bool, error) {
	f.isCompleteCalls = append(f.isCompleteCalls, specName)
	if f.isCompleteFn != nil {
		return f.isCompleteFn(specName)
	}
	return false, nil
}

func (f *fakeSpecMergeController) Trigger(ctx context.Context, specName string) error {
	f.triggerCalls = append(f.triggerCalls, specName)
	if f.triggerFn != nil {
		return f.triggerFn(ctx, specName)
	}
	return nil
}
