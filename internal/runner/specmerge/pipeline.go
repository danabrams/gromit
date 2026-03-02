package specmerge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/specgate"
)

const (
	specLabelPrefix      = "spec:"
	fixBeadPriorityLabel = "P1"
)

// beadQuery defines the subset of the bead client needed by Pipeline.
type beadQuery interface {
	ListWithLabel(ctx context.Context, label string) ([]*bead.Bead, error)
}

// Controller coordinates spec merge completion triggers.
type Controller interface {
	IsSpecComplete(ctx context.Context, specName string) (bool, error)
	Trigger(ctx context.Context, specName string) error
}

// Pipeline runs spec merge orchestration helpers.
type Pipeline struct {
	query      beadQuery
	emitter    CycleRecordEmitter
	logEmitter *events.Emitter
	flow       FlowExecutor
	fixDeps    FixBeadDependencies
	retryCap   int
	attemptsMu sync.Mutex
	attempts   map[string]int

	prClient   PRClient
	stateStore PRStateStore
	gitPush    func(ctx context.Context, branch string) error
}

// PRDeps holds optional PR creation dependencies.
type PRDeps struct {
	PRClient   PRClient
	StateStore PRStateStore
	GitPush    func(ctx context.Context, branch string) error
}

// WithPRDeps configures optional PR creation dependencies.
func (p *Pipeline) WithPRDeps(deps PRDeps) *Pipeline {
	if p == nil {
		return p
	}
	p.prClient = deps.PRClient
	p.stateStore = deps.StateStore
	p.gitPush = deps.GitPush
	return p
}

// WithLogEmitter attaches an event emitter for structured logging.
func (p *Pipeline) WithLogEmitter(emitter *events.Emitter) *Pipeline {
	if p == nil {
		return nil
	}
	p.logEmitter = emitter
	return p
}

// NewPipeline constructs a Pipeline with the provided bead query dependencies.
func NewPipeline(query beadQuery, emitter CycleRecordEmitter, flow FlowExecutor, fixDeps FixBeadDependencies, retryCap int) *Pipeline {
	if emitter == nil {
		emitter = NoopCycleRecordEmitter()
	}
	return &Pipeline{
		query:    query,
		emitter:  emitter,
		flow:     flow,
		fixDeps:  fixDeps,
		retryCap: retryCap,
		attempts: map[string]int{},
	}
}

// IsSpecComplete returns true when no open beads remain for the given spec.
func (p *Pipeline) IsSpecComplete(ctx context.Context, specName string) (bool, error) {
	if p == nil {
		return false, fmt.Errorf("pipeline is nil")
	}
	if p.query == nil {
		return false, fmt.Errorf("bead query is required")
	}
	specName = strings.TrimSpace(specName)
	if specName == "" {
		return false, fmt.Errorf("spec name is required")
	}

	label := specLabelPrefix + specName
	beads, err := p.query.ListWithLabel(ctx, label)
	if err != nil {
		return false, fmt.Errorf("list beads for spec %q: %w", specName, err)
	}
	for _, b := range beads {
		if b == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(b.Status), "closed") {
			continue
		}
		return false, nil
	}
	return true, nil
}

