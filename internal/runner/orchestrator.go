// Package runner provides the Gromit loop orchestrator.
package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/experiment"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
)

// OrchestratorConfig holds the wired dependencies for an Orchestrator.
// All stage fields are required except Review (optional).
type OrchestratorConfig struct {
	// Gate is Stage 1: decides whether to attempt an iteration.
	Gate pipeline.Stage
	// Build is Stage 2: authors code via LLM invocation.
	Build pipeline.Stage
	// Validate is Stage 3: runs programmatic checks.
	Validate pipeline.Stage
	// Review is Stage 4: optional LLM code review. Nil means skip.
	Review pipeline.Stage
	// Epilogue is Stage 5: bead lifecycle and cleanup.
	Epilogue pipeline.Stage

	// GetBead returns the next bead to process, or nil when the queue is empty.
	GetBead func(ctx context.Context) (*bead.Bead, error)
	// GetBeadByID resolves a bead by ID for explicit sequence execution.
	GetBeadByID func(ctx context.Context, beadID string) (*bead.Bead, error)

	// Config is the loaded gromit configuration.
	Config *config.Config

	// GlobalStatsPath is the path to the global stats JSON file (e.g. ~/.gromit/stats.json).
	// When non-empty, Run merges per-run stats into this file at completion.
	GlobalStatsPath string

	// GetRunID returns the current run's log ID for stats aggregation.
	// Optional: when nil or returning "", global stats merge is skipped.
	GetRunID func() string

	// LogsDir is the directory containing iteration log JSONL files.
	// Required when GlobalStatsPath and GetRunID are set for stats merging.
	LogsDir string

	// Output receives diagnostic log messages. Nil defaults to os.Stderr.
	Output io.Writer

	// StatusWriter is called at the start of each iteration to update status.json.
	// Optional: nil means skip.
	StatusWriter func(iteration int, beadID, beadTitle string, dl time.Time)

	// StateSaver persists provider routing state after the loop completes.
	// Optional: nil means skip. Typically backed by state.File.
	StateSaver StateSaver

	// ProviderCostDefs maps runtime provider names to their configuration,
	// enabling cost estimation from token counts when providers don't report cost.
	ProviderCostDefs map[string]config.ProviderDef

	// TrendUpdater refreshes SPC process trend metrics from iteration logs.
	// Optional: nil means skip refresh lifecycle management.
	TrendUpdater trendUpdaterCloser

	// CoverageTracker tracks acceptance criteria coverage across the TDD cycle.
	// Optional: nil means skip tracker state transitions.
	CoverageTracker *coverage.CoverageTracker

	// ExperimentMgr manages experiments and variant selection.
	// Optional: nil means experiments are disabled.
	ExperimentMgr *experiment.Manager
}

type trendUpdaterCloser interface {
	Close()
}

// StateSaver persists provider routing state (provider counts, availability) to disk.
type StateSaver interface {
	Save() error
}

// Orchestrator sequences the 5-stage pipeline (Gate → Build → Validate → Review →
// Epilogue) for each bead in the work queue.
//
// It holds no business logic beyond stage sequencing, monotonic iteration accounting,
// and global stats merging. All per-stage logic lives in internal/pipeline/<stage>/.
//
// Structural guarantee: this file imports only internal/pipeline and internal/logger;
// no stage sub-package is imported. Wire stages in constructor.go.
type Orchestrator struct {
	cfg OrchestratorConfig
}

// NewOrchestrator returns an Orchestrator wired with the given configuration.
func NewOrchestrator(cfg OrchestratorConfig) *Orchestrator {
	return &Orchestrator{cfg: cfg}
}

