package remediation

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/v2/stage"
)

func TestRemediationRunnerRun_requiresSpecID(t *testing.T) {
	runner := newRunnerForSpecValidation()

	if err := runner.Run(context.Background(), "", ""); !errors.Is(err, ErrSpecIDRequired) {
		t.Fatalf("expected ErrSpecIDRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresAcceptStage(t *testing.T) {
	runner := newRunnerWithAcceptStage(nil)

	if err := runner.Run(context.Background(), "spec-id", ""); !errors.Is(err, ErrAcceptStageRequired) {
		t.Fatalf("expected ErrAcceptStageRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresBeadRunner(t *testing.T) {
	artifacts := &stage.DecomposeArtifacts{Beads: []*bead.Bead{}}
	runner := newRunnerForRemediationCycle(newDecisionFailStage(), newDecomposeStageReturning(artifacts), nil, 1)

	if err := runner.Run(context.Background(), "spec-id", ""); !errors.Is(err, ErrBeadRunnerRequired) {
		t.Fatalf("expected ErrBeadRunnerRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresDecomposeStage(t *testing.T) {
	runner := newRunnerForDecomposeFailure(newDecisionFailStage(), 1)

	if err := runner.Run(context.Background(), "spec-id", ""); !errors.Is(err, ErrDecomposeStageRequired) {
		t.Fatalf("expected ErrDecomposeStageRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresValidDecomposeArtifacts(t *testing.T) {
	runner := newRunnerForUnexpectedArtifacts(newDecisionFailStage(), newDecomposeStageReturning("unexpected"), 1)

	if err := runner.Run(context.Background(), "spec-id", ""); !errors.Is(err, ErrUnexpectedDecomposeArtifacts) {
		t.Fatalf("expected ErrUnexpectedDecomposeArtifacts, got %v", err)
	}
}

func TestRemediationRunnerRunWithFindingsCallsDecomposeAndBeadRunner(t *testing.T) {
	findings := []stage.Finding{{Description: "Fix flake", Severity: stage.FindingSeverityWarning}}
	var capturedReq *stage.Request
	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, req *stage.Request) (*stage.StageResult, error) {
			capturedReq = req
			return &stage.StageResult{
				Artifacts: &stage.DecomposeArtifacts{Beads: []*bead.Bead{{ID: "fix-bead"}}},
			}, nil
		},
	}

	beadRunner := &recordingBeadRunner{expectedBeadID: "fix-bead"}
	runner := newRunnerForRemediationCycle(nil, decompose, beadRunner, 1)

	if err := runner.Run(context.Background(), "spec-id", "/worktree", findings); err != nil {
		t.Fatalf("runner.Run failed: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("decompose stage was not called")
	}
	if !capturedReq.Remediation {
		t.Fatal("expected remediation flag to be set")
	}
	if len(capturedReq.Findings) != len(findings) || capturedReq.Findings[0].Description != "Fix flake" {
		t.Fatalf("Findings = %v, want %v", capturedReq.Findings, findings)
	}
	if beadRunner.callCount != 1 {
		t.Fatalf("bead runner called %d times, want 1", beadRunner.callCount)
	}
}

func TestRemediationRunnerUsesDefaultGenerationCapWhenNegative(t *testing.T) {
	ctx := context.Background()

	failuresRemaining := 3
	accept := &testStage{
		name: "accept",
		run: func(ctx context.Context, _ *stage.Request) (*stage.StageResult, error) {
			if failuresRemaining > 0 {
				failuresRemaining--
				return &stage.StageResult{Decision: stage.DecisionFail}, nil
			}
			return &stage.StageResult{Decision: stage.DecisionProceed}, nil
		},
	}

	decomposeCalls := 0
	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, _ *stage.Request) (*stage.StageResult, error) {
			decomposeCalls++
			return &stage.StageResult{
				Artifacts: &stage.DecomposeArtifacts{
					Beads: []*bead.Bead{{ID: "generated"}},
				},
			}, nil
		},
	}

	runner := newRunnerForRemediationCycle(accept, decompose, &testBeadRunner{}, -1)
	if err := runner.Run(ctx, "spec-id", ""); err != nil {
		t.Fatalf("remediation run failed: %v", err)
	}

	if decomposeCalls != 3 {
		t.Fatalf("decompose calls = %d, want 3", decomposeCalls)
	}
}

func TestRemediationRunnerRun_resetsGenerationCountBetweenRuns(t *testing.T) {
	ctx := context.Background()

	// Accept stage: fail once then succeed (per run).
	callCount := 0
	accept := &testStage{
		name: "accept",
		run: func(ctx context.Context, _ *stage.Request) (*stage.StageResult, error) {
			callCount++
			// Odd calls fail, even calls succeed (fail-then-pass per run).
			if callCount%2 == 1 {
				return &stage.StageResult{Decision: stage.DecisionFail}, nil
			}
			return &stage.StageResult{Decision: stage.DecisionProceed}, nil
		},
	}

	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, _ *stage.Request) (*stage.StageResult, error) {
			return &stage.StageResult{
				Artifacts: &stage.DecomposeArtifacts{
					Beads: []*bead.Bead{{ID: "b1"}},
				},
			}, nil
		},
	}

	// GenerationCap=1: only one remediation allowed per run.
	runner := newRunnerForRemediationCycle(accept, decompose, &testBeadRunner{}, 1)

	// First run: should succeed (one remediation, then accept passes).
	if err := runner.Run(ctx, "spec-1", ""); err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	// Second run: if generationCount is not reset, the runner thinks it already
	// hit the cap and returns "generation cap reached" immediately.
	if err := runner.Run(ctx, "spec-2", ""); err != nil {
		t.Fatalf("second run should succeed but got: %v", err)
	}
}

func TestRemediationRunnerRun_contextCancelledDuringLoop(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	acceptCalls := 0
	accept := &testStage{
		name: "accept",
		run: func(ctx context.Context, _ *stage.Request) (*stage.StageResult, error) {
			acceptCalls++
			// Always return failure so the loop would run forever without
			// context cancellation.
			if acceptCalls >= 2 {
				// Cancel the context on the second accept call to simulate
				// external cancellation while the loop is running.
				cancel()
				return nil, ctx.Err()
			}
			return &stage.StageResult{Decision: stage.DecisionFail}, nil
		},
	}

	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, _ *stage.Request) (*stage.StageResult, error) {
			return &stage.StageResult{
				Artifacts: &stage.DecomposeArtifacts{
					Beads: []*bead.Bead{{ID: "remediation-bead"}},
				},
			}, nil
		},
	}

	runner := newRunnerForRemediationCycle(accept, decompose, &testBeadRunner{}, 10)

	err := runner.Run(ctx, "spec-cancel", "")
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	// The loop should have terminated after the context was cancelled,
	// not after exhausting the generation cap.
	if acceptCalls > 2 {
		t.Fatalf("accept calls = %d, expected at most 2 (loop should stop on context cancellation)", acceptCalls)
	}
}

func TestRemediationRunnerRun_worktreePopulatedInRequest(t *testing.T) {
	worktree := "/tmp/test-worktree"
	var capturedWorktree string

	accept := &testStage{
		name: "accept",
		run: func(ctx context.Context, req *stage.Request) (*stage.StageResult, error) {
			capturedWorktree = req.Worktree
			return &stage.StageResult{Decision: stage.DecisionProceed}, nil
		},
	}

	runner := NewRemediationRunner(RemediationRunnerConfig{AcceptStage: accept})
	if err := runner.Run(context.Background(), "spec-id", worktree); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedWorktree != worktree {
		t.Fatalf("request worktree = %q, want %q", capturedWorktree, worktree)
	}
}

func TestRemediationRunnerPassesGapAnalysisToDecompose(t *testing.T) {
	t.Parallel()

	gapText := "Criterion 1 failed: commits not produced"

	acceptCalls := 0
	accept := &testStage{
		name: "accept",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			acceptCalls++
			if acceptCalls == 1 {
				return &stage.Result{
					Decision:  stage.DecisionFail,
					Artifacts: &gapArtifacts{gap: gapText},
				}, nil
			}
			return &stage.Result{Decision: stage.DecisionProceed}, nil
		},
	}

	var capturedGap string
	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			capturedGap = req.GapAnalysis
			return &stage.Result{
				Artifacts: &stage.DecomposeArtifacts{
					Beads: []*bead.Bead{{ID: "gap-bead"}},
				},
			}, nil
		},
	}

	runner := newRunnerForRemediationCycle(accept, decompose, &testBeadRunner{}, 1)
	if err := runner.Run(context.Background(), "spec-gap", ""); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if capturedGap != gapText {
		t.Fatalf("gap analysis = %q, want %q", capturedGap, gapText)
	}
}

