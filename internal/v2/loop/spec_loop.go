package loop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/presentation"
	stageaccept "github.com/danabrams/gromit/internal/v2/stage/accept"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

const (
	defaultGromitDir = ".gromit"
	v2DirName        = "v2"
)

// StageSequence lists the canonical stages the spec loop emits.
var StageSequence = []string{
	"plan",
	"decompose",
	"gate",
	"build",
	"validate",
	"review",
	"epilogue",
	"accept",
	"present",
}

// StageRecorder observes the stage names executed within the spec loop.
type StageRecorder interface {
	RecordStage(name string)
}

// BeadRunner executes a set of beads via the inner loop.
type BeadRunner interface {
	Run(ctx context.Context, beads []*bead.Bead) error
}

type remediationRunner interface {
	Run(ctx context.Context, specID string) error
}

// SpecLoopOption configures optional behavior when constructing a SpecLoop.
type SpecLoopOption func(*SpecLoop)

// WithStageRecorder installs the provided recorder into the spec loop.
func WithStageRecorder(recorder StageRecorder) SpecLoopOption {
	return func(s *SpecLoop) {
		s.recorder = recorder
	}
}

// WithEmitter attaches an event emitter to the spec loop.
func WithEmitter(emitter *events.Emitter) SpecLoopOption {
	return func(s *SpecLoop) {
		s.emitter = emitter
	}
}

// WithRemediationRunner sets the runner the spec loop should invoke when acceptance fails.
func WithRemediationRunner(r remediationRunner) SpecLoopOption {
	return func(s *SpecLoop) {
		s.remediationRunner = r
	}
}

// WithAcceptStage configures the accept stage the loop should evaluate.
func WithAcceptStage(stage stagepkg.Stage) SpecLoopOption {
	return func(s *SpecLoop) {
		s.acceptStage = stage
	}
}

// WithDecomposeStage configures the stage that breaks the plan into beads.
func WithDecomposeStage(stage stagepkg.Stage) SpecLoopOption {
	return func(s *SpecLoop) {
		s.decomposeStage = stage
	}
}

// WithBeadLoop installs the bead runner the spec loop should use to execute beads.
func WithBeadLoop(runner BeadRunner) SpecLoopOption {
	return func(s *SpecLoop) {
		s.beadRunner = runner
	}
}

// AdapterSet alias exposes the adapter basket consumed by the run loop.
type AdapterSet = adapter.AdapterSet

// DependencyGate enforces spec-level dependency checks before a run executes.
type DependencyGate interface {
	EnsureSpecReady(ctx context.Context, specID string) error
}

// SpecLoop orchestrates the adapters that drive a single spec iteration.
type SpecLoop struct {
	adapters            adapter.AdapterSet
	cfg                 *config.Config
	gate                DependencyGate
	recorder            StageRecorder
	acceptStage         stagepkg.Stage
	emitter             *events.Emitter
	remediationRunner   remediationRunner
	gapAnalysisFilename string
	decomposeStage      stagepkg.Stage
	beadRunner          BeadRunner
}

// NewSpecLoop constructs a spec loop backed by the provided adapters and configuration.
func NewSpecLoop(adapters adapter.AdapterSet, cfg *config.Config, gate DependencyGate, opts ...SpecLoopOption) (*SpecLoop, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if adapters.Git == nil {
		return nil, fmt.Errorf("git adapter required")
	}
	if adapters.LLM == nil {
		return nil, fmt.Errorf("llm adapter required")
	}
	if adapters.TaskTracker == nil {
		return nil, fmt.Errorf("task tracker adapter required")
	}
	if adapters.Presenter == nil {
		return nil, fmt.Errorf("presenter adapter required")
	}
	if gate == nil {
		return nil, fmt.Errorf("dependency gate required")
	}
	loopInstance := &SpecLoop{adapters: adapters, cfg: cfg, gate: gate, gapAnalysisFilename: "gap-analysis.md"}
	for _, opt := range opts {
		opt(loopInstance)
	}
	return loopInstance, nil
}

