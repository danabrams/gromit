package loop

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/presentation"
	v2review "github.com/danabrams/gromit/internal/v2/review"
	"github.com/danabrams/gromit/internal/v2/routing"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stageaccept "github.com/danabrams/gromit/internal/v2/stage/accept"
	planstage "github.com/danabrams/gromit/internal/v2/stage/plan"
	present "github.com/danabrams/gromit/internal/v2/stage/present"
	"github.com/danabrams/gromit/internal/v2/trackertypes"
)

const (
	defaultGromitDir     = ".gromit"
	v2DirName            = "v2"
	maxAcceptanceRetries = 5
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

var ErrAcceptanceRetriesExceeded = errors.New("acceptance retries exceeded")

// StageRecorder observes the stage names executed within the spec loop.
type StageRecorder interface {
	RecordStage(name string)
}

// BeadRunner executes a set of beads via the inner loop.
type BeadRunner interface {
	Run(ctx context.Context, beads []*bead.Bead, stopCh <-chan struct{}) (BeadLoopResult, error)
}

type remediationRunner interface {
	Run(ctx context.Context, specID, worktree string, initialResult *stagepkg.Result) error
}

// SelectiveRevalidator checks beads for regressions and returns the subset
// that failed validation and need to be re-queued in the bead loop.
type SelectiveRevalidator interface {
	Revalidate(ctx context.Context, beads []*bead.Bead, worktree string) ([]*bead.Bead, error)
}

// GapAnalyzer provides the data needed for resume gap analysis: the set of
// files that changed since the last run and a mapping from bead ID to the
// files each bead touched.
type GapAnalyzer interface {
	Analyze(ctx context.Context, worktree string, beads []*bead.Bead) (changedFiles []string, beadFileMap map[string][]string, err error)
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

// WithTypedEmitter attaches a typed event emitter to the spec loop.
func WithTypedEmitter(em *event.Emitter) SpecLoopOption {
	return func(s *SpecLoop) {
		s.typedEmitter = em
	}
}

// LegacySubscriberWarmup is run when the spec loop checks out a worktree and
// before any typed events are emitted, allowing callers to set up event
// bridging or other legacy subscribers.
type LegacySubscriberWarmup func(ctx context.Context, specID, worktree string, typed *event.Emitter, legacy *events.Emitter) error

// WithLegacySubscriberWarmup installs the provided warmup hook.
func WithLegacySubscriberWarmup(warmup LegacySubscriberWarmup) SpecLoopOption {
	return func(s *SpecLoop) {
		s.legacySubscriberWarmup = warmup
	}
}

// WithStageCommitter installs a StageCommitter that creates a git commit after each spec-level stage.
func WithStageCommitter(sc StageCommitter) SpecLoopOption {
	return func(s *SpecLoop) {
		s.stageCommitter = sc
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

// WithPlanStage configures the plan stage the loop should execute.
func WithPlanStage(stage stagepkg.Stage) SpecLoopOption {
	return func(s *SpecLoop) {
		s.planStage = stage
	}
}

// WithSelectiveRevalidator installs a revalidator that checks completed beads
// for regressions before the bead loop starts on resume.
func WithSelectiveRevalidator(r SelectiveRevalidator) SpecLoopOption {
	return func(s *SpecLoop) {
		s.selectiveRevalidator = r
	}
}

// WithGapAnalyzer installs the gap analyzer used to identify which completed
// beads have changed files and need selective revalidation on resume.
func WithGapAnalyzer(a GapAnalyzer) SpecLoopOption {
	return func(s *SpecLoop) {
		s.gapAnalyzer = a
	}
}

// WithRouter configures the routing engine for provider/model selection.
func WithRouter(r *routing.Router) SpecLoopOption {
	return func(s *SpecLoop) {
		s.router = r
	}
}

// WithPhaseModels sets the phase-to-tier mapping for routing.
func WithPhaseModels(pm map[string]string) SpecLoopOption {
	return func(s *SpecLoop) {
		s.phaseModels = pm
	}
}

// WithPreserveOnFailure controls whether the worktree is kept when the spec
// fails. The default is true (preserve). Pass false to remove it on failure.
func WithPreserveOnFailure(preserve bool) SpecLoopOption {
	return func(s *SpecLoop) {
		s.preserveOnFailure = preserve
	}
}

// WithPresentStage injects the presentation stage and context used by the loop.
func WithPresentStage(stage stagepkg.Stage, ctx *present.SummaryContext) SpecLoopOption {
	return func(s *SpecLoop) {
		s.presentStage = stage
		s.presentSummaryContext = ctx
	}
}

func WithBeadSquasher(fn func(context.Context, string, []presentation.BeadSummary) error) SpecLoopOption {
	return func(s *SpecLoop) {
		s.beadSquasher = fn
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
	adapters               adapter.AdapterSet
	cfg                    *config.Config
	gate                   DependencyGate
	recorder               StageRecorder
	acceptStage            stagepkg.Stage
	emitter                *events.Emitter
	typedEmitter           *event.Emitter
	legacySubscriberWarmup LegacySubscriberWarmup
	stageCommitter         StageCommitter
	remediationRunner      remediationRunner
	gapAnalysisFilename    string
	decomposeStage         stagepkg.Stage
	beadRunner             BeadRunner
	planStage              stagepkg.Stage
	presentStage           stagepkg.Stage
	presentSummaryContext  *present.SummaryContext
	beadSquasher           func(context.Context, string, []presentation.BeadSummary) error
	preserveOnFailure      bool // restore t.Cleanup if overriding in tests
	selectiveRevalidator   SelectiveRevalidator
	gapAnalyzer            GapAnalyzer
	router                 *routing.Router
	phaseModels            map[string]string
}

type worktreeSetter interface {
	SetWorktree(string)
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
	loopInstance := &SpecLoop{adapters: adapters, cfg: cfg, gate: gate, gapAnalysisFilename: "gap-analysis.md", preserveOnFailure: true}
	for _, opt := range opts {
		opt(loopInstance)
	}
	return loopInstance, nil
}

// Run executes the configured adapters for the requested spec.
func (s *SpecLoop) Run(ctx context.Context, specID string, stopCh <-chan struct{}) (retErr error) {
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

	if err := s.runLegacySubscriberWarmup(ctx, specID, worktree); err != nil {
		return err
	}

	if s.typedEmitter != nil {
		fs := event.NewFileSubscriber(s.eventsFilePath(worktree))
		fs.SubscribeTo(s.typedEmitter)
		defer fs.Close()
		s.typedEmitter.Emit(event.SpecStartedEvent{
			Event:    event.Event{SchemaVersion: event.SchemaVersion, Timestamp: time.Now(), Type: event.EventTypeSpecStarted},
			SpecID:   specID,
			Worktree: worktree,
		})
	}

	var handleFailureCleaned bool
	var succeeded bool
	defer func() {
		if handleFailureCleaned || succeeded {
			return
		}
		if retErr != nil {
			if s.preserveOnFailure {
				return
			}
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cleanupCancel()
			_ = s.adapters.Git.RemoveWorktree(cleanupCtx, worktree)
		}
	}()

	s.emit(&events.SpecStartedEvent{SpecID: specID, Worktree: worktree})

	req := s.specStageRequest(specID, worktree)

	if err := s.ctxErr(ctx); err != nil {
		return err
	}

	var plan string
	s.recordStage("plan")
	planPath := s.planFilePath(worktree)
	if existingPlan, readErr := os.ReadFile(planPath); readErr == nil && len(strings.TrimSpace(string(existingPlan))) > 0 && planstage.ValidatePlanContent(string(existingPlan)) == nil {
		plan = string(existingPlan)
		s.emit(&events.PlanResumedEvent{SpecID: specID, Path: planPath})
	} else {
		s.applyRouting(&req, "plan")
		planRes, err := s.runPlanStage(ctx, req)
		if err != nil {
			return fmt.Errorf("plan stage: %w", err)
		}
		if planRes == nil || planRes.Artifacts == nil {
			return fmt.Errorf("plan stage returned no artifacts")
		}
		planArtifacts, ok := planRes.Artifacts.(*planstage.PlanArtifacts)
		if !ok {
			return fmt.Errorf("unexpected artifacts type from plan stage: %T", planRes.Artifacts)
		}
		plan = planArtifacts.Plan

		// Persist the plan so it survives a crash and can be resumed.
		if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
			return fmt.Errorf("create plan directory: %w", err)
		}
		if err := os.WriteFile(planPath, []byte(plan), 0o644); err != nil {
			return fmt.Errorf("persist plan: %w", err)
		}
		if err := s.commitStage(ctx, worktree, "plan", 0, "proceed"); err != nil {
			return fmt.Errorf("commit after plan: %w", err)
		}
	}

	if err := s.ctxErr(ctx); err != nil {
		return err
	}

	s.recordStage("decompose")
	var beads []*bead.Bead
	allBeads, err := s.beadsForSpec(ctx, specID)
	if err != nil {
		return fmt.Errorf("check existing beads: %w", err)
	}
	if len(allBeads) > 0 {
		// Filter for open beads in-memory instead of a second round-trip.
		var openBeads []*bead.Bead
		for _, b := range allBeads {
			if b.Status == tracker.StatusOpen {
				openBeads = append(openBeads, b)
			}
		}
		beads = openBeads
		s.emit(&events.DecomposeResumedEvent{SpecID: specID, BeadCount: len(beads)})
		if len(beads) > 0 && s.selectiveRevalidator != nil {
			// Gap analysis: identify which beads touched files that changed.
			revalidationCandidates := beads
			if s.gapAnalyzer != nil {
				changedFiles, beadFileMap, err := s.gapAnalyzer.Analyze(ctx, worktree, beads)
				if err != nil {
					return fmt.Errorf("gap analysis: %w", err)
				}
				if flagged := FlagChangedBeads(beads, changedFiles, beadFileMap); len(flagged) > 0 {
					revalidationCandidates = flagged
				}
			}
			requeueBeads, err := s.selectiveRevalidator.Revalidate(ctx, revalidationCandidates, worktree)
			if err != nil {
				return fmt.Errorf("selective revalidation: %w", err)
			}
			// Merge re-queued beads: mark existing ones in-place, append truly new ones.
			existingIDs := make(map[string]*bead.Bead, len(beads))
			for _, b := range beads {
				existingIDs[b.ID] = b
			}
			for _, rb := range requeueBeads {
				if existing, ok := existingIDs[rb.ID]; ok {
					existing.Status = tracker.StatusOpen
				} else {
					beads = append(beads, rb)
				}
			}
		}
	} else {
		s.applyRouting(&req, "decompose")
		beads, err = s.runDecompose(ctx, req)
		if err != nil {
			return err
		}
		if err := s.commitStage(ctx, worktree, "decompose", 0, "proceed"); err != nil {
			return fmt.Errorf("commit after decompose: %w", err)
		}
	}

	if err := s.ctxErr(ctx); err != nil {
		return err
	}

	s.recordBeadStages()
	beadResult, err := s.runBeadLoop(ctx, beads, worktree, stopCh)
	if err != nil {
		return err
	}

	baseSummary := s.buildSuccessSummary(specID, worktree, plan, beads, nil, beadResult.OutOfScopeFindings)

	if err := s.ctxErr(ctx); err != nil {
		return err
	}

	s.recordStage("accept")
	acceptRes, err := s.ensureAcceptance(ctx, &req, specID)
	if err != nil {
		handleFailureCleaned = true
		return s.handleFailure(ctx, specID, baseSummary, err)
	}
	if err := s.commitStage(ctx, worktree, "accept", 0, "proceed"); err != nil {
		return fmt.Errorf("commit after accept: %w", err)
	}

	summary := s.buildSuccessSummary(specID, worktree, plan, beads, acceptRes, beadResult.OutOfScopeFindings)

	if err := s.ctxErr(ctx); err != nil {
		return err
	}

	if err := s.presentSummary(ctx, specID, summary); err != nil {
		return err
	}

	if err := s.cleanupWorktree(ctx, cleanupOptions{specID: specID, worktree: worktree, success: true}); err != nil {
		return err
	}

	s.emit(&events.SpecCompletedEvent{SpecID: specID, Worktree: worktree, Success: true})

	succeeded = true
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

// beadsForSpec queries all beads (any status) for the given spec via a single
// round-trip. Callers filter by status in-memory when needed.
func (s *SpecLoop) beadsForSpec(ctx context.Context, specID string) ([]*bead.Bead, error) {
	if s.adapters.TaskTracker == nil {
		return nil, nil
	}
	label := fmt.Sprintf("spec:%s", specID)
	resp, err := s.adapters.TaskTracker.QueryBeads(ctx, trackertypes.TaskTrackerQueryBeadsRequest{
		Labels: []string{label},
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || len(resp.Beads) == 0 {
		return nil, nil
	}
	beads := make([]*bead.Bead, len(resp.Beads))
	for i, b := range resp.Beads {
		beads[i] = &bead.Bead{
			ID:          b.ID,
			Title:       b.Title,
			Description: b.Description,
			Priority:    b.Priority,
			Labels:      b.Labels,
			Status:      b.Status,
			DependsOn:   stringsToDependencies(b.DependsOn),
			BlockedBy:   stringsToDependencies(b.BlockedBy),
		}
	}
	return beads, nil
}

func (s *SpecLoop) eventsFilePath(worktree string) string {
	gromitDir := s.cfg.Paths.GromitDir
	if gromitDir == "" {
		gromitDir = defaultGromitDir
	}
	return filepath.Join(worktree, gromitDir, v2DirName, "events.jsonl")
}

func (s *SpecLoop) planFilePath(worktree string) string {
	gromitDir := s.cfg.Paths.GromitDir
	if gromitDir == "" {
		gromitDir = defaultGromitDir
	}
	return filepath.Join(worktree, gromitDir, v2DirName, "plan.md")
}

func (s *SpecLoop) runPlanStage(ctx context.Context, req stagepkg.Request) (*stagepkg.Result, error) {
	if s.planStage == nil {
		return nil, fmt.Errorf("plan stage required")
	}
	res, err := s.planStage.Run(ctx, &req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *SpecLoop) recordBeadStages() {
	for _, name := range []string{"gate", "build", "validate", "review", "epilogue"} {
		s.recordStage(name)
	}
}

func (s *SpecLoop) runBeadLoop(ctx context.Context, beads []*bead.Bead, worktree string, stopCh <-chan struct{}) (BeadLoopResult, error) {
	if s.beadRunner == nil {
		return BeadLoopResult{}, fmt.Errorf("bead runner required")
	}
	if setter, ok := s.beadRunner.(worktreeSetter); ok {
		setter.SetWorktree(worktree)
	}
	return s.beadRunner.Run(ctx, beads, stopCh)
}

func (s *SpecLoop) ensureAcceptance(ctx context.Context, req *stagepkg.Request, specID string) (*stagepkg.Result, error) {
	retriesRemaining := maxAcceptanceRetries
	for {
		if err := s.ctxErr(ctx); err != nil {
			return nil, err
		}

		s.applyRouting(req, "accept")
		res, err := s.runAcceptStage(ctx, req)
		if err != nil {
			return res, err
		}
		if !s.acceptFailed(res) {
			return res, nil
		}
		if s.remediationRunner == nil {
			return res, fmt.Errorf("accept failed")
		}
		if retriesRemaining <= 0 {
			return res, fmt.Errorf("%w: limit %d reached", ErrAcceptanceRetriesExceeded, maxAcceptanceRetries)
		}
		if setter, ok := s.remediationRunner.(completedBeadTitlesSetter); ok {
			titles := s.closedBeadTitles(ctx, specID)
			setter.SetCompletedBeadTitles(titles)
		}
		if err := s.remediationRunner.Run(ctx, specID, req.Worktree, res); err != nil {
			return res, err
		}
		// The remediation runner already verified acceptance internally.
		// Trust its result and return success to avoid a redundant
		// acceptance check that could trigger another remediation cycle.
		return nil, nil
	}
}

// completedBeadTitlesSetter is an optional interface that a remediationRunner
// may implement to receive bead titles that have already been completed.
type completedBeadTitlesSetter interface {
	SetCompletedBeadTitles(titles []string)
}

// closedBeadTitles queries all beads for the spec and returns the titles
// of those that have been closed, so remediation can avoid re-creating them.
func (s *SpecLoop) closedBeadTitles(ctx context.Context, specID string) []string {
	allBeads, err := s.beadsForSpec(ctx, specID)
	if err != nil || len(allBeads) == 0 {
		return nil
	}
	var titles []string
	for _, b := range allBeads {
		if b.Status == tracker.StatusClosed {
			titles = append(titles, b.Title)
		}
	}
	return titles
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

func (s *SpecLoop) buildSuccessSummary(specID, worktree, plan string, beads []*bead.Bead, acceptRes *stagepkg.Result, outOfScope []v2review.Finding) presentation.PresentationSummary {
	integrationBranch := strings.TrimSpace(s.cfg.Git.BaseBranch)
	if integrationBranch == "" {
		integrationBranch = presentation.DefaultIntegrationBranch()
	}

	return presentation.PresentationSummary{
		SpecName:           specID,
		SpecBranch:         presentation.SpecBranchName(specID),
		IntegrationBranch:  integrationBranch,
		Plan:               plan,
		Worktree:           worktree,
		BeadSummaries:      s.beadSummaries(beads),
		Success:            true,
		AcceptanceResults:  s.extractAcceptanceResults(acceptRes),
		OutOfScopeFindings: cloneOutOfScopeFindings(outOfScope),
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

func cloneOutOfScopeFindings(findings []v2review.Finding) []v2review.Finding {
	if len(findings) == 0 {
		return nil
	}
	clones := make([]v2review.Finding, len(findings))
	for i, finding := range findings {
		clones[i] = finding
		clones[i].AffectedFiles = append([]string(nil), finding.AffectedFiles...)
	}
	return clones
}

func (s *SpecLoop) runLegacySubscriberWarmup(ctx context.Context, specID, worktree string) error {
	if s.legacySubscriberWarmup == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.legacySubscriberWarmup(ctx, specID, worktree, s.typedEmitter, s.emitter); err != nil {
		return fmt.Errorf("legacy subscriber warmup: %w", err)
	}
	return nil
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

	if err := s.cleanupWorktree(ctx, cleanupOptions{specID: specID, worktree: base.Worktree, success: false}); err != nil {
		return err
	}

	s.emit(&events.SpecCompletedEvent{
		SpecID:        specID,
		Worktree:      base.Worktree,
		Success:       false,
		FailureReason: reason,
	})

	return fmt.Errorf("accept failure: %w", failure)
}

type cleanupOptions struct {
	specID   string
	worktree string
	success  bool
}

func (s *SpecLoop) cleanupWorktree(_ context.Context, opts cleanupOptions) error {
	trimmed := strings.TrimSpace(opts.worktree)
	specID := opts.specID
	if trimmed == "" {
		return nil
	}
	git := s.adapters.Git
	if git == nil {
		return fmt.Errorf("git adapter required for cleanup")
	}
	// Use a fresh context so cleanup succeeds even when the caller's context
	// has been cancelled (exec.CommandContext fails immediately otherwise).
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if !opts.success {
		status, err := git.Status(cleanupCtx, trimmed)
		if err != nil {
			log.Printf("git status during cleanup of spec %s: %v", specID, err)
		} else if strings.TrimSpace(status) != "" {
			message := fmt.Sprintf("[gromit: partial work] spec %s", specID)
			if _, err := git.Commit(cleanupCtx, trimmed, message); err != nil {
				log.Printf("commit partial work for spec %s: %v", specID, err)
			}
		}
		if s.preserveOnFailure {
			return nil
		}
	}
	if err := git.RemoveWorktree(cleanupCtx, trimmed); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}
	return nil
}

func (s *SpecLoop) presentSummary(ctx context.Context, specID string, summary presentation.PresentationSummary) error {
	if err := s.ctxErr(ctx); err != nil {
		return err
	}

	if s.beadSquasher != nil {
		if err := s.beadSquasher(ctx, summary.Worktree, summary.BeadSummaries); err != nil {
			return fmt.Errorf("squash: %w", err)
		}
	}

	s.recordStage("present")
	if s.presentStage == nil || s.presentSummaryContext == nil {
		return fmt.Errorf("present stage required")
	}
	s.populatePresentationContext(summary)
	req := s.specStageRequest(specID, summary.Worktree)
	res, err := s.presentStage.Run(ctx, &req)
	if err != nil {
		return err
	}
	if res != nil && res.Decision == stagepkg.DecisionFail {
		return fmt.Errorf("present stage failed")
	}
	s.emitTypedSpecCompleted(specID, summary.Worktree, true, "")
	if err := s.commitStage(ctx, summary.Worktree, "present", 0, "proceed"); err != nil {
		return fmt.Errorf("commit after present: %w", err)
	}
	return nil
}

func (s *SpecLoop) populatePresentationContext(summary presentation.PresentationSummary) {
	ctx := s.presentSummaryContext
	if ctx == nil {
		return
	}
	ctx.Plan = summary.Plan
	ctx.Worktree = summary.Worktree
	ctx.BeadSummaries = summary.BeadSummaries
	ctx.Success = summary.Success
	ctx.AcceptanceResults = summary.AcceptanceResults
	ctx.OutOfScopeFindings = summary.OutOfScopeFindings
	ctx.FailureSummary = summary.FailureSummary
	ctx.RemainingWork = summary.RemainingWork
	ctx.BranchLink = summary.BranchLink
	ctx.DiffLink = summary.DiffLink
	ctx.IntegrationBranch = summary.IntegrationBranch
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

func (s *SpecLoop) commitStage(ctx context.Context, worktree, stageName string, iteration int, decision string) error {
	if s.stageCommitter == nil {
		return nil
	}
	return s.stageCommitter.CommitStage(ctx, worktree, "", stageName, iteration, capitalizedDecision(decision))
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

func (s *SpecLoop) emitTypedSpecCompleted(specID, worktree string, success bool, failureReason string) {
	if s.typedEmitter == nil {
		return
	}
	s.typedEmitter.Emit(event.SpecCompletedEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeSpecCompleted,
		},
		SpecID:        specID,
		Worktree:      worktree,
		Success:       success,
		FailureReason: failureReason,
	})
}

func stringsToDependencies(ids []string) []bead.Dependency {
	if len(ids) == 0 {
		return nil
	}
	deps := make([]bead.Dependency, len(ids))
	for i, id := range ids {
		deps[i] = bead.Dependency{ID: id}
	}
	return deps
}

func (s *SpecLoop) ctxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// applyRouting populates req.Provider and req.Model via the Router, if configured.
func (s *SpecLoop) applyRouting(req *stagepkg.Request, phase string) {
	if s.router == nil {
		return
	}
	tier := routing.TierForPhase(phase, s.phaseModels, routing.TierMedium)
	req.Tier = string(tier)
	provider, model, _, err := s.router.Select(phase, tier)
	if err != nil {
		log.Printf("WARNING: routing failed for phase %s tier %s: %v (using default provider)", phase, tier, err)
	} else if provider != nil {
		req.Provider = provider
		req.Model = model
	}
}