// Run executes the Gromit pipeline loop until the bead queue is empty, maxIterations
// is reached, the context is cancelled, or a stop signal is received via stopCh.
//
// Iteration numbers are monotonically increasing across all beads, including those
// blocked or skipped at the Gate stage. ValidationFailures from a failed Validate
// stage are accumulated and fed into the next Build stage's Input. Global stats are
// merged (not overwritten) at completion when GlobalStatsPath is configured.
func (o *Orchestrator) Run(ctx context.Context, maxIterations int, deadline time.Time, stopCh <-chan struct{}) error {
	if stopCh == nil {
		stopCh = make(chan struct{})
	}
	if o.cfg.TrendUpdater != nil {
		defer o.cfg.TrendUpdater.Close()
	}
	if o.cfg.StateSaver != nil {
		if setter, ok := o.cfg.StateSaver.(interface{ SetCleanExit(bool) }); ok {
			setter.SetCleanExit(false)
			if err := o.cfg.StateSaver.Save(); err != nil {
				o.logf("Warning: could not mark state clean_exit=false: %v", err)
			}
		}
	}

	var validationFailures []string
	var touchedPackages []string
	iteration := 0

	// Check experiments for convergence and emit summary
	if o.cfg.ExperimentMgr != nil {
		experiments := o.cfg.ExperimentMgr.ListExperiments()
		for _, exp := range experiments {
			o.logf("Experiment %s (%s): checking if converged", exp.ID, exp.Phase)
		}
	}

runLoop:
	for {
		// Check stop signals before each iteration.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-stopCh:
			break runLoop
		default:
		}

		// Check max iterations cap.
		if maxIterations > 0 && iteration >= maxIterations {
			break runLoop
		}

		// Check wall-clock deadline before starting a new iteration.
		if !deadline.IsZero() && time.Now().After(deadline) {
			break runLoop
		}

		// Get the next bead from the work queue.
		b, err := o.cfg.GetBead(ctx)
		if err != nil {
			return fmt.Errorf("orchestrator: getting next bead: %w", err)
		}
		if b == nil {
			break runLoop
		}

		// Iteration numbers are assigned monotonically, one per bead regardless
		// of whether the bead proceeds through all stages or is blocked early.
		iteration++
		o.logf("Iteration %d: processing bead %s (%s)", iteration, b.ID, b.Title)

		if o.cfg.StatusWriter != nil {
			o.cfg.StatusWriter(iteration, b.ID, b.Title, deadline)
		}

		baseIn := o.buildInput(b, iteration, deadline, validationFailures, touchedPackages)

		// Stage 1: Gate — precheck, stuck detection, scope gate, proactive decomposition.
		gateOut, gateErr := o.cfg.Gate.Run(ctx, baseIn)
		if gateErr != nil {
			o.logf("Warning: gate error for bead %s (iteration %d): %v", b.ID, iteration, gateErr)
		}
		baseIn.ComplexityRouting = gateOut.ComplexityRouting
		if gateOut.Decision != pipeline.Proceed {
			// Bead is skipped or blocked; run Epilogue in the failure path for
			// cleanup and logging (e.g. status write, iteration log).
			baseIn.Result = &logger.IterationLog{
				Timestamp:                time.Now(),
				Iteration:                iteration,
				BeadID:                   b.ID,
				BeadTitle:                b.Title,
				Success:                  false,
				Complexity:               baseIn.Complexity,
				ComplexitySource:         baseIn.ComplexitySource,
				ComplexityFallbackReason: baseIn.ComplexityFallbackReason,
			}
			o.runEpilogue(ctx, baseIn, false)
			continue
		}

		// Stage 2: Build — selects methodology, renders prompt, invokes LLM via StreamRun.
		if o.cfg.CoverageTracker != nil {
			o.cfg.CoverageTracker.ToCollecting()
		}
		buildOut, buildErr := o.cfg.Build.Run(ctx, baseIn)
		if buildErr != nil {
			o.logf("Warning: build failed for bead %s (iteration %d): %v", b.ID, iteration, buildErr)
			failurePhase := inferBuildFailurePhase(buildErr)
			baseIn.Result = &logger.IterationLog{
				Timestamp:                time.Now(),
				Iteration:                iteration,
				BeadID:                   b.ID,
				BeadTitle:                b.Title,
				Success:                  false,
				Error:                    buildErr.Error(),
				FailurePhase:             failurePhase,
				Model:                    buildOut.Model,
				CostUSD:                  buildOut.CostUSD,
				InputTokens:              buildOut.InputTokens,
				OutputTokens:             buildOut.OutputTokens,
				OriginalTier:             buildOut.OriginalTier,
				ActualTier:               buildOut.ActualTier,
				Complexity:               baseIn.Complexity,
				ComplexitySource:         baseIn.ComplexitySource,
				ComplexityFallbackReason: baseIn.ComplexityFallbackReason,
			}
			o.runEpilogue(ctx, baseIn, false)
			continue
		}

		// Stage 3: Validate — runs fast validation commands, enforces deadline.
		if o.cfg.CoverageTracker != nil {
			o.cfg.CoverageTracker.ToValidating()
		}
		validateOut, validateErr := o.cfg.Validate.Run(ctx, baseIn)
		if validateErr != nil || validateOut.Decision != pipeline.Proceed {
			// Accumulate failure summaries for the next Build invocation.
			validationFailures = validateOut.ValidationFailures
			baseIn.FailureOutput = strings.Join(validateOut.ValidationFailures, "\n")
			baseIn.Result = &logger.IterationLog{
				Timestamp:                time.Now(),
				Iteration:                iteration,
				BeadID:                   b.ID,
				BeadTitle:                b.Title,
				Success:                  false,
				ValidationFailures:       validateOut.ValidationFailures,
				Complexity:               baseIn.Complexity,
				ComplexitySource:         baseIn.ComplexitySource,
				ComplexityFallbackReason: baseIn.ComplexityFallbackReason,
			}
			o.runEpilogue(ctx, baseIn, false)
			continue
		}

		// Validation passed: clear accumulated failures so the next bead starts clean.
		validationFailures = nil

		if o.cfg.CoverageTracker != nil {
			o.cfg.CoverageTracker.ToComplete()
		}

		// Stage 4: Review — optional LLM code review.
		if o.cfg.Review != nil && o.cfg.Config != nil && o.cfg.Config.Review.Enabled {
			_, _ = o.cfg.Review.Run(ctx, baseIn)
		}

		// Stage 5: Epilogue — close bead, sync, write status, write iteration log,
		// run between-iterations command, trigger thorough review when due.
		escalated := buildOut.OriginalTier != buildOut.ActualTier
		validationRetried := len(baseIn.ValidationFailures) > 0
		baseIn.Result = &logger.IterationLog{
			Timestamp:                time.Now(),
			Iteration:                iteration,
			BeadID:                   b.ID,
			BeadTitle:                b.Title,
			Success:                  true,
			Validated:                true,
			FirstPassSuccess:         !validationRetried && !escalated,
			QualityScore:             logger.ComputeQualityScore(0, 0, validationRetried, false, escalated, 0),
			Model:                    buildOut.Model,
			OriginalTier:             buildOut.OriginalTier,
			ActualTier:               buildOut.ActualTier,
			Complexity:               baseIn.Complexity,
			ComplexitySource:         baseIn.ComplexitySource,
			ComplexityFallbackReason: baseIn.ComplexityFallbackReason,
			DurationMs:               buildOut.DurationMs,
			CostUSD:                  buildOut.CostUSD,
			InputTokens:              buildOut.InputTokens,
			OutputTokens:             buildOut.OutputTokens,
			CacheHit:                 buildOut.CacheHit,
			CacheMiss:                buildOut.CacheMiss,
			CacheWrite:               buildOut.CacheWrite,
			CacheClass:               buildOut.CacheClass,
			CacheKey:                 buildOut.CacheKey,
			CacheInvalidationReason:  buildOut.CacheInvalidationReason,
			CacheVersionMarker:       buildOut.CacheVersionMarker,
		}
		epilogueOut := o.runEpilogue(ctx, baseIn, true)
		o.logf("Iteration %d: bead %s completed successfully", iteration, b.ID)
		touchedPackages = mergeTouchedPackages(touchedPackages, epilogueOut.TouchedPackages)
	}

	o.logf("Gromit loop complete. Processed %d iterations.", iteration)

	// Merge per-run model stats into the global stats file without overwriting
	// pre-existing entries from prior runs.
	o.mergeGlobalStats()

	// Post-run completeness assertion: verify all iterations have complete efficiency data
	if err := o.assertEfficiencyCompleteness(iteration); err != nil {
		return err
	}

	// Persist provider routing state so availability counts survive across runs.
	if o.cfg.StateSaver != nil {
		if setter, ok := o.cfg.StateSaver.(interface{ SetCleanExit(bool) }); ok {
			setter.SetCleanExit(true)
		}
		if err := o.cfg.StateSaver.Save(); err != nil {
			o.logf("Warning: could not save provider state: %v", err)
		}
	}
	return nil
}