// Trigger is a placeholder that will eventually start the spec merge pipeline.
func (p *Pipeline) Trigger(ctx context.Context, specName string) error {
	if p == nil {
		return fmt.Errorf("pipeline is nil")
	}
	if p.flow == nil {
		return fmt.Errorf("trigger flow is not configured")
	}

	if _, err := p.flow.Run(ctx, specName); err != nil {
		var stageErr StageFailureError
		if errors.As(err, &stageErr) {
			attempt := p.incrementAttempt(specName)

			if p.fixDeps.BeadCreator != nil {
				opts := HandleStageFailureOptions{
					SpecName:     specName,
					Failures:     stageResultFailures(stageErr.Result),
					Priority:     fixBeadPriorityLabel,
					AttemptCount: attempt,
					RetryCap:     p.retryCap,
				}
				if handleErr := HandleStageFailure(ctx, p.fixDeps, opts); handleErr != nil {
					return fmt.Errorf("handle stage failure: %w", handleErr)
				}
			}

			if p.retryCap > 0 {
				capExceeded, capErr := CheckRetryCapExceeded(attempt, p.retryCap)
				if capErr != nil {
					return fmt.Errorf("check retry cap: %w", capErr)
				}
				if capExceeded {
					alert := EmitRetryCapReachedAlert(specName, p.retryCap)
					return errors.New(alert)
				}
			}

			return err
		}

		attempt := p.incrementAttempt(specName)
		if p.retryCap > 0 {
			capExceeded, capErr := CheckRetryCapExceeded(attempt, p.retryCap)
			if capErr != nil {
				return fmt.Errorf("check retry cap: %w", capErr)
			}
			if capExceeded {
				alert := EmitRetryCapReachedAlert(specName, p.retryCap)
				return errors.New(alert)
			}
		}

		return err
	}

	if p.prClient != nil {
		branch := "gromit/spec-" + specName
		if p.gitPush != nil {
			if err := p.gitPush(ctx, branch); err != nil {
				return fmt.Errorf("push spec branch %q: %w", branch, err)
			}
		}

		title := fmt.Sprintf("Spec: %s", specName)
		body, _ := BuildPRSummary(ctx, PRSummaryInput{SpecName: specName})
		ref, err := p.prClient.CreatePR(ctx, title, body, branch, "main")
		if err != nil {
			return fmt.Errorf("create PR for spec %q: %w", specName, err)
		}

		if p.stateStore != nil {
			state := &PRState{
				SpecName:    specName,
				PRRef:       ref,
				Outcome:     PROutcomePending,
				LastUpdated: time.Now(),
			}
			if err := p.stateStore.Save(ctx, state); err != nil {
				return fmt.Errorf("save PR state for spec %q: %w", specName, err)
			}
		}
	}

	p.captureCycleRecord(ctx, specName)
	return nil
}

// FixBeadDependencies holds the dependencies needed to create fix beads.
type FixBeadDependencies struct {
	BeadCreator specgate.BeadCreator
}

// HandleStageFailureOptions holds the parameters for handling a stage failure.
type HandleStageFailureOptions struct {
	SpecName     string
	Failures     []specgate.CriterionResult
	Priority     string
	AttemptCount int
	RetryCap     int
}

// HandleStageFailure processes a stage failure by creating fix beads for failed criteria.
func HandleStageFailure(ctx context.Context, deps FixBeadDependencies, opts HandleStageFailureOptions) error {
	if deps.BeadCreator == nil {
		return fmt.Errorf("bead creator is required")
	}

	_, err := specgate.SynthesizeFixBeads(ctx, opts.SpecName, opts.Failures, opts.Priority, deps.BeadCreator)
	if err != nil {
		return fmt.Errorf("synthesize fix beads: %w", err)
	}

	return nil
}

func stageResultFailures(stage StageResult) []specgate.CriterionResult {
	if stage.ReviewResult == nil {
		return nil
	}
	evidence := strings.TrimSpace(stage.ReviewResult.Summary)
	if evidence == "" {
		evidence = fmt.Sprintf("stage %s failed to pass", stage.StageName)
	}
	return []specgate.CriterionResult{
		{
			Criterion: stage.StageName,
			Passed:    false,
			Evidence:  evidence,
		},
	}
}

// CheckRetryCapExceeded returns true if the attempt count has reached or exceeded the retry cap.
func CheckRetryCapExceeded(attemptCount, retryCap int) (bool, error) {
	return attemptCount >= retryCap, nil
}

// EmitRetryCapReachedAlert returns a terminal alert message when the retry cap is reached.
func EmitRetryCapReachedAlert(specName string, retryCap int) string {
	return fmt.Sprintf("merge pipeline for spec %q has reached retry cap of %d fix attempts", specName, retryCap)
}

// Stage1ValidationDependencies holds the dependencies required by RunStage1Validation.
type Stage1ValidationDependencies struct {
	CmdRunner runtypes.CmdRunnerFn
	GetDiff   func(ctx context.Context) (string, error)
}

// Stage1ValidationOptions configures how Stage 1 runs validation commands.
type Stage1ValidationOptions struct {
	Config          *config.Config
	WorkDir         string
	ATDDActive      bool
	TouchedPackages []string
}

// Stage1ValidationResult summarizes the outcome of the validation gate.
type Stage1ValidationResult struct {
	Success  bool
	Diff     string
	Failures []specgate.CriterionResult
}

