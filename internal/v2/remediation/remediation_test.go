package remediation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/stage"
	"github.com/danabrams/gromit/internal/v2/stage/finding"
)

func TestRemediationRunnerRun_requiresSpecID(t *testing.T) {
	runner := NewRemediationRunner(RemediationRunnerConfig{})

	if err := runner.Run(context.Background(), "", "", nil); !errors.Is(err, ErrSpecIDRequired) {
		t.Fatalf("expected ErrSpecIDRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresDecomposeStage(t *testing.T) {
	runner := NewRemediationRunner(RemediationRunnerConfig{
		BeadRunner:    &testBeadRunner{},
		GenerationCap: DefaultGenerationCap,
	})

	if err := runner.Run(context.Background(), "spec-id", "", nil); !errors.Is(err, ErrDecomposeStageRequired) {
		t.Fatalf("expected ErrDecomposeStageRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresBeadRunner(t *testing.T) {
	artifacts := &stage.DecomposeArtifacts{Beads: []*bead.Bead{{ID: "bead"}}}
	runner := NewRemediationRunner(RemediationRunnerConfig{
		DecomposeStage: newDecomposeStageReturning(artifacts),
		GenerationCap:  DefaultGenerationCap,
	})

	if err := runner.Run(context.Background(), "spec-id", "", nil); !errors.Is(err, ErrBeadRunnerRequired) {
		t.Fatalf("expected ErrBeadRunnerRequired, got %v", err)
	}
}

func TestRemediationRunnerRun_requiresValidDecomposeArtifacts(t *testing.T) {
	runner := NewRemediationRunner(RemediationRunnerConfig{
		DecomposeStage: newDecomposeStageReturning("unexpected"),
		BeadRunner:     &testBeadRunner{},
		GenerationCap:  DefaultGenerationCap,
	})

	if err := runner.Run(context.Background(), "spec-id", "", nil); !errors.Is(err, ErrUnexpectedDecomposeArtifacts) {
		t.Fatalf("expected ErrUnexpectedDecomposeArtifacts, got %v", err)
	}
}

func TestRemediationRunnerPassesFindingsToDecompose(t *testing.T) {
	var capturedReq *stage.Request
	findings := []stage.SpecFinding{{Title: "missing tests", Description: "tests missing"}}

	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, req *stage.Request) (*stage.StageResult, error) {
			capturedReq = req
			return &stage.StageResult{Artifacts: &stage.DecomposeArtifacts{Beads: []*bead.Bead{{ID: "b"}}}}, nil
		},
	}

	runner := NewRemediationRunner(RemediationRunnerConfig{
		DecomposeStage: decompose,
		BeadRunner:     &testBeadRunner{},
		GenerationCap:  DefaultGenerationCap,
	})

	if err := runner.Run(context.Background(), "spec-ok", "", findings); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if capturedReq == nil {
		t.Fatal("decompose was not invoked")
	}
	if len(capturedReq.Findings) != len(findings) {
		t.Fatalf("findings count = %d, want %d", len(capturedReq.Findings), len(findings))
	}
	for i, want := range findings {
		got := capturedReq.Findings[i]
		if got.Title != want.Title || got.Description != want.Description {
			t.Fatalf("finding[%d] = %+v, want %+v", i, got, want)
		}
	}
}

func TestRemediationRunnerFallsBackToGapAnalysisFileWhenNoFindings(t *testing.T) {
	worktree := t.TempDir()
	gapDir := filepath.Join(worktree, ".gromit", "v2")
	if err := os.MkdirAll(gapDir, 0o755); err != nil {
		t.Fatalf("create gap dir: %v", err)
	}
	gapContent := "failure details"
	gapFile := filepath.Join(gapDir, "gap-analysis.md")
	if err := os.WriteFile(gapFile, []byte(gapContent), 0o644); err != nil {
		t.Fatalf("write gap analysis: %v", err)
	}

	var capturedGap string
	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, req *stage.Request) (*stage.StageResult, error) {
			capturedGap = req.GapAnalysis
			return &stage.StageResult{Artifacts: &stage.DecomposeArtifacts{Beads: []*bead.Bead{{ID: "b"}}}}, nil
		},
	}

	runner := NewRemediationRunner(RemediationRunnerConfig{
		DecomposeStage: decompose,
		BeadRunner:     &testBeadRunner{},
		GenerationCap:  DefaultGenerationCap,
	})

	if err := runner.Run(context.Background(), "spec-gap", worktree, nil); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if capturedGap != gapContent {
		t.Fatalf("gap analysis = %q, want %q", capturedGap, gapContent)
	}
}

type testStage struct {
	name string
	run  func(context.Context, *stage.Request) (*stage.StageResult, error)
}

func (s *testStage) Name() string { return s.name }

func (s *testStage) Run(ctx context.Context, req *stage.Request) (*stage.StageResult, error) {
	if s.run == nil {
		return nil, nil
	}
	return s.run(ctx, req)
}

type testBeadRunner struct{}

func (testBeadRunner) Run(context.Context, []*bead.Bead) error { return nil }

func newDecomposeStageReturning(artifacts any) stage.Stage {
	return &testStage{
		name: "decompose",
		run: func(ctx context.Context, _ *stage.Request) (*stage.StageResult, error) {
			return &stage.StageResult{Artifacts: artifacts}, nil
		},
	}
}