func inferBuildFailurePhase(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "red phase"):
		return "red"
	case strings.Contains(msg, "green phase"):
		return "green"
	case strings.Contains(msg, "refactor phase"):
		return "refactor"
	case strings.Contains(msg, "final validation"):
		return "final_validation"
	default:
		return "build"
	}
}

// RunSequence executes the pipeline for an explicit, caller-provided bead ID sequence.
// IDs are resolved in-order via GetBeadByID, then processed through the existing run loop.
func (o *Orchestrator) RunSequence(
	ctx context.Context,
	beadIDs []string,
	maxIterations int,
	deadline time.Time,
	stopCh <-chan struct{},
) error {
	if len(beadIDs) == 0 {
		return o.Run(ctx, maxIterations, deadline, stopCh)
	}
	if o.cfg.GetBeadByID == nil {
		return fmt.Errorf("orchestrator: GetBeadByID is not configured")
	}

	index := 0
	getFromSequence := func(seqCtx context.Context) (*bead.Bead, error) {
		if index >= len(beadIDs) {
			return nil, nil
		}
		id := beadIDs[index]
		index++
		b, err := o.cfg.GetBeadByID(seqCtx, id)
		if err != nil {
			return nil, fmt.Errorf("resolving bead %s: %w", id, err)
		}
		if b == nil {
			return nil, fmt.Errorf("bead %s not found", id)
		}
		return b, nil
	}

	cloned := *o
	cloned.cfg.GetBead = getFromSequence
	return cloned.Run(ctx, maxIterations, deadline, stopCh)
}

