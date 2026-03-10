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
	stageaccept "github.com/danabrams/gromit/internal/v2/stage/accept"
	"github.com/danabrams/gromit/internal/v2/stage/finding"
	"github.com/danabrams/gromit/internal/v2/stage/specreview"
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

func TestRemediationRunnerUsesConfiguredFindings(t *testing.T) {
    t.Parallel()

    configFindings := []finding.Finding{{
        Title:         "targeted fix",
        Severity:      finding.SeverityWarning,
        Category:      finding.CategoryQuality,
        Scope:         "spec",
        Description:   "resolve gap",
        AffectedFiles: []string{"internal/v2/remediation/remediation.go"},
    }}

    var capturedReq *stage.Request
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
        Findings:       append([]finding.Finding(nil), configFindings...),
    })

    if err := runner.Run(context.Background(), "spec-config", "", nil); err != nil {
        t.Fatalf("run failed: %v", err)
    }

    if capturedReq == nil {
        t.Fatal("decompose was not invoked")
    }

    if len(capturedReq.Findings) != len(configFindings) {
        t.Fatalf("findings count = %d, want %d", len(capturedReq.Findings), len(configFindings))
    }
    got := capturedReq.Findings[0]
    want := configFindings[0]
    if got.Title != want.Title {
        t.Fatalf("finding title = %q, want %q", got.Title, want.Title)
    }
    if got.Description != want.Description {
        t.Fatalf("finding description = %q, want %q", got.Description, want.Description)
    }
    if got.Severity != stage.Severity(want.Severity) {
        t.Fatalf("finding severity = %q, want %q", got.Severity, stage.Severity(want.Severity))
    }
    if got.Category != stage.Category(want.Category) {
        t.Fatalf("finding category = %q, want %q", got.Category, stage.Category(want.Category))
    }
    if got.Scope != stage.Scope(want.Scope) {
        t.Fatalf("finding scope = %q, want %q", got.Scope, stage.Scope(want.Scope))
    }
    if len(got.AffectedFiles) != len(want.AffectedFiles) {
        t.Fatalf("affected files count = %d, want %d", len(got.AffectedFiles), len(want.AffectedFiles))
    }
    if got.AffectedFiles[0] != want.AffectedFiles[0] {
        t.Fatalf("affected file = %q, want %q", got.AffectedFiles[0], want.AffectedFiles[0])
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

// planArtifactsWithContent is a test helper implementing planContentProvider.
type planArtifactsWithContent struct {
	plan string
}

func (p *planArtifactsWithContent) GetPlanContent() string {
	return p.plan
}

func TestRemediation_PersistsPlanContentOverGapAnalysis(t *testing.T) {
	t.Parallel()

	worktree := t.TempDir()
	gapText := "Criterion 1 failed: missing integration"
	planText := "# Remediation Plan\n\n## Tasks\n1. Add integration tests"

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
			return &stage.Result{
				Artifacts: &planArtifactsWithContent{plan: planText},
			}, nil
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

	if err := runner.Run(context.Background(), "spec-plan-content", worktree); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	gromitDir := filepath.Join(worktree, ".gromit", "v2")
	remContent, err := os.ReadFile(filepath.Join(gromitDir, "remediation-1.md"))
	if err != nil {
		t.Fatalf("remediation-1.md not found: %v", err)
	}
	if string(remContent) != planText {
		t.Fatalf("remediation-1.md = %q, want plan content %q", string(remContent), planText)
	}
}

func TestRemediation_FallsBackToGapAnalysisWhenNoPlanContent(t *testing.T) {
	t.Parallel()

	worktree := t.TempDir()
	gapText := "Criterion 3 failed: no event log"

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

	// Plan stage returns empty artifacts (no plan content).
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

	if err := runner.Run(context.Background(), "spec-fallback", worktree); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	gromitDir := filepath.Join(worktree, ".gromit", "v2")
	remContent, err := os.ReadFile(filepath.Join(gromitDir, "remediation-1.md"))
	if err != nil {
		t.Fatalf("remediation-1.md not found: %v", err)
	}
	if string(remContent) != gapText {
		t.Fatalf("remediation-1.md = %q, want gap analysis %q", string(remContent), gapText)
	}
}

func TestRemediationRunnerCollectsAcceptAndSpecReviewFindings(t *testing.T) {
	t.Parallel()

	gapText := "Criterion 4 failed: no docs"
	acceptFindings := []stage.SpecFinding{{
		Title:       "missing docs",
		Description: "Document the APIs",
		Severity:    stage.SpecFindingSeverityHigh,
		Category:    stage.SpecFindingCategoryQuality,
		Scope:       stage.SpecFindingScopeSpec,
	}}
	specReviewFinding := specreview.SpecReviewFinding{
		Title:       "cleanup",
		Description: "Remove dead code",
		Severity:    stage.SpecFindingSeverityMedium,
		Category:    stage.SpecFindingCategoryQuality,
		Scope:       stage.SpecFindingScopeGeneral,
	}

	var acceptCalls, specReviewCalls int
	accept := &testStage{
		name: "accept",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			acceptCalls++
			if acceptCalls == 1 {
				return &stage.Result{
					Decision: stage.DecisionFail,
					Artifacts: &stageaccept.AcceptArtifacts{
						GapSummary:   gapText,
						SpecFindings: append([]stage.SpecFinding(nil), acceptFindings...),
					},
				}, nil
			}
			return &stage.Result{Decision: stage.DecisionProceed}, nil
		},
	}

	specReview := &testStage{
		name: "spec-review",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			specReviewCalls++
			return &stage.Result{
				Decision: stage.DecisionFail,
				Artifacts: &specreview.SpecReviewArtifacts{
					Findings: []specreview.SpecReviewFinding{specReviewFinding},
				},
			}, nil
		},
	}

	var capturedReq *stage.Request
	decompose := &testStage{
		name: "decompose",
		run: func(ctx context.Context, req *stage.Request) (*stage.Result, error) {
			capturedReq = req
			return &stage.Result{
				Artifacts: &stage.DecomposeArtifacts{
					Beads: []*bead.Bead{{ID: "fix"}},
				},
			}, nil
		},
	}

	runner := NewRemediationRunner(RemediationRunnerConfig{
		AcceptStage:     accept,
		SpecReviewStage: specReview,
		DecomposeStage:  decompose,
		BeadRunner:      &testBeadRunner{},
		GenerationCap:   DefaultGenerationCap,
	})

	if err := runner.Run(context.Background(), "spec-findings", ""); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if specReviewCalls != 1 {
		t.Fatalf("spec review calls = %d, want 1", specReviewCalls)
	}
	if capturedReq == nil {
		t.Fatal("decompose stage not invoked")
	}
	if capturedReq.GapAnalysis != gapText {
		t.Fatalf("gap analysis = %q, want %q", capturedReq.GapAnalysis, gapText)
	}
	if len(capturedReq.Findings) != 2 {
		t.Fatalf("findings = %d, want 2", len(capturedReq.Findings))
	}
}

