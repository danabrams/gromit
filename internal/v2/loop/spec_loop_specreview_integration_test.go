package loop

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/findings"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/presentation"
	"github.com/danabrams/gromit/internal/v2/remediation"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stageaccept "github.com/danabrams/gromit/internal/v2/stage/accept"
	specreview "github.com/danabrams/gromit/internal/v2/stage/specreview"
	"github.com/danabrams/gromit/internal/v2/testutil"
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
	reviewOutput := `{"verdict": "pass", "findings": []}`
	llmAdapter.SetResponse("Spec-Level Review Instructions", &llmtypes.LLMInvokeResponse{Success: true, Output: reviewOutput})

	planStage := newFakePlanStage(specID)
	fakeDecompose := newFakeDecomposeStage(specID)
	beadLoop, err := NewBeadLoop(defaultIntegrationBeadLoopConfig())
	if err != nil {
		t.Fatalf("create bead loop: %v", err)
	}

	failure := stagepkg.Result{
		Decision: stagepkg.DecisionFail,
		Artifacts: &stageaccept.AcceptArtifacts{
			Results: []presentation.AcceptanceResult{
				{Title: "criterion", Description: "criterion failure"},
			},
			Findings: []findings.Finding{{
				Severity:    findings.SeverityCritical,
				Category:    "acceptance",
				Scope:       "spec",
				Description: "criterion failure",
			}},
			GapSummary: "gap summary",
		},
	}
	loopAccept := newScriptedAcceptStage(failure, stagepkg.Result{Decision: stagepkg.DecisionProceed})

	remediationBeadRunner := &recordingRemediationBeadRunner{}
	remediationRunner := remediation.NewRemediationRunner(remediation.RemediationRunnerConfig{
		DecomposeStage: fakeDecompose,
		BeadRunner:     remediationBeadRunner,
		GenerationCap:  -1,
	})

	presenter := newIntegrationPresenterAdapter(t)
	presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)
	specReviewStage, err := specreview.New(git, llmAdapter, "")
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
		WithAcceptStage(loopAccept),
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
	if len(remediationBeadRunner.beads) == 0 {
		t.Fatalf("remediation beads missing")
	}
	if !fakeDecompose.remediationRequest {
		t.Fatalf("remediation decompose stage not invoked")
	}
	if len(fakeDecompose.remediationFindings) != len(failure.Artifacts.(*stageaccept.AcceptArtifacts).Findings) {
		t.Fatalf("remediation findings = %v, want %v", fakeDecompose.remediationFindings, failure.Artifacts.(*stageaccept.AcceptArtifacts).Findings)
	}
	if loopAccept.calls < 2 {
		t.Fatalf("accept calls = %d, want at least 2", loopAccept.calls)
	}
}

func TestIntegration_SpecLoop_PassWithImprovementsCreatesFromReviewBeads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-loop-specreview-from-review"
	cfg := &config.Config{
		Paths:  config.PathsConfig{GromitDir: ".gromit"},
		Models: config.ModelsConfig{P0: "p0-model"},
	}

	git := newIntegrationGitAdapter(t)
	git.DiffOutput = "fake diff"

	llmAdapter := newTestLLMAdapter()
	reviewOutput := fmt.Sprintf(`{
		"verdict": "pass",
		"findings": [
			{
				"severity": "warning",
				"category": "quality",
				"scope": "spec",
				"description": "Fix spec issue %s",
				"affected_files": ["spec/%s.md"]
			},
			{
				"severity": "warning",
				"category": "quality",
				"scope": "spec",
				"description": "General observation",
				"affected_files": []
			}
		]
	}`, specID, specID)
	llmAdapter.SetResponse("Spec-Level Review Instructions", &llmtypes.LLMInvokeResponse{Success: true, Output: reviewOutput})

	planStage := newFakePlanStage(specID)
	fakeDecompose := newFakeDecomposeStage(specID)
	beadLoop, err := NewBeadLoop(defaultIntegrationBeadLoopConfig())
	if err != nil {
		t.Fatalf("create bead loop: %v", err)
	}

	acceptStage := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionProceed})

	presenter := newIntegrationPresenterAdapter(t)
	presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)
	specReviewStage, err := specreview.New(git, llmAdapter, "")
	if err != nil {
		t.Fatalf("create specreview stage: %v", err)
	}

	taskTracker := newFakeTaskTrackerAdapter()
	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         llmAdapter,
		TaskTracker: taskTracker,
		Presenter:   presenter,
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{},
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(fakeDecompose),
		WithBeadLoop(beadLoop),
		WithAcceptStage(acceptStage),
		WithSpecReviewStage(specReviewStage),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if len(taskTracker.created) != 2 {
		t.Fatalf("created beads = %d, want 2", len(taskTracker.created))
	}

	specLabel := tracker.SpecLabelFor(specID)

	first := taskTracker.created[0]
	if len(first.Labels) == 0 || first.Labels[0] != "from-review" {
		t.Fatalf("first bead labels = %v, want from-review first", first.Labels)
	}
	if first.Labels[len(first.Labels)-1] != specLabel {
		t.Fatalf("first bead last label = %q, want %q", first.Labels[len(first.Labels)-1], specLabel)
	}

	second := taskTracker.created[1]
	if !hasLabel(second.Labels, "from-review") {
		t.Fatalf("second bead labels = %v, missing from-review", second.Labels)
	}
	if !hasLabel(second.Labels, specLabel) {
		t.Fatalf("second bead labels = %v, missing spec label", second.Labels)
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

type recordingRemediationBeadRunner struct {
	ran   bool
	beads []*bead.Bead
}

func (r *recordingRemediationBeadRunner) Run(ctx context.Context, beads []*bead.Bead) error {
	r.ran = true
	r.beads = append([]*bead.Bead(nil), beads...)
	return nil
}

type testLLMAdapter struct {
	fake *testutil.FakeLLM
}

func newTestLLMAdapter() *testLLMAdapter {
	return &testLLMAdapter{fake: testutil.NewFakeLLM()}
}

func (a *testLLMAdapter) SetResponse(key string, resp *llmtypes.LLMInvokeResponse) {
	if resp == nil {
		return
	}
	a.fake.SetResponse(key, resp)
}

func (a *testLLMAdapter) Invoke(ctx context.Context, req llmtypes.LLMInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	return a.fake.Invoke(ctx, req)
}

func (a *testLLMAdapter) StreamInvoke(ctx context.Context, req llmtypes.LLMStreamInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	return a.fake.StreamInvoke(ctx, req)
}

func (a *testLLMAdapter) GeneratePlan(_ context.Context, specID string) (string, error) {
	return specID + "-plan", nil
}