// buildInput constructs the pipeline.Input for a given bead and iteration.
func (o *Orchestrator) buildInput(b *bead.Bead, iteration int, deadline time.Time, validationFailures, touchedPackages []string) pipeline.Input {
	cfg := o.cfg.Config
	escalationEnabled := cfg != nil && cfg.Escalation.Enabled
	return pipeline.Input{
		Bead:               b,
		Config:             cfg,
		Iteration:          iteration,
		Deadline:           deadline,
		ValidationFailures: validationFailures,
		EscalationEnabled:  escalationEnabled,
		TouchedPackages:    touchedPackages,
	}
}

// runEpilogue calls the Epilogue stage with BuildSucceeded set accordingly.
// Returns the stage output (zero-value if Epilogue is nil).
func (o *Orchestrator) runEpilogue(ctx context.Context, in pipeline.Input, buildSucceeded bool) pipeline.Output {
	if o.cfg.Epilogue == nil {
		return pipeline.Output{}
	}
	in.BuildSucceeded = buildSucceeded
	out, err := o.cfg.Epilogue.Run(ctx, in)
	if err != nil {
		o.logf("Warning: epilogue error for bead %s (iteration %d): %v", in.Bead.ID, in.Iteration, err)
	}
	return out
}

func mergeTouchedPackages(existing, incoming []string) []string {
	if len(existing) == 0 && len(incoming) == 0 {
		return nil
	}
	combined := append([]string(nil), existing...)
	combined = append(combined, incoming...)
	return normalizeTouchedPackages(combined)
}

func normalizeTouchedPackages(touchedPackages []string) []string {
	uniqueTouched := make([]string, 0, len(touchedPackages))
	seen := make(map[string]struct{}, len(touchedPackages))

	for _, pkg := range touchedPackages {
		trimmed := strings.TrimSpace(pkg)
		normalized := strings.Trim(strings.TrimPrefix(trimmed, "./"), "/")
		if trimmed == "." || normalized == "." {
			normalized = "."
		}
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		uniqueTouched = append(uniqueTouched, normalized)
	}

	return uniqueTouched
}

// mergeGlobalStats reads the existing global stats file and merges the current
// run's per-model stats into it using an atomic write. Pre-existing entries from
// prior runs are preserved; only new data is accumulated.
func (o *Orchestrator) mergeGlobalStats() {
	if o.cfg.GlobalStatsPath == "" {
		return
	}
	var runID string
	if o.cfg.GetRunID != nil {
		runID = o.cfg.GetRunID()
	}
	if o.cfg.LogsDir == "" || runID == "" {
		return // Nothing to merge
	}
	runStats, err := logger.ReadRunModelStats(o.cfg.LogsDir, runID)
	if err != nil {
		o.logf("Warning: could not read run stats for global merge: %v", err)
		return
	}
	if err := logger.UpdateGlobalStats(o.cfg.GlobalStatsPath, runStats); err != nil {
		o.logf("Warning: could not update global stats: %v", err)
	}
}

// assertEfficiencyCompleteness verifies that all iterations have complete efficiency data.
// If iterations exist but efficiency data is incomplete, returns an error with diagnostics.
func (o *Orchestrator) assertEfficiencyCompleteness(totalIterations int) error {
	// Skip check if logs directory or run ID is not configured
	if o.cfg.LogsDir == "" {
		return nil
	}
	var runID string
	if o.cfg.GetRunID != nil {
		runID = o.cfg.GetRunID()
	}
	if runID == "" {
		return nil
	}

	// Check completeness using logger utility
	// This will check any iterations that exist in the logs, regardless of totalIterations
	result, diags := logger.AssertEfficiencyCompleteness(o.cfg.LogsDir, runID)

	// Only fail if there were iterations recorded and data was incomplete
	if result.TotalIterations > 0 && !result.IsComplete {
		diagMsg := strings.Join(diags, "\n  ")
		return fmt.Errorf("efficiency data completeness assertion failed: %d/%d iterations missing efficiency data\nDiagnostics:\n  %s", result.MissingDataCount, result.TotalIterations, diagMsg)
	}

	return nil
}

// logf writes a formatted diagnostic message to the configured output.
func (o *Orchestrator) logf(format string, args ...any) {
	w := o.cfg.Output
	if w == nil {
		w = os.Stderr
	}
	_, _ = fmt.Fprintf(w, format+"\n", args...)
}
