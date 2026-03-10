package loop

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/v2/adapter"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stageaccept "github.com/danabrams/gromit/internal/v2/stage/accept"
	specreview "github.com/danabrams/gromit/internal/v2/stage/specreview"
)

const (
	specFindingSeverityWarning    = stagepkg.SpecFindingSeverity("warning")
	specFindingSeveritySuggestion = stagepkg.SpecFindingSeverity("suggestion")
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
			Artifacts: &stageaccept.AcceptArtifacts{SpecFindings: []stagepkg.SpecFinding{
				acceptFinding,
			}},
		},
		stagepkg.Result{Decision: stagepkg.DecisionProceed},
	)

	reviewFinding := specreview.SpecReviewFinding{
		Title:       "spec review issue",
		Description: "value drift",
		Severity:    specFindingSeverityWarning,
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

func TestIntegration_SpecLoop_FromReviewWarningsCreateBeads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-loop-from-review"
	cfg := &config.Config{
		Paths:  config.PathsConfig{GromitDir: ".gromit"},
		Models: config.ModelsConfig{P0: "p0-model"},
	}

	git := newIntegrationGitAdapter(t)

	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()

	acceptStage := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionProceed})

	reviewFindings := []specreview.SpecReviewFinding{
		{
			Title:       "fix spec detail",
			Description: "adjust spec behavior",
			Severity:    specFindingSeverityWarning,
			Category:    stagepkg.SpecFindingCategoryQuality,
			Scope:       stagepkg.SpecFindingScopeSpec,
		},
		{
			Title:       "general observation",
			Description: "nice to have improvement",
			Severity:    specFindingSeveritySuggestion,
			Category:    stagepkg.SpecFindingCategoryQuality,
			Scope:       stagepkg.SpecFindingScopeBead,
		},
	}
	specReviewStage := newFakeSpecReviewStage(stagepkg.Result{
		Decision: stagepkg.DecisionProceed,
		Artifacts: &specreview.SpecReviewArtifacts{
			Verdict:  "pass",
			Findings: reviewFindings,
		},
	})

	taskTracker := newRecordingTaskTracker()

	presenter := newIntegrationPresenterAdapter(t)
	presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)

	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadRunner),
		WithAcceptStage(acceptStage),
		WithSpecReviewStage(specReviewStage),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if got, want := len(taskTracker.created), len(reviewFindings); got != want {
		t.Fatalf("created beads = %d, want %d", got, want)
	}

	specLabel := tracker.SpecLabelFor(specID)

	first := taskTracker.created[0]
	if !hasLabel(first.Labels, "from-review") {
		t.Fatalf("first bead labels = %v, missing from-review", first.Labels)
	}
	if !hasLabel(first.Labels, specLabel) {
		t.Fatalf("first bead labels = %v, missing spec label", first.Labels)
	}

	second := taskTracker.created[1]
	if !hasLabel(second.Labels, "from-review") {
		t.Fatalf("second bead labels = %v, missing from-review", second.Labels)
	}
	if hasLabel(second.Labels, specLabel) {
		t.Fatalf("second bead labels = %v, unexpected spec label", second.Labels)
	}
}

func hasLabel(labels []string, target string) bool {
	for _, label := range labels {
		if label == target {
			return true
		}
	}
	return false
}
