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
	runner := newRunnerForRemediationCycle(newDecomposeStageReturning(&stage.DecomposeArtifacts{Beads: []*bead.Bead{{ID: "b"}}}), &testBeadRunner{}, 1)

	if err := runner.Run(context.Background(), "", "", nil); !errors.Is(err, ErrSpecIDRequired) {
		t.Fatalf("expected ErrSpecIDRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresBeadRunner(t *testing.T) {
	artifacts := &stage.DecomposeArtifacts{Beads: []*bead.Bead{{ID: "b"}}}
	runner := newRunnerForRemediationCycle(newDecomposeStageReturning(artifacts), nil, 1)

	if err := runner.Run(context.Background(), "spec-id", "", nil); !errors.Is(err, ErrBeadRunnerRequired) {
		t.Fatalf("expected ErrBeadRunnerRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresDecomposeStage(t *testing.T) {
	runner := NewRemediationRunner(RemediationRunnerConfig{
		BeadRunner:    &testBeadRunner{},
		GenerationCap: 1,
	})

	if err := runner.Run(context.Background(), "spec-id", "", nil); !errors.Is(err, ErrDecomposeStageRequired) {
		t.Fatalf("expected ErrDecomposeStageRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresValidDecomposeArtifacts(t *testing.T) {
	runner := newRunnerForRemediationCycle(newDecomposeStageReturning("invalid"), &testBeadRunner{}, 1)

	if err := runner.Run(context.Background(), "spec-id", "", nil); !errors.Is(err, ErrUnexpectedDecomposeArtifacts) {
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
			return &stage.StageResult{Artifacts: &stage.DecomposeArtifacts{Beads: []*bead.Bead{{ID: "fix-bead"}}}}, nil
		},
	}

	beadRunner := &recordingBeadRunner{expectedBeadID: "fix-bead"}
	runner := newRunnerForRemediationCycle(decompose, beadRunner, 1)

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

func TestRemediationRunnerRun_worktreePopulatedInRequest(t *testing.T) {
	worktree := "/tmp/test-worktree"
	var capturedWorktree string
	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, req *stage.Request) (*stage.StageResult, error) {
			capturedWorktree = req.Worktree
			return &stage.StageResult{Artifacts: &stage.DecomposeArtifacts{Beads: []*bead.Bead{{ID: "w"}}}}, nil
		},
	}

	runner := newRunnerForRemediationCycle(decompose, &testBeadRunner{}, 1)
	if err := runner.Run(context.Background(), "spec-id", worktree, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedWorktree != worktree {
		t.Fatalf("request worktree = %q, want %q", capturedWorktree, worktree)
	}
}

func newRunnerForRemediationCycle(decompose stage.Stage, beadRunner BeadRunner, generationCap int) *RemediationRunner {
	return NewRemediationRunner(RemediationRunnerConfig{
		DecomposeStage: decompose,
		BeadRunner:     beadRunner,
		GenerationCap:  generationCap,
	})
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
