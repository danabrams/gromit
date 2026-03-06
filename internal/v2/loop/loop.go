package loop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/presentation"
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
	s.recordStage("gate")
	s.recordStage("build")
	s.recordStage("validate")
	s.recordStage("review")
	s.recordStage("epilogue")
	s.recordStage("accept")

	specBranch := presentation.SpecBranchName(specID)
	integrationBranch := strings.TrimSpace(s.cfg.Git.BaseBranch)
	if integrationBranch == "" {
		integrationBranch = presentation.DefaultIntegrationBranch()
	}
	summary := presentation.PresentationSummary{
		SpecName:          specID,
		SpecBranch:        specBranch,
		IntegrationBranch: integrationBranch,
		Plan:              plan,
		Worktree:          worktree,
	}

	if err := s.ensureAcceptance(ctx, specID); err != nil {
		return s.handleFailure(ctx, specID, summary, err)
	}

	summary.Success = true
	if err := s.presentSummary(ctx, specID, summary); err != nil {
		return err
	}

	if err := os.RemoveAll(worktree); err != nil {
		return fmt.Errorf("cleanup worktree: %w", err)
	}

	s.emit(&events.SpecCompletedEvent{SpecID: specID, Worktree: worktree, Success: true})

	return nil
}

func (s *SpecLoop) ensureAcceptance(ctx context.Context, specID string) error {
	failed, err := s.runAcceptStage(ctx, specID)
	if err != nil {
		return err
	}
	if !failed {
		return nil
	}
	if s.remediationRunner == nil {
		return fmt.Errorf("accept failed")
	}
	if err := s.remediationRunner.Run(ctx, specID); err != nil {
		return err
	}
	// Retry accept after successful remediation
	failed, err = s.runAcceptStage(ctx, specID)
	if err != nil {
		return err
	}
	if !failed {
		return nil
	}
	return fmt.Errorf("accept failed after remediation")
}

func (s *SpecLoop) runAcceptStage(ctx context.Context, specID string) (bool, error) {
	if s.acceptStage == nil {
		return false, nil
	}
	req := &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: specID},
		Config: s.cfg,
	}
	res, err := s.acceptStage.Run(ctx, req)
	if err != nil {
		return true, err
	}
	if res != nil && res.Decision == stagepkg.DecisionFail {
		return true, nil
	}
	return false, nil
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