func TestRemediationRunnerGapAnalysisFlowsToDecompose(t *testing.T) {
	t.Parallel()

	gapText := "Criterion 1 failed: no stage commits\nCriterion 3 failed: no event log"

	acceptCalls := 0
	accept := &testStage{
		name: "accept",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			acceptCalls++
			if acceptCalls == 1 {
				return &stage.Result{
					Decision:  stage.DecisionFail,
					Artifacts: &gapArtifacts{gap: gapText},
				}, nil
			}
			return &stage.Result{Decision: stage.DecisionProceed}, nil
		},
	}

	var capturedReq *stage.Request
	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			capturedReq = req
			return &stage.Result{
				Artifacts: &stage.DecomposeArtifacts{
					Beads: []*bead.Bead{{ID: "targeted-bead"}},
				},
			}, nil
		},
	}

	runner := newRunnerForRemediationCycle(accept, decompose, &testBeadRunner{}, 1)
	if err := runner.Run(context.Background(), "spec-gap-flow", ""); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("decompose was not called")
	}
	if capturedReq.GapAnalysis != gapText {
		t.Fatalf("GapAnalysis = %q, want %q", capturedReq.GapAnalysis, gapText)
	}
	if !capturedReq.Remediation {
		t.Fatal("Remediation flag not set")
	}
}

