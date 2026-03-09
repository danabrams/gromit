package loop

import (
    "context"
    "fmt"
    "testing"

    "github.com/danabrams/gromit/internal/config"
    "github.com/danabrams/gromit/internal/v2/adapter"
    "github.com/danabrams/gromit/internal/v2/findings"
    "github.com/danabrams/gromit/internal/v2/llmtypes"
    "github.com/danabrams/gromit/internal/v2/remediation"
    stageaccept "github.com/danabrams/gromit/internal/v2/stage/accept"
    specreview "github.com/danabrams/gromit/internal/v2/stage/specreview"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestIntegration_SpecLoop_RemediationCreatesTargetedBeads(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    specID := "spec-loop-specreview-targeted"
    cfg := &config.Config{
        Paths:  config.PathsConfig{GromitDir: ".gromit"},
        Models: config.ModelsConfig{P0: "p0-model"},
    }

    git := newIntegrationGitAdapter(t)
    git.DiffOutput = "fake diff"

    llmAdapter := newTestLLMAdapter()
    reviewOutput := `{"passed": true}`
    llmAdapter.SetResponse("Spec-Level Code Review Instructions", &llmtypes.LLMInvokeResponse{Success: true, Output: reviewOutput})

    planStage := newFakePlanStage(specID)
    fakeDecompose := newFakeDecomposeStage(specID)
    beadLoop, err := NewBeadLoop(defaultIntegrationBeadLoopConfig())
    if err != nil {
        t.Fatalf("create bead loop: %v", err)
    }

    failure := stagepkg.Result{
        Decision: stagepkg.DecisionFail,
        Artifacts: &stageaccept.AcceptArtifacts{
            Findings: []findings.Finding{{
                Severity:    findings.SeverityCritical,
                Category:    "acceptance",
                Scope:       "spec",
                Description: "criterion failure",
            }},
            GapSummary: "gap summary",
        },
    }
    acceptStage := newScriptedAcceptStage(failure, stagepkg.Result{Decision: stagepkg.DecisionProceed})

    remediationBeadRunner := newFakeBeadRunner()
    remediationRunner := remediation.NewRemediationRunner(remediation.RemediationRunnerConfig{
        AcceptStage:    acceptStage,
        DecomposeStage: fakeDecompose,
        BeadRunner:     remediationBeadRunner,
        GenerationCap:  -1,
    })

    presenter := newIntegrationPresenterAdapter(t)
    presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)
    specReviewStage, err := specreview.New(cfg, git, llmAdapter, "", "", "")
    if err != nil {
        t.Fatalf("create specreview stage: %v", err)
    }

    adapters := adapter.AdapterSet{
        Git:         git,
        LLM:         llmAdapter,
        TaskTracker: newFakeTaskTrackerAdapter(),
        Presenter:   presenter,
    }

    loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
        WithPlanStage(planStage),
        WithPresentStage(presentStage, summaryCtx),
        WithDecomposeStage(fakeDecompose),
        WithBeadLoop(beadLoop),
        WithAcceptStage(acceptStage),
        WithRemediationRunner(remediationRunner),
        WithSpecReviewStage(specReviewStage),
    )
    if err != nil {
        t.Fatalf("create spec loop: %v", err)
    }

    if err := loopInstance.Run(ctx, specID, nil); err != nil {
        t.Fatalf("run spec loop: %v", err)
    }

    if !remediationBeadRunner.ran {
        t.Fatalf("remediation beads did not run")
    }
    if !fakeDecompose.remediationRequest {
        t.Fatalf("remediation decompose stage not invoked")
    }
    if len(fakeDecompose.remediationFindings) != len(failure.Artifacts.(*stageaccept.AcceptArtifacts).Findings) {
        t.Fatalf("remediation findings = %v, want %v", fakeDecompose.remediationFindings, failure.Artifacts.(*stageaccept.AcceptArtifacts).Findings)
    }
    if acceptStage.calls < 3 {
        t.Fatalf("accept calls = %d, want at least 3", acceptStage.calls)
    }
}
