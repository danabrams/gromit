package loop

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stageaccept "github.com/danabrams/gromit/internal/v2/stage/accept"
	specreview "github.com/danabrams/gromit/internal/v2/stage/specreview"
	"github.com/danabrams/gromit/internal/v2/trackertypes"
)

func TestIntegration_SpecLoop_RemediationFindingsTargetedBeads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-integration-remediation-findings"
	projectRoot := filepath.Clean(".")
	cfg := &config.Config{
		ProjectRoot: projectRoot,
		Paths:       config.PathsConfig{GromitDir: ".gromit"},
	}

	baseInstructions, err := loadBaseInstructions(projectRoot)
	if err != nil {
		t.Fatalf("load base instructions: %v", err)
	}
	projectContext, err := loadProjectContext(projectRoot)
	if err != nil {
		t.Fatalf("load project context: %v", err)
	}
	specReviewFragment, err := loadFragment(projectRoot, "review_spec_v2.md")
	if err != nil {
		t.Fatalf("load spec review fragment: %v", err)
	}

	git := newIntegrationGitAdapter(t)
	taskTracker := newIntegrationTaskTrackerAdapter()
	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()

	acceptFinding := stagepkg.SpecFinding{
		Title:       "acceptance gap",
		Description: "missing behavior",
		Severity:    stagepkg.SpecFindingSeverityCritical,
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

	specReviewDescription := "spec review indicates a drift"
	failureResponse := &llm.LLMInvokeResponse{
		Success: true,
		Output:  fmt.Sprintf(`{"verdict":"issue","summary":"issue","findings":[{"verdict":"issue","severity":"critical","category":"bug","scope":"spec","description":"%s","affected_files":["internal/v2/loop/spec_loop.go"]}]}`, specReviewDescription),
	}
	passResponse := &llm.LLMInvokeResponse{
		Success: true,
		Output:  `{"verdict":"pass","summary":"ok","findings":[]}`,
	}
	provider := newSequentialLLMProvider(failureResponse, passResponse)
	specReviewStage, err := specreview.New(cfg, git, provider, taskTracker, baseInstructions, projectContext, specReviewFragment)
	if err != nil {
		t.Fatalf("create spec review stage: %v", err)
	}

	decompose.onRun = func() {
		if decompose.lastRequest == nil || !decompose.lastRequest.Remediation {
			return
		}
		beads := make([]*bead.Bead, len(decompose.lastRequest.Findings))
		for i, f := range decompose.lastRequest.Findings {
			beads[i] = &bead.Bead{
				ID:          fmt.Sprintf("%s-remediation-%d", specID, i),
				Title:       f.Description,
				Description: fmt.Sprintf("Targeted fix for %s", f.Description),
				Priority:    1,
			}
		}
		decompose.producedBeads = beads
	}

	remediationRunner := &targetedRemediationRunner{
		decompose:  decompose,
		beadRunner: beadRunner,
	}

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
		WithRemediationRunner(remediationRunner),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	expectedDescriptions := []string{acceptFinding.Description, specReviewDescription}
	if len(remediationRunner.findings) != len(expectedDescriptions) {
		t.Fatalf("remediation findings = %d, want %d", len(remediationRunner.findings), len(expectedDescriptions))
	}
	for i, expected := range expectedDescriptions {
		if remediationRunner.findings[i].Description != expected {
			t.Fatalf("remediation finding[%d] = %q, want %q", i, remediationRunner.findings[i].Description, expected)
		}
	}

	if !decompose.remediationRequest {
		t.Fatalf("decompose not invoked for remediation")
	}
	if len(decompose.remediationFindings) != len(expectedDescriptions) {
		t.Fatalf("recorded remediation findings = %d, want %d", len(decompose.remediationFindings), len(expectedDescriptions))
	}
	for i, expected := range expectedDescriptions {
		if decompose.remediationFindings[i].Description != expected {
			t.Fatalf("decompose finding[%d] = %q, want %q", i, decompose.remediationFindings[i].Description, expected)
		}
	}

	if remediationRunner.request == nil {
		t.Fatalf("remediation runner request not captured")
	}
	if len(remediationRunner.request.Findings) != len(expectedDescriptions) {
		t.Fatalf("request findings = %d, want %d", len(remediationRunner.request.Findings), len(expectedDescriptions))
	}
	for i, expected := range expectedDescriptions {
		if remediationRunner.request.Findings[i].Description != expected {
			t.Fatalf("request finding[%d] = %q, want %q", i, remediationRunner.request.Findings[i].Description, expected)
		}
	}

	if len(beadRunner.lastBeads) != len(expectedDescriptions) {
		t.Fatalf("bead loop ran with %d beads, want %d", len(beadRunner.lastBeads), len(expectedDescriptions))
	}
	for i, bead := range beadRunner.lastBeads {
		if bead.Title != expectedDescriptions[i] {
			t.Fatalf("bead[%d].Title = %q, want %q", i, bead.Title, expectedDescriptions[i])
		}
	}
}

