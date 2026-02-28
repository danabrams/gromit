package specmerge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/methodology"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/specgate"
)

const specLabelPrefix = "spec:"

// beadQuery defines the subset of the bead client needed by Pipeline.
type beadQuery interface {
	ListWithLabel(label string) ([]*bead.Bead, error)
}

// Controller coordinates spec merge completion triggers.
type Controller interface {
	IsSpecComplete(specName string) (bool, error)
	Trigger(ctx context.Context, specName string) error
}

// Pipeline runs spec merge orchestration helpers.
type Pipeline struct {
	query   beadQuery
	emitter CycleRecordEmitter
}

// NewPipeline constructs a Pipeline with the provided bead query dependencies.
func NewPipeline(query beadQuery, emitter CycleRecordEmitter) *Pipeline {
	if emitter == nil {
		emitter = NoopCycleRecordEmitter()
	}
	return &Pipeline{
		query:   query,
		emitter: emitter,
	}
}

// IsSpecComplete returns true when no open beads remain for the given spec.
func (p *Pipeline) IsSpecComplete(specName string) (bool, error) {
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
	beads, err := p.query.ListWithLabel(label)
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
	_ = p.emitter.CaptureCycleRecord(ctx, record)
}