func TestRemediation_RemediationBeadsCarrySpecLabel(t *testing.T) {
	// SKIP: The spec label (e.g. "spec:SPECID") is applied by the decompose
	// stage (internal/v2/stage/decompose), not by the remediation runner.
	// The remediation runner receives beads from DecomposeStage.Run() with
	// labels already attached. Testing spec-label presence here would require
	// wiring up the real decompose stage, which is an integration-level concern.
	// The unit-level spec-label tests belong in internal/v2/stage/decompose/.
	t.Skip("spec labels are applied by the decompose stage, not the remediation runner")
}

func TestConvertSpecFindingsPreservesTitle(t *testing.T) {
	t.Parallel()

	src := []stage.SpecFinding{{
		Title:       "document security posture",
		Description: "missing defender plan",
		Severity:    stage.SpecFindingSeverityCritical,
		Category:    stage.SpecFindingCategorySafety,
		Scope:       stage.SpecFindingScopeSpec,
	}}

	got := convertSpecFindings(src)
	if len(got) != 1 {
		t.Fatalf("convertSpecFindings returned %d entries, want 1", len(got))
	}

	if got[0].Title != src[0].Title {
		t.Fatalf("converted finding Title = %q, want %q", got[0].Title, src[0].Title)
	}
}

func TestConvertSpecFindingsCopiesAffectedFiles(t *testing.T) {
	t.Parallel()

	src := []stage.SpecFinding{{
		Title:         "missing logs",
		Description:   "audit trail gap",
		Severity:      stage.SpecFindingSeverityWarning,
		Category:      stage.SpecFindingCategoryQuality,
		Scope:         stage.SpecFindingScopeSpec,
		AffectedFiles: []string{"audit/log.md", "docs/process.md"},
	}}

	got := convertSpecFindings(src)
	if len(got) != 1 {
		t.Fatalf("convertSpecFindings returned %d entries, want 1", len(got))
	}

	if len(got[0].AffectedFiles) != len(src[0].AffectedFiles) {
		t.Fatalf("converted affected files count = %d, want %d", len(got[0].AffectedFiles), len(src[0].AffectedFiles))
	}
	for i, want := range src[0].AffectedFiles {
		if got[0].AffectedFiles[i] != want {
			t.Fatalf("affected file[%d] = %q, want %q", i, got[0].AffectedFiles[i], want)
		}
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

	if err := runner.Run(context.Background(), "spec-plan", "", nil); err != nil {
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

	if err := runner.Run(context.Background(), "spec-persist", worktree, nil); err != nil {
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

	if err := runner.Run(context.Background(), "spec-multi", worktree, nil); err != nil {
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

// planArtifactsWithContent is a test helper implementing planContentProvider.
type planArtifactsWithContent struct {
	plan string
}

func (p *planArtifactsWithContent) GetPlanContent() string {
	return p.plan
}

func TestRemediation_PersistsPlanContentOverGapAnalysis(t *testing.T) {
	t.Parallel()

	worktree := t.TempDir()
	gapText := "Criterion 1 failed: missing integration"
	planText := "# Remediation Plan\n\n## Tasks\n1. Add integration tests"

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
			return &stage.Result{
				Artifacts: &planArtifactsWithContent{plan: planText},
			}, nil
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

	if err := runner.Run(context.Background(), "spec-plan-content", worktree, nil); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	gromitDir := filepath.Join(worktree, ".gromit", "v2")
	remContent, err := os.ReadFile(filepath.Join(gromitDir, "remediation-1.md"))
	if err != nil {
		t.Fatalf("remediation-1.md not found: %v", err)
	}
	if string(remContent) != planText {
		t.Fatalf("remediation-1.md = %q, want plan content %q", string(remContent), planText)
	}
}

func TestRemediation_FallsBackToGapAnalysisWhenNoPlanContent(t *testing.T) {
	t.Parallel()

	worktree := t.TempDir()
	gapText := "Criterion 3 failed: no event log"

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

	// Plan stage returns empty artifacts (no plan content).
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

	if err := runner.Run(context.Background(), "spec-fallback", worktree, nil); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	gromitDir := filepath.Join(worktree, ".gromit", "v2")
	remContent, err := os.ReadFile(filepath.Join(gromitDir, "remediation-1.md"))
	if err != nil {
		t.Fatalf("remediation-1.md not found: %v", err)
	}
	if string(remContent) != gapText {
		t.Fatalf("remediation-1.md = %q, want gap analysis %q", string(remContent), gapText)
	}
}

func TestRemediation_RemediationBeadsCarrySpecLabel(t *testing.T) {
	// SKIP: The spec label (e.g. "spec:SPECID") is applied by the decompose
	// stage (internal/v2/stage/decompose), not by the remediation runner.
	// The remediation runner receives beads from DecomposeStage.Run() with
	// labels already attached. Testing spec-label presence here would require
	// wiring up the real decompose stage, which is an integration-level concern.
	// The unit-level spec-label tests belong in internal/v2/stage/decompose/.
	t.Skip("spec labels are applied by the decompose stage, not the remediation runner")
}