func TestIntegration_SpecLoop_PassWithImprovementsCreatesDeferredBeads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-integration-pass-with-improvements"
	projectRoot := filepath.Clean(".")
	cfg := &config.Config{
		ProjectRoot: projectRoot,
		Paths:       config.PathsConfig{GromitDir: ".gromit"},
		SpecGate:    config.SpecGateConfig{Model: config.ModelOpus},
	}

	baseInstructions, err := loadBaseInstructions(projectRoot)
	if err != nil {
		t.Fatalf("load base instructions: %v", err)
	}
	projectContext, err := loadProjectContext(projectRoot)
	if err != nil {
		t.Fatalf("load project context: %v", err)
	}
	specReviewFragment, err := loadFragment(projectRoot, "review_spec_v2.md")
	if err != nil {
		t.Fatalf("load spec review fragment: %v", err)
	}

	git := newIntegrationGitAdapter(t)
	taskTracker := newIntegrationTaskTrackerAdapter()
	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	beadRunner := newFakeBeadRunner()

	acceptStage := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionProceed})

	passResponse := &llm.LLMInvokeResponse{
		Success: true,
		Output: `{"verdict":"pass","summary":"pass with improvements","findings":[` +
			`{"verdict":"pass","severity":"warning","category":"quality","scope":"spec","description":"Improve spec detail","affected_files":["internal/v2/loop/spec_loop.go"]},` +
			`{"verdict":"pass","severity":"suggestion","category":"architecture","scope":"stage","description":"General cleanup","affected_files":["docs/README.md"]}` +
			`]}`,
	}
	provider := newSequentialLLMProvider(passResponse)
	specReviewStage, err := specreview.New(cfg, git, provider, taskTracker, baseInstructions, projectContext, specReviewFragment)
	if err != nil {
		t.Fatalf("create spec review stage: %v", err)
	}

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

	assertPresenterSuccess(t, presenter, true)

	beads, err := collectOpenFromReviewBeads(ctx, taskTracker)
	if err != nil {
		t.Fatalf("collect from-review beads: %v", err)
	}
	if len(beads) != 2 {
		t.Fatalf("found %d from-review beads, want 2", len(beads))
	}

	specLabel := tracker.SpecLabelFor(specID)
	specCount := 0
	generalCount := 0
	for _, bead := range beads {
		if bead.Status != tracker.StatusOpen {
			t.Fatalf("bead %s status = %q, want %q", bead.ID, bead.Status, tracker.StatusOpen)
		}
		if !labelContains(bead.Labels, "from-review") {
			t.Fatalf("bead %s missing from-review label: %v", bead.ID, bead.Labels)
		}
		if labelContains(bead.Labels, specLabel) {
			specCount++
		} else {
			generalCount++
		}
	}
	if specCount != 1 {
		t.Fatalf("spec-scoped beads = %d, want 1", specCount)
	}
	if generalCount != 1 {
		t.Fatalf("general beads = %d, want 1", generalCount)
	}
}

type targetedRemediationRunner struct {
	decompose  *fakeDecomposeStage
	beadRunner *fakeBeadRunner
	request    *stagepkg.Request
	findings   []stagepkg.SpecFinding
}

func (r *targetedRemediationRunner) Run(ctx context.Context, specID, worktree string, findings []stagepkg.SpecFinding) error {
	r.findings = append([]stagepkg.SpecFinding(nil), findings...)
		req := stagepkg.Request{
			Bead:         stagepkg.BeadInfo{ID: specID},
			Worktree:     worktree,
			SpecFindings: append([]stagepkg.SpecFinding(nil), findings...),
		}
	r.request = &req
	res, err := r.decompose.Run(ctx, &req)
	if err != nil {
		return err
	}
	if res == nil || res.Artifacts == nil {
		return fmt.Errorf("remediation decompose returned nothing")
	}
	artifacts, ok := res.Artifacts.(*stagepkg.DecomposeArtifacts)
	if !ok {
		return fmt.Errorf("unexpected artifacts %T", res.Artifacts)
	}
	_, err = r.beadRunner.Run(ctx, artifacts.Beads, nil)
	return err
}

type sequentialLLMProvider struct {
	mu        sync.Mutex
	responses []*llm.LLMInvokeResponse
}

func newSequentialLLMProvider(responses ...*llm.LLMInvokeResponse) *sequentialLLMProvider {
	copied := append([]*llm.LLMInvokeResponse(nil), responses...)
	return &sequentialLLMProvider{responses: copied}
}

func (p *sequentialLLMProvider) Invoke(_ context.Context, req llm.LLMInvokeRequest) (*llm.LLMInvokeResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.responses) == 0 {
		return nil, fmt.Errorf("no fake LLM responses available")
	}
	resp := p.responses[0]
	p.responses = p.responses[1:]
	return resp, nil
}

func (p *sequentialLLMProvider) StreamInvoke(ctx context.Context, req llm.LLMStreamInvokeRequest) (*llm.LLMInvokeResponse, error) {
	resp, err := p.Invoke(ctx, llm.LLMInvokeRequest{Prompt: req.Prompt, Model: req.Model, Dir: req.Dir})
	if err != nil {
		return nil, err
	}
	if req.Output != nil && resp.Output != "" {
		_, _ = io.WriteString(req.Output, resp.Output)
	}
	return resp, nil
}

func collectOpenFromReviewBeads(ctx context.Context, trackerAdapter trackertypes.TaskTracker) ([]trackertypes.Bead, error) {
	if trackerAdapter == nil {
		return nil, fmt.Errorf("task tracker required")
	}
	resp, err := trackerAdapter.QueryBeads(ctx, trackertypes.TaskTrackerQueryBeadsRequest{
		Status: tracker.StatusOpen,
		Labels: []string{"from-review"},
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	return resp.Beads, nil
}

func labelContains(labels []string, target string) bool {
	for _, label := range labels {
		if label == target {
			return true
		}
	}
	return false
}
