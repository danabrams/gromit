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
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/stage"
	stageaccept "github.com/danabrams/gromit/internal/v2/stage/accept"
	"github.com/danabrams/gromit/internal/v2/stage/finding"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	specreview "github.com/danabrams/gromit/internal/v2/stage/specreview"
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
	tracker := newIntegrationTaskTrackerAdapter()
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
			Artifacts: &stageaccept.AcceptArtifacts{Findings: []stagepkg.SpecFinding{
				acceptFinding,
			}},
		},
		stagepkg.Result{Decision: stagepkg.DecisionProceed},
	)

	specReviewDescription := "spec review indicates a drift"
	failureResponse := &llm.LLMInvokeResponse{
		Success: true,
		Output: fmt.Sprintf(`{"verdict":"issue","summary":"issue","findings":[{"verdict":"issue","severity":"critical","category":"bug","scope":"spec","description":"%s","affected_files":["internal/v2/loop/spec_loop.go"]}]}`, specReviewDescription),
	}
	passResponse := &llm.LLMInvokeResponse{
		Success: true,
		Output: `{"verdict":"pass","summary":"ok","findings":[]}`,
	}
	provider := newSequentialLLMProvider(failureResponse, passResponse)
	specReviewStage, err := specreview.New(cfg, git, provider, tracker, baseInstructions, projectContext, specReviewFragment)
	if err != nil {
		t.Fatalf("create spec review stage: %v", err)
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
		TaskTracker: tracker,
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

	if _, reviewErr := loopInstance.Run(ctx, specID, nil); reviewErr != nil {
		t.Fatalf("run spec loop: %v", reviewErr)
	}

	t.Fatalf("not implemented: verify remediation findings pipeline")
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
		Bead:        stagepkg.BeadInfo{ID: specID},
		Worktree:    worktree,
		Findings:    convertSpecFindings(findings),
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

func convertSpecFindings(src []stagepkg.SpecFinding) []finding.Finding {
	if len(src) == 0 {
		return nil
	}
	out := make([]finding.Finding, len(src))
	for i, entry := range src {
		out[i] = finding.Finding{
			Severity:      finding.Severity(entry.Severity),
			Category:      finding.Category(entry.Category),
			Scope:         string(entry.Scope),
			Description:   entry.Description,
			AffectedFiles: nil,
		}
	}
	return out
}