// Run executes the configured adapters for the requested spec.
func (s *SpecLoop) Run(ctx context.Context, specID string) error {
	if specID == "" {
		return fmt.Errorf("spec ID required")
	}

	if err := s.gate.EnsureSpecReady(ctx, specID); err != nil {
		return fmt.Errorf("dependency gate: %w", err)
	}

	worktree, err := s.adapters.Git.Checkout(ctx, specID)
	if err != nil {
		return fmt.Errorf("checkout: %w", err)
	}

	s.emit(&events.SpecStartedEvent{SpecID: specID, Worktree: worktree})

	req := s.specStageRequest(specID, worktree)

	s.recordStage("plan")
	plan, err := s.adapters.LLM.GeneratePlan(ctx, specID)
	if err != nil {
		return fmt.Errorf("generate plan: %w", err)
	}

	if err := s.writePlanFile(worktree, plan); err != nil {
		return fmt.Errorf("persist plan: %w", err)
	}

	if err := s.adapters.TaskTracker.RecordPlan(ctx, specID, plan); err != nil {
		return fmt.Errorf("record plan: %w", err)
	}

	s.recordStage("decompose")
	beads, err := s.runDecompose(ctx, req)
	if err != nil {
		return err
	}

	s.recordBeadStages()
	if err := s.runBeadLoop(ctx, beads); err != nil {
		return err
	}

	baseSummary := s.buildSuccessSummary(specID, worktree, plan, beads, nil)

	s.recordStage("accept")
	acceptRes, err := s.ensureAcceptance(ctx, &req, specID)
	if err != nil {
		return s.handleFailure(ctx, specID, baseSummary, err)
	}

	summary := s.buildSuccessSummary(specID, worktree, plan, beads, acceptRes)
	if err := s.presentSummary(ctx, specID, summary); err != nil {
		return err
	}

	if err := os.RemoveAll(worktree); err != nil {
		return fmt.Errorf("cleanup worktree: %w", err)
	}

	s.emit(&events.SpecCompletedEvent{SpecID: specID, Worktree: worktree, Success: true})

	return nil
}

func (s *SpecLoop) runDecompose(ctx context.Context, req stagepkg.Request) ([]*bead.Bead, error) {
	if s.decomposeStage == nil {
		return nil, fmt.Errorf("decompose stage required")
	}
	res, err := s.decomposeStage.Run(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("decompose stage: %w", err)
	}
	if res == nil || res.Artifacts == nil {
		return nil, fmt.Errorf("decompose stage returned no artifacts")
	}
	artifacts, ok := res.Artifacts.(*stagepkg.DecomposeArtifacts)
	if !ok {
		return nil, fmt.Errorf("unexpected artifacts type from decompose stage")
	}
	return append([]*bead.Bead(nil), artifacts.Beads...), nil
}

func (s *SpecLoop) recordBeadStages() {
	for _, name := range []string{"gate", "build", "validate", "review", "epilogue"} {
		s.recordStage(name)
	}
}

func (s *SpecLoop) runBeadLoop(ctx context.Context, beads []*bead.Bead) error {
	if s.beadRunner == nil {
		return fmt.Errorf("bead runner required")
	}
	return s.beadRunner.Run(ctx, beads)
}

func (s *SpecLoop) ensureAcceptance(ctx context.Context, req *stagepkg.Request, specID string) (*stagepkg.Result, error) {
	res, err := s.runAcceptStage(ctx, req)
	if err != nil {
		return nil, err
	}
	if !s.acceptFailed(res) {
		return res, nil
	}
	if s.remediationRunner == nil {
		return res, fmt.Errorf("accept failed")
	}
	if err := s.remediationRunner.Run(ctx, specID); err != nil {
		return res, err
	}
	res, err = s.runAcceptStage(ctx, req)
	if err != nil {
		return res, err
	}
	if !s.acceptFailed(res) {
		return res, nil
	}
	return res, fmt.Errorf("accept failed after remediation")
}

func (s *SpecLoop) runAcceptStage(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if s.acceptStage == nil {
		return nil, nil
	}
	res, err := s.acceptStage.Run(ctx, req)
	if err != nil {
		return res, err
	}
	return res, nil
}

func (s *SpecLoop) acceptFailed(res *stagepkg.Result) bool {
	if res == nil {
		return false
	}
	return res.Decision == stagepkg.DecisionFail
}

