package loop

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stageaccept "github.com/danabrams/gromit/internal/v2/stage/accept"
	specreview "github.com/danabrams/gromit/internal/v2/stage/specreview"
)

func TestIntegration_SpecLoop_RemediationFindingsFlowToRunner(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-loop-remediation-findings"
	cfg := &config.Config{
		Paths:  config.PathsConfig{GromitDir: ".gromit"},
		Models: config.ModelsConfig{P0: "p0-model"},
	}

	git := newIntegrationGitAdapter(t)

	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()

	acceptFinding := stagepkg.SpecFinding{
		Title:       "acceptance gap",
		Description: "missing behavior",
		Severity:    stagepkg.SpecFindingSeverityHigh,
		Category:    stagepkg.SpecFindingCategoryAcceptance,
		Scope:       stagepkg.SpecFindingScopeSpec,
	}
	acceptStage := newScriptedAcceptStage(
		stagepkg.Result{
			Decision: stagepkg.DecisionFail,
			Artifacts: &stageaccept.AcceptArtifacts{Findings: []stagepkg.SpecFinding{
				acceptFinding,
			}},
		},
		stagepkg.Result{Decision: stagepkg.DecisionProceed},
	)

	reviewFinding := specreview.SpecReviewFinding{
		Title:       "spec review issue",
		Description: "value drift",
		Severity:    stagepkg.SpecFindingSeverityWarning,
		Category:    stagepkg.SpecFindingCategoryQuality,
		Scope:       stagepkg.SpecFindingScopeSpec,
	}
	specReviewStage := newFakeSpecReviewStage(
		stagepkg.Result{
			Decision: stagepkg.DecisionFail,
			Artifacts: &specreview.SpecReviewArtifacts{
				Verdict:  "issue",
				Findings: []specreview.SpecReviewFinding{reviewFinding},
			},
		},
		stagepkg.Result{
			Decision: stagepkg.DecisionProceed,
			Artifacts: &specreview.SpecReviewArtifacts{
				Verdict: "pass",
			},
		},
	)

	remediationRunner := &recordingRemediationRunner{}

	presenter := newIntegrationPresenterAdapter(t)
	presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: newFakeTaskTrackerAdapter(),
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(acceptStage),
		WithSpecReviewStage(specReviewStage),
		WithRemediationRunner(remediationRunner),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if remediationRunner.calls != 1 {
		t.Fatalf("remediation calls = %d, want 1", remediationRunner.calls)
	}
	if remediationRunner.lastSpecID != specID {
		t.Fatalf("remediation spec ID = %q, want %q", remediationRunner.lastSpecID, specID)
	}
	if len(remediationRunner.lastFindings) != 2 {
		t.Fatalf("remediation findings = %v, want accept+review", remediationRunner.lastFindings)
	}
}
