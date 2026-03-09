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

func TestRemediation_CreatesRemediationPlanNotOriginal(t *testing.T) {
	t.Parallel()

	gapText := "Criterion 1 failed: missing tests"

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

	planCalled := false
	var planRemediation bool
	var planGapAnalysis string
	planStage := &testStage{
		name: "plan",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			planCalled = true
			planRemediation = req.Remediation
			planGapAnalysis = req.GapAnalysis
			return &stage.Result{}, nil
		},
	}

	decomposeCalled := false
	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			decomposeCalled = true
			return &stage.Result{
				Artifacts: &stage.DecomposeArtifacts{
					Beads: []*bead.Bead{{ID: "remediation-bead"}},
				},
			}, nil
		},
	}

	runner := NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage:    accept,
		PlanStage:      planStage,
		DecomposeStage: decompose,
		BeadRunner:     &testBeadRunner{},
		GenerationCap:  1,
	})

	if err := runner.Run(context.Background(), "spec-plan", ""); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if !planCalled {
		t.Fatal("plan stage was not called")
	}
	if !planRemediation {
		t.Fatal("plan stage did not receive Remediation=true")
	}
	if planGapAnalysis == "" {
		t.Fatal("plan stage received empty GapAnalysis")
	}
	if planGapAnalysis != gapText {
		t.Fatalf("plan GapAnalysis = %q, want %q", planGapAnalysis, gapText)
	}
	if !decomposeCalled {
		t.Fatal("decompose stage was not called")
	}
}

func TestRemediation_RemediationPlanPersistedSeparately(t *testing.T) {
	t.Parallel()

	worktree := t.TempDir()
	gromitDir := filepath.Join(worktree, ".gromit", "v2")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("create gromit dir: %v", err)
	}
	originalPlan := "# Original plan"
	if err := os.WriteFile(filepath.Join(gromitDir, "plan.md"), []byte(originalPlan), 0o644); err != nil {
		t.Fatalf("write plan.md: %v", err)
	}

	gapText := "Criterion 2 failed: missing integration"

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

	planStage := &testStage{
		name: "plan",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			return &stage.Result{}, nil
		},
	}

	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			return &stage.Result{
				Artifacts: &stage.DecomposeArtifacts{
					Beads: []*bead.Bead{{ID: "b1"}},
				},
			}, nil
		},
	}

	runner := NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage:    accept,
		PlanStage:      planStage,
		DecomposeStage: decompose,
		BeadRunner:     &testBeadRunner{},
		GenerationCap:  1,
	})

	if err := runner.Run(context.Background(), "spec-persist", worktree); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Original plan.md should be unchanged.
	got, err := os.ReadFile(filepath.Join(gromitDir, "plan.md"))
	if err != nil {
		t.Fatalf("read plan.md: %v", err)
	}
	if string(got) != originalPlan {
		t.Fatalf("plan.md = %q, want %q", string(got), originalPlan)
	}

	// remediation-1.md should exist with the gap analysis.
	remPath := filepath.Join(gromitDir, "remediation-1.md")
	remContent, err := os.ReadFile(remPath)
	if err != nil {
		t.Fatalf("remediation-1.md not found: %v", err)
	}
	if string(remContent) != gapText {
		t.Fatalf("remediation-1.md = %q, want %q", string(remContent), gapText)
	}
}

func TestRemediation_SecondRemediationCreatesRemediationPlan2(t *testing.T) {
	t.Parallel()

	worktree := t.TempDir()
	gromitDir := filepath.Join(worktree, ".gromit", "v2")
	if err := os.MkdirAll(gromitDir, 0o755); err != nil {
		t.Fatalf("create gromit dir: %v", err)
	}

	gap1 := "Gap round 1"
	gap2 := "Gap round 2"

	acceptCalls := 0
	accept := &testStage{
		name: "accept",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			acceptCalls++
			switch acceptCalls {
			case 1:
				return &stage.Result{
					Decision:  stage.DecisionFail,
					Artifacts: &gapArtifacts{gap: gap1},
				}, nil
			case 2:
				return &stage.Result{
					Decision:  stage.DecisionFail,
					Artifacts: &gapArtifacts{gap: gap2},
				}, nil
			default:
				return &stage.Result{Decision: stage.DecisionProceed}, nil
			}
		},
	}

	planStage := &testStage{
		name: "plan",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			return &stage.Result{}, nil
		},
	}

	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			return &stage.Result{
				Artifacts: &stage.DecomposeArtifacts{
					Beads: []*bead.Bead{{ID: "b"}},
				},
			}, nil
		},
	}

	runner := NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage:    accept,
		PlanStage:      planStage,
		DecomposeStage: decompose,
		BeadRunner:     &testBeadRunner{},
		GenerationCap:  3,
	})

	if err := runner.Run(context.Background(), "spec-multi", worktree); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	rem1, err := os.ReadFile(filepath.Join(gromitDir, "remediation-1.md"))
	if err != nil {
		t.Fatalf("remediation-1.md not found: %v", err)
	}
	if string(rem1) != gap1 {
		t.Fatalf("remediation-1.md = %q, want %q", string(rem1), gap1)
	}

	rem2, err := os.ReadFile(filepath.Join(gromitDir, "remediation-2.md"))
	if err != nil {
		t.Fatalf("remediation-2.md not found: %v", err)
	}
	if string(rem2) != gap2 {
		t.Fatalf("remediation-2.md = %q, want %q", string(rem2), gap2)
	}
}