func (s *SpecLoop) buildSuccessSummary(specID, worktree, plan string, beads []*bead.Bead, acceptRes *stagepkg.Result) presentation.PresentationSummary {
	integrationBranch := strings.TrimSpace(s.cfg.Git.BaseBranch)
	if integrationBranch == "" {
		integrationBranch = presentation.DefaultIntegrationBranch()
	}

	return presentation.PresentationSummary{
		SpecName:          specID,
		SpecBranch:        presentation.SpecBranchName(specID),
		IntegrationBranch: integrationBranch,
		Plan:              plan,
		Worktree:          worktree,
		BeadSummaries:     s.beadSummaries(beads),
		Success:           true,
		AcceptanceResults: s.extractAcceptanceResults(acceptRes),
	}
}

func (s *SpecLoop) extractAcceptanceResults(res *stagepkg.Result) []presentation.AcceptanceResult {
	if res == nil || res.Artifacts == nil {
		return nil
	}
	artifacts, ok := res.Artifacts.(*stageaccept.AcceptArtifacts)
	if !ok {
		return nil
	}
	return append([]presentation.AcceptanceResult(nil), artifacts.Results...)
}

func (s *SpecLoop) beadSummaries(beads []*bead.Bead) []presentation.BeadSummary {
	if len(beads) == 0 {
		return nil
	}
	summaries := make([]presentation.BeadSummary, 0, len(beads))
	for _, b := range beads {
		if b == nil {
			continue
		}
		summaries = append(summaries, presentation.BeadSummary{ID: b.ID, Title: b.Title, Description: b.Description})
	}
	return summaries
}

func (s *SpecLoop) specStageRequest(specID, worktree string) stagepkg.Request {
	return stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}, Config: s.cfg, Worktree: worktree}
}

func (s *SpecLoop) handleFailure(ctx context.Context, specID string, base presentation.PresentationSummary, failure error) error {
	s.recordStage("gap-analysis")
	gapSummary, err := s.readGapAnalysis(base.Worktree)
	if err != nil {
		return fmt.Errorf("read gap analysis: %w", err)
	}
	s.recordStage("decompose")
	s.recordStage("bead-loop")

	reason := fmt.Sprintf("spec %s remediation halted: %s", specID, failure.Error())
	s.emit(&events.AndonTriggeredEvent{SpecID: specID, Reason: reason})
	s.emit(&events.SpecFailedEvent{SpecID: specID, Worktree: base.Worktree, FailureReason: reason})

	summary := base
	summary.Success = false
	summary.RemainingWork = nil
	summary.FailureSummary = reason
	if gapSummary != "" {
		summary.FailureSummary = fmt.Sprintf("%s\n\nGap analysis:\n%s", reason, gapSummary)
		summary.RemainingWork = []string{gapSummary}
	}

	if err := s.presentSummary(ctx, specID, summary); err != nil {
		return fmt.Errorf("present failure summary: %w", err)
	}

	return fmt.Errorf("accept failure: %w", failure)
}

func (s *SpecLoop) presentSummary(ctx context.Context, specID string, summary presentation.PresentationSummary) error {
	s.recordStage("present")
	if err := s.adapters.Presenter.PresentSummary(ctx, specID, summary); err != nil {
		return fmt.Errorf("present summary: %w", err)
	}
	return nil
}

func (s *SpecLoop) readGapAnalysis(worktree string) (string, error) {
	if strings.TrimSpace(worktree) == "" {
		return "", nil
	}
	gromitDir := s.cfg.Paths.GromitDir
	if gromitDir == "" {
		gromitDir = defaultGromitDir
	}
	path := filepath.Join(worktree, gromitDir, v2DirName, s.gapAnalysisFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *SpecLoop) writePlanFile(worktree, plan string) error {
	if strings.TrimSpace(worktree) == "" {
		return fmt.Errorf("worktree required for plan persistence")
	}
	path := filepath.Join(worktree, "plan.md")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		return fmt.Errorf("create worktree directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(plan), 0o644); err != nil {
		return fmt.Errorf("write plan file: %w", err)
	}
	return nil
}

func (s *SpecLoop) recordStage(name string) {
	if s.recorder == nil {
		return
	}
	s.recorder.RecordStage(name)
}

func (s *SpecLoop) emit(evt events.Event) {
	if s.emitter == nil {
		return
	}
	s.emitter.Emit(evt)
}