// RunStage1Validation executes full validation commands and optional ATDD acceptance tests.
func RunStage1Validation(ctx context.Context, deps Stage1ValidationDependencies, opts Stage1ValidationOptions) (Stage1ValidationResult, error) {
	if deps.CmdRunner == nil {
		return Stage1ValidationResult{}, fmt.Errorf("cmd runner is required")
	}

	diff, err := resolveDiff(ctx, deps.GetDiff)
	if err != nil {
		return Stage1ValidationResult{}, fmt.Errorf("get diff: %w", err)
	}

	if opts.Config == nil || !opts.Config.Validation.Enabled {
		return Stage1ValidationResult{Success: true, Diff: diff}, nil
	}

	fullCommands := opts.Config.Validation.FullCommandsOrDefault()
	failure, err := runValidationCommands(ctx, deps.CmdRunner, fullCommands, opts.WorkDir, "full validation")
	if err != nil {
		return Stage1ValidationResult{}, err
	}
	if failure != nil {
		return Stage1ValidationResult{Diff: diff, Failures: []specgate.CriterionResult{*failure}}, nil
	}

	if opts.ATDDActive {
		acceptance := methodology.AcceptanceCommands(fullCommands, opts.TouchedPackages)
		failure, err := runValidationCommands(ctx, deps.CmdRunner, acceptance, opts.WorkDir, "ATDD acceptance")
		if err != nil {
			return Stage1ValidationResult{}, err
		}
		if failure != nil {
			return Stage1ValidationResult{Diff: diff, Failures: []specgate.CriterionResult{*failure}}, nil
		}
	}

	return Stage1ValidationResult{Success: true, Diff: diff}, nil
}

func runValidationCommands(ctx context.Context, runner runtypes.CmdRunnerFn, commands []string, workDir, stageLabel string) (*specgate.CriterionResult, error) {
	if len(commands) == 0 {
		return nil, nil
	}

	for _, command := range commands {
		stdout, stderr, exitCode, err := runner(ctx, command, workDir)
		if err != nil {
			return nil, fmt.Errorf("run validation command %q: %w", command, err)
		}
		if exitCode != 0 {
			return &specgate.CriterionResult{
				Criterion: fmt.Sprintf("%s command failed: %s", stageLabel, command),
				Passed:    false,
				Evidence:  formatCommandEvidence(command, exitCode, stdout, stderr),
			}, nil
		}
	}

	return nil, nil
}

func resolveDiff(ctx context.Context, getDiff func(ctx context.Context) (string, error)) (string, error) {
	if getDiff == nil {
		return "", nil
	}
	return getDiff(ctx)
}

func formatCommandEvidence(command string, exitCode int, stdout, stderr string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Command failed: %s (exit code %d)", command, exitCode)

	if trimmed := strings.TrimSpace(stdout); trimmed != "" {
		b.WriteString("\nStdout:\n")
		b.WriteString(trimmed)
	}
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		b.WriteString("\nStderr:\n")
		b.WriteString(trimmed)
	}

	return b.String()
}

func (p *Pipeline) captureCycleRecord(ctx context.Context, specName string) {
	if p == nil || p.emitter == nil {
		return
	}
	specID := strings.TrimSpace(specName)
	if specID == "" {
		return
	}
	record := CycleRecord{
		SpecID:              specID,
		CycleEndPresentedAt: time.Now(),
	}
	if err := p.emitter.CaptureCycleRecord(ctx, record); err != nil {
		p.logCaptureCycleRecordError(specID, err)
	}
}

func (p *Pipeline) logCaptureCycleRecordError(specID string, err error) {
	if err == nil || p == nil || p.logEmitter == nil {
		return
	}
	if !p.logEmitter.HasSubscribers() {
		return
	}
	logger := &events.EmitterLogger{Emitter: p.logEmitter}
	logger.Log("warning", "captureCycleRecord failed for spec %s: %v", specID, err)
}

func (p *Pipeline) incrementAttempt(specName string) int {
	p.attemptsMu.Lock()
	defer p.attemptsMu.Unlock()
	if p.attempts == nil {
		p.attempts = make(map[string]int)
	}
	p.attempts[specName]++
	return p.attempts[specName]
}