// gapArtifacts is a test helper implementing the gapSummaryProvider interface.
type gapArtifacts struct {
	gap string
}

func (g *gapArtifacts) GetGapSummary() string {
	return g.gap
}

func newRunnerForSpecValidation() *RemediationRunner {
	return NewRemediationRunner(RemediationRunnerConfig{})
}

func newRunnerWithAcceptStage(stage stage.Stage) *RemediationRunner {
	return NewRemediationRunner(RemediationRunnerConfig{AcceptStage: stage})
}

func newRunnerForRemediationCycle(accept stage.Stage, decompose stage.Stage, beadRunner BeadRunner, generationCap int) *RemediationRunner {
	return NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage:    accept,
		DecomposeStage: decompose,
		BeadRunner:     beadRunner,
		GenerationCap:  generationCap,
	})
}

func newRunnerForDecomposeFailure(accept stage.Stage, generationCap int) *RemediationRunner {
	return newRunnerForRemediationCycle(accept, nil, &testBeadRunner{}, generationCap)
}

func newRunnerForUnexpectedArtifacts(accept stage.Stage, decompose stage.Stage, generationCap int) *RemediationRunner {
	return newRunnerForRemediationCycle(accept, decompose, &testBeadRunner{}, generationCap)
}

func newDecisionFailStage() stage.Stage {
	return &testStage{
		name: "decision-fail",
		run: func(ctx context.Context, _ *stage.Request) (*stage.StageResult, error) {
			return &stage.StageResult{Decision: stage.DecisionFail}, nil
		},
	}
}

func newDecomposeStageReturning(artifacts any) stage.Stage {
	return &testStage{
		name: "decompose",
		run: func(ctx context.Context, _ *stage.Request) (*stage.StageResult, error) {
			return &stage.StageResult{Artifacts: artifacts}, nil
		},
	}
}

type testStage struct {
	name string
	run  func(context.Context, *stage.Request) (*stage.StageResult, error)
}

func (s *testStage) Name() string {
	return s.name
}

func (s *testStage) Run(ctx context.Context, req *stage.Request) (*stage.StageResult, error) {
	if s.run == nil {
		return nil, nil
	}
	return s.run(ctx, req)
}

type testBeadRunner struct{}

func (testBeadRunner) Run(context.Context, []*bead.Bead) error {
	return nil
}

type recordingBeadRunner struct {
	callCount      int
	expectedBeadID string
}

func (r *recordingBeadRunner) Run(ctx context.Context, beads []*bead.Bead) error {
	r.callCount++
	if len(beads) != 1 {
		return fmt.Errorf("expected 1 bead, got %d", len(beads))
	}
	if r.expectedBeadID != "" && beads[0].ID != r.expectedBeadID {
		return fmt.Errorf("unexpected bead id %q", beads[0].ID)
	}
	return nil
}
