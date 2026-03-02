// Package runner provides the Gromit loop orchestrator.
package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"os/exec"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/events/cli"
	"github.com/danabrams/gromit/internal/events/stream"
	"github.com/danabrams/gromit/internal/experiment"
	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/procutil"
	"github.com/danabrams/gromit/internal/runner/specmerge"
	"github.com/danabrams/gromit/internal/specflow"
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
	// LocalGate runs spec-only acceptance checks after implementation succeeds.
	LocalGate pipeline.Stage
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

	// SpecMergeController triggers the merge pipeline when spec work completes.
	SpecMergeController specmerge.Controller

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
	// AutoTriageService runs SPC auto-triage after a run completes.
	AutoTriageService SPCAutoTriageService
	// CoverageTracker tracks acceptance criteria coverage across the TDD cycle.
	// Optional: nil means skip tracker state transitions.
	CoverageTracker *coverage.CoverageTracker

	// ExperimentMgr manages experiments and variant selection.
	// Optional: nil means experiments are disabled.
	ExperimentMgr *experiment.Manager

	// BranchRouter maps bead labels to git branch names.
	// Optional: nil means branch checkout is skipped.
	BranchRouter BranchRouter

	// StageContext carries specflow metadata for spec-scoped runs, if any.
	StageContext *StageContext

	// PreImplementationHook orchestrates acceptance-test authoring beads before implementation.
	PreImplementationHook func(ctx context.Context) error

	// GitCheckout performs git branch checkout operations.
	// Optional: nil means branch checkout is skipped.
	GitCheckout GitCheckout

	// Coordinator performs integration of queued session branches into main between iterations.
	// Optional: nil means coordinator is disabled.
	Coordinator Coordinator
}

type trendUpdaterCloser interface {
	Close()
}

var orchestratorNowFn = time.Now

// orchestratorWaitForProcessCapacityFn is the pre-Build process capacity check.
// It is a package-level variable so tests can inject a failing stub.
var orchestratorWaitForProcessCapacityFn = procutil.WaitForProcessCapacity

// orchestratorPreBuildCapacityWait is the timeout for the pre-Build capacity check.
const orchestratorPreBuildCapacityWait = 3 * time.Second

// orchestratorLookPathFn wraps exec.LookPath for testability.
var orchestratorLookPathFn = exec.LookPath

// orchestratorPrelaunchBackoffFn is called after a pre-launch failure to prevent
// tight retry loops. It is a package-level variable so tests can inject a no-op.
var orchestratorPrelaunchBackoffFn = func(d time.Duration) { time.Sleep(d) }

// orchestratorPrelaunchBackoffDuration is the sleep duration after a pre-launch failure.
const orchestratorPrelaunchBackoffDuration = 3 * time.Second

// StateSaver persists provider routing state (provider counts, availability) to disk.
type StateSaver interface {
	Save() error
}

// Coordinator performs integration of queued session branches into main.
// It is invoked between iterations in the run loop to process ready branches,
// and during startup to recover from crashes.
type Coordinator interface {
	// Coordinate processes the integration queue, attempting to integrate ready branches into main.
	// It should not error out on failures from individual integrations; errors in one branch
	// should be isolated and logged, allowing the run loop to continue.
	Coordinate(ctx context.Context) error
	// RecoverFromCrash detects entries left in integrating state by a prior crash
	// and transitions them back to a recoverable state (e.g., ready).
	RecoverFromCrash(ctx context.Context) error
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
	cfg                      OrchestratorConfig
	emitter                  *events.Emitter
	preImplementationHookRan bool
}

// skipTracker records how many processed beads have been skipped since the
// last discovery of a new bead. It snapshots the count of known beads so
// repeated interleaved skips eventually trigger the exit condition.
type skipTracker struct {
	processed map[string]bool
	skipCount int
	target    int
}

func newSkipTracker() *skipTracker {
	return &skipTracker{processed: make(map[string]bool)}
}

func (s *skipTracker) hasProcessed(id string) bool {
	return s.processed[id]
}

func (s *skipTracker) markProcessed(id string) {
	s.processed[id] = true
	s.skipCount = 0
	s.target = len(s.processed)
}

func (s *skipTracker) recordSkip(_ string) bool {
	if s.target == 0 {
		return false
	}
	s.skipCount++
	return s.skipCount >= s.target
}

func (s *skipTracker) processedCount() int {
	return len(s.processed)
}

func (s *skipTracker) registerBead(id string) (skip bool, stop bool) {
	if s.hasProcessed(id) {
		return true, s.recordSkip(id)
	}
	s.markProcessed(id)
	return false, false
}

// NewOrchestrator returns an Orchestrator wired with the given configuration.
func NewOrchestrator(cfg OrchestratorConfig) *Orchestrator {
	o := &Orchestrator{
		cfg:     cfg,
		emitter: events.NewEmitter(),
	}
	o.attachEmitterToStage(cfg.Gate)
	o.attachEmitterToStage(cfg.Build)
	o.attachEmitterToStage(cfg.Validate)
	o.attachEmitterToStage(cfg.LocalGate)
	o.attachEmitterToStage(cfg.Review)
	o.attachEmitterToStage(cfg.Epilogue)
	return o
}

func (o *Orchestrator) attachEmitterToStage(stage pipeline.Stage) {
	if stage == nil {
		return
	}
	type emitterSetter interface {
		SetEmitter(*events.Emitter)
	}
	if setter, ok := stage.(emitterSetter); ok {
		setter.SetEmitter(o.emitter)
	}
}

// GetEmitter returns the Emitter for this Orchestrator.
func (o *Orchestrator) GetEmitter() *events.Emitter {
	return o.emitter
}

// StartSubscribers registers and starts all subscribers (CLI always, status/tmux conditionally).
// It starts subscriber goroutines that consume events until the context is cancelled or the
// emitter is closed. This should be called before Run() to ensure subscribers are active.
// The returned WaitGroup completes when all subscriber goroutines have exited.
func (o *Orchestrator) StartSubscribers(ctx context.Context) (*sync.WaitGroup, error) {
	var wg sync.WaitGroup

	// CLI subscriber is always started
	output := o.cfg.Output
	if output == nil {
		output = os.Stderr
	}

	cliSubscriber := cli.NewCLISubscriber(cli.BasicWriter(output), o.emitter)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = cliSubscriber.Start(ctx)
	}()

	if o.cfg.LogsDir != "" {
		streamSubscriber, err := stream.NewFileSubscriber(o.cfg.LogsDir, o.emitter)
		if err != nil {
			o.logWarning("Warning: could not start stream subscriber: %v", err)
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = streamSubscriber.Start(ctx)
			}()
		}
	}

	return &wg, nil
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

	// Start subscribers before entering the main loop
	subscriberWg, err := o.StartSubscribers(ctx)
	if err != nil {
		return fmt.Errorf("failed to start subscribers: %w", err)
	}

	// Cleanup ordering (defers run LIFO): close emitter first to unblock
	// subscriber channel reads, then wait for subscriber goroutines to drain.
	defer func() {
		o.emitter.Close()
		subscriberWg.Wait()
	}()
	if o.cfg.TrendUpdater != nil {
		defer o.cfg.TrendUpdater.Close()
	}
	if o.cfg.StateSaver != nil {
		if setter, ok := o.cfg.StateSaver.(interface{ SetCleanExit(bool) }); ok {
			setter.SetCleanExit(false)
			if err := o.cfg.StateSaver.Save(); err != nil {
				o.logWarning("Warning: could not mark state clean_exit=false: %v", err)
			}
		}
	}

	var validationFailures []string
	var touchedPackages []string
	skipTracker := newSkipTracker()
	iteration := 0

	// Recover from any crash that may have left entries in integrating state
	if o.cfg.Coordinator != nil {
		if err := o.cfg.Coordinator.RecoverFromCrash(ctx); err != nil {
			o.logWarning("Warning: coordinator crash recovery failed: %v", err)
		}
	}

	// Emit RunStartEvent
	timeBudget := time.Duration(0)
	if !deadline.IsZero() {
		timeBudget = time.Until(deadline)
		if timeBudget < 0 {
			timeBudget = 0
		}
	}
	o.emitter.Emit(&events.RunStartEvent{
		MaxIterations: maxIterations,
		DryRun:        false,
		TimeBudget:    timeBudget,
		Time:          time.Now(),
	})

	// Check experiments for convergence and emit summary
	if o.cfg.ExperimentMgr != nil {
		experiments := o.cfg.ExperimentMgr.ListExperiments()
		for _, exp := range experiments {
			o.logInfo("Experiment %s (%s): checking if converged", exp.ID, exp.Phase)
		}
	}

	if err := o.runPreImplementationHook(ctx); err != nil {
		return err
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
		if !deadline.IsZero() && orchestratorNowFn().After(deadline) {
			break runLoop
		}

		// Check cgroup PID pressure before starting work.
		if pidCur, pidMax, pidErr := procutil.PIDPressure(); pidErr == nil && pidCur > 0 && pidMax > 0 {
			pct := pidCur * 100 / pidMax
			if pct >= 90 {
				return fmt.Errorf("cgroup PID usage at %d%% (%d/%d), stopping to prevent resource exhaustion", pct, pidCur, pidMax)
			}
			if pct >= 70 {
				o.logWarning("PID pressure at %d%% (%d/%d)", pct, pidCur, pidMax)
			}
		}

		// Get the next bead from the work queue.
		b, err := o.cfg.GetBead(ctx)
		if err != nil {
			return fmt.Errorf("orchestrator: getting next bead: %w", err)
		}
		if b == nil {
			break runLoop
		}
		if !deadline.IsZero() && orchestratorNowFn().After(deadline) {
			break runLoop
		}

		// Skip beads already processed this run. bd returns the same
		// bead repeatedly when it cannot transition to closed (e.g.
		// open dependencies, gate/build failures). Track skip events since the
		// last new bead; once we have observed as many skips as known beads,
		// every bead has been re-offered and there is no new work.
		if skip, stop := skipTracker.registerBead(b.ID); skip {
			if stop {
				o.logInfo("No remaining work: all %d processed bead(s) re-offered by bd (likely uncloseable due to open dependencies)", skipTracker.processedCount())
				break runLoop
			}
			continue
		}

		// Iteration numbers are assigned monotonically, one per bead regardless
		// of whether the bead proceeds through all stages or is blocked early.
		iteration++
		o.logInfo("Iteration %d: processing bead %s (%s)", iteration, b.ID, b.Title)

		// Emit IterationStartEvent
		o.emitter.Emit(&events.IterationStartEvent{
			Iteration: iteration,
			BeadID:    b.ID,
			BeadTitle: b.Title,
			Time:      time.Now(),
		})

		if o.cfg.StatusWriter != nil {
			o.cfg.StatusWriter(iteration, b.ID, b.Title, deadline)
		}

		baseIn := o.buildInput(b, iteration, deadline, validationFailures, touchedPackages)

		// Stage 1: Gate — precheck, stuck detection, scope gate, proactive decomposition.
		gateOut, gateErr := o.cfg.Gate.Run(ctx, baseIn)
		if gateErr != nil {
			o.logWarning("Warning: gate error for bead %s (iteration %d): %v", b.ID, iteration, gateErr)
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
				FailurePhase:             failurephase.Prelaunch,
				Complexity:               baseIn.Complexity,
				ComplexitySource:         baseIn.ComplexitySource,
				ComplexityFallbackReason: baseIn.ComplexityFallbackReason,
			}
			switch gateOut.Decision {
			case pipeline.Skip:
				o.emitter.Emit(&events.BeadSkippedEvent{
					BeadID: b.ID,
					Reason: "gate stage returned skip decision",
					Time:   time.Now(),
				})
			case pipeline.Block:
				baseIn.Result.GateBlockReason = gateOut.GateBlockReason
				if gateOut.GateBlockReason == "failure_threshold_exceeded" {
					o.emitBeadStuckEvent(b, "gate stage returned block decision")
				} else {
					reason := "gate stage returned block decision"
					if gateOut.GateBlockReason != "" {
						reason = fmt.Sprintf("%s: %s", reason, gateOut.GateBlockReason)
					}
					o.emitter.Emit(&events.BeadSkippedEvent{
						BeadID: b.ID,
						Reason: reason,
						Time:   time.Now(),
					})
				}
			default:
				o.emitter.Emit(&events.BeadSkippedEvent{
					BeadID: b.ID,
					Reason: "gate stage returned non-proceed decision",
					Time:   time.Now(),
				})
			}
			o.runEpilogue(ctx, baseIn, false)
			// Emit IterationCompleteEvent
			o.emitter.Emit(&events.IterationCompleteEvent{
				Iteration: iteration,
				BeadID:    b.ID,
				Success:   false,
				Duration:  0,
				Time:      time.Now(),
			})
			continue
		}

		// Checkout branch if router and checkout are configured.
		if o.cfg.BranchRouter != nil && o.cfg.GitCheckout != nil {
			branch, branchErr := o.cfg.BranchRouter.BranchForLabels(b.Labels)
			if branchErr != nil {
				o.logWarning("Warning: branch resolution failed for bead %s: %v", b.ID, branchErr)
			} else if branch != "" {
				if checkoutErr := o.cfg.GitCheckout.CreateOrCheckoutSpecBranch(ctx, branch); checkoutErr != nil {
					o.logWarning("Branch checkout failed for %s: %v; marking bead %s as failed", branch, checkoutErr, b.ID)
					o.emitBeadFailedEvent(b, fmt.Sprintf("branch checkout failed for %s: %v", branch, checkoutErr))
					baseIn.Result = &logger.IterationLog{
						Timestamp:                time.Now(),
						Iteration:                iteration,
						BeadID:                   b.ID,
						BeadTitle:                b.Title,
						Success:                  false,
						Error:                    fmt.Sprintf("branch checkout failed for %s: %v", branch, checkoutErr),
						FailurePhase:             failurephase.Prelaunch,
						Complexity:               baseIn.Complexity,
						ComplexitySource:         baseIn.ComplexitySource,
						ComplexityFallbackReason: baseIn.ComplexityFallbackReason,
					}
					o.runEpilogue(ctx, baseIn, false)
					o.emitter.Emit(&events.IterationCompleteEvent{
						Iteration: iteration,
						BeadID:    b.ID,
						Success:   false,
						Duration:  0,
						Time:      time.Now(),
					})
					continue
				}
			}
		}

		// Pre-Build diagnostic: verify subprocess capacity before launching the LLM process.
		if capErr := orchestratorWaitForProcessCapacityFn(ctx, orchestratorPreBuildCapacityWait); capErr != nil {
			o.logWarning("Pre-build capacity check failed for bead %s (iteration %d): %v", b.ID, iteration, capErr)
			o.emitBeadFailedEvent(b, capErr.Error())
			baseIn.Result = &logger.IterationLog{
				Timestamp:                time.Now(),
				Iteration:                iteration,
				BeadID:                   b.ID,
				BeadTitle:                b.Title,
				Success:                  false,
				Error:                    capErr.Error(),
				FailurePhase:             failurephase.Prelaunch,
				GateBlockReason:          "process_capacity_exhausted",
				Complexity:               baseIn.Complexity,
				ComplexitySource:         baseIn.ComplexitySource,
				ComplexityFallbackReason: baseIn.ComplexityFallbackReason,
			}
			o.runEpilogue(ctx, baseIn, false)
			o.emitter.Emit(&events.IterationCompleteEvent{
				Iteration: iteration,
				BeadID:    b.ID,
				Success:   false,
				Duration:  0,
				Time:      time.Now(),
			})
			orchestratorPrelaunchBackoffFn(orchestratorPrelaunchBackoffDuration)
			continue
		}

		// Pre-Build diagnostic: verify at least one configured provider binary exists on PATH.
		if o.cfg.Config != nil && len(o.cfg.Config.Providers) > 0 {
			anyFound := false
			for _, pDef := range o.cfg.Config.Providers {
				if pDef.Binary == "" {
					continue
				}
				if _, lookErr := orchestratorLookPathFn(pDef.Binary); lookErr == nil {
					anyFound = true
					break
				}
			}
			if !anyFound {
				o.logWarning("Pre-build binary check failed for bead %s (iteration %d): no configured provider binary found on PATH", b.ID, iteration)
				o.emitBeadFailedEvent(b, "no configured provider binary found on PATH")
				baseIn.Result = &logger.IterationLog{
					Timestamp:                time.Now(),
					Iteration:                iteration,
					BeadID:                   b.ID,
					BeadTitle:                b.Title,
					Success:                  false,
					Error:                    "no configured provider binary found on PATH",
					FailurePhase:             failurephase.Prelaunch,
					GateBlockReason:          "provider_binary_missing",
					Complexity:               baseIn.Complexity,
					ComplexitySource:         baseIn.ComplexitySource,
					ComplexityFallbackReason: baseIn.ComplexityFallbackReason,
				}
				o.runEpilogue(ctx, baseIn, false)
				o.emitter.Emit(&events.IterationCompleteEvent{
					Iteration: iteration,
					BeadID:    b.ID,
					Success:   false,
					Duration:  0,
					Time:      time.Now(),
				})
				orchestratorPrelaunchBackoffFn(orchestratorPrelaunchBackoffDuration)
				continue
			}
		}

		// Stage 2: Build — selects methodology, renders prompt, invokes LLM via StreamRun.
		if o.cfg.CoverageTracker != nil {
			o.cfg.CoverageTracker.ToCollecting()
		}
		buildOut, buildErr := o.cfg.Build.Run(ctx, baseIn)
		if buildErr != nil {
			o.logWarning("Warning: build failed for bead %s (iteration %d): %v", b.ID, iteration, buildErr)
			failurePhase := inferBuildFailurePhase(buildErr)
			o.emitBeadFailedEvent(b, buildErr.Error())
			baseIn.Result = &logger.IterationLog{
				Timestamp:                time.Now(),
				Iteration:                iteration,
				BeadID:                   b.ID,
				BeadTitle:                b.Title,
				Success:                  false,
				Error:                    buildErr.Error(),
				FailurePhase:             failurePhase,
				Complexity:               baseIn.Complexity,
				ComplexitySource:         baseIn.ComplexitySource,
				ComplexityFallbackReason: baseIn.ComplexityFallbackReason,
			}
			stampBuildAttribution(baseIn.Result, buildOut)
			o.runEpilogue(ctx, baseIn, false)
			// Emit IterationCompleteEvent
			o.emitter.Emit(&events.IterationCompleteEvent{
				Iteration: iteration,
				BeadID:    b.ID,
				Success:   false,
				Duration:  0,
				Time:      time.Now(),
			})
			continue
		}

		// Stage 3: Validate — runs fast validation commands, enforces deadline.
		if o.cfg.CoverageTracker != nil {
			o.cfg.CoverageTracker.ToValidating()
		}
		validateOut, validateErr := o.cfg.Validate.Run(ctx, baseIn)
		validationPassed := validateErr == nil && validateOut.Decision == pipeline.Proceed
		if !validationPassed {
			validationFailures = append([]string(nil), validateOut.ValidationFailures...)
			maxValidationRetries := 0
			if o.cfg.Config != nil {
				maxValidationRetries = o.cfg.Config.Validation.MaxValidationRetries
			}
			for attempt := 1; attempt <= maxValidationRetries && !validationPassed; attempt++ {
				// Non-validation execution errors from Validate stage are not recoverable.
				if validateErr != nil {
					break
				}
				o.logWarning("Warning: validation failed for bead %s (iteration %d), attempting recovery build %d/%d", b.ID, iteration, attempt, maxValidationRetries)
				retryIn := baseIn
				retryIn.ValidationFailures = append([]string(nil), validationFailures...)

				retryBuildOut, retryBuildErr := o.cfg.Build.Run(ctx, retryIn)
				buildOut = retryBuildOut
				if retryBuildErr != nil {
					buildErr = retryBuildErr
					break
				}

				validateOut, validateErr = o.cfg.Validate.Run(ctx, retryIn)
				validationPassed = validateErr == nil && validateOut.Decision == pipeline.Proceed
				if !validationPassed {
					validationFailures = append([]string(nil), validateOut.ValidationFailures...)
				}
			}
		}
		if buildErr != nil {
			o.logWarning("Warning: build failed for bead %s (iteration %d): %v", b.ID, iteration, buildErr)
			failurePhase := inferBuildFailurePhase(buildErr)
			o.emitBeadFailedEvent(b, buildErr.Error())
			baseIn.Result = &logger.IterationLog{
				Timestamp:                time.Now(),
				Iteration:                iteration,
				BeadID:                   b.ID,
				BeadTitle:                b.Title,
				Success:                  false,
				Error:                    buildErr.Error(),
				FailurePhase:             failurePhase,
				Complexity:               baseIn.Complexity,
				ComplexitySource:         baseIn.ComplexitySource,
				ComplexityFallbackReason: baseIn.ComplexityFallbackReason,
			}
			stampBuildAttribution(baseIn.Result, buildOut)
			o.runEpilogue(ctx, baseIn, false)
			// Emit IterationCompleteEvent
			o.emitter.Emit(&events.IterationCompleteEvent{
				Iteration: iteration,
				BeadID:    b.ID,
				Success:   false,
				Duration:  0,
				Time:      time.Now(),
			})
			continue
		}
		if !validationPassed {
			// Accumulate failure summaries for the next Build invocation.
			validationFailures = append([]string(nil), validateOut.ValidationFailures...)
			baseIn.FailureOutput = strings.Join(validationFailures, "\n")
			baseIn.Result = &logger.IterationLog{
				Timestamp:                time.Now(),
				Iteration:                iteration,
				BeadID:                   b.ID,
				BeadTitle:                b.Title,
				Success:                  false,
				FailurePhase:             failurephase.Validation,
				ValidationFailures:       validationFailures,
				Complexity:               baseIn.Complexity,
				ComplexitySource:         baseIn.ComplexitySource,
				ComplexityFallbackReason: baseIn.ComplexityFallbackReason,
			}
			stampBuildAttribution(baseIn.Result, buildOut)
			failureReasons := make([]string, 0, len(validationFailures)+1)
			failureReasons = append(failureReasons, validationFailures...)
			if validateErr != nil {
				failureReasons = append(failureReasons, validateErr.Error())
			}
			failureMessage := strings.Join(failureReasons, "; ")
			if failureMessage == "" {
				failureMessage = "validation failed"
			}
			o.emitBeadFailedEvent(b, failureMessage)
			o.runEpilogue(ctx, baseIn, false)
			// Emit IterationCompleteEvent
			o.emitter.Emit(&events.IterationCompleteEvent{
				Iteration: iteration,
				BeadID:    b.ID,
				Success:   false,
				Duration:  0,
				Time:      time.Now(),
			})
			continue
		}

		// Validation passed: clear accumulated failures so the next bead starts clean.
		validationFailures = nil

		if o.cfg.CoverageTracker != nil {
			o.cfg.CoverageTracker.ToComplete()
		}

		if o.cfg.LocalGate != nil {
			localGateOut, localGateErr := o.cfg.LocalGate.Run(ctx, baseIn)
			if localGateErr != nil || localGateOut.Decision != pipeline.Proceed {
				failureReasons := append([]string(nil), localGateOut.ValidationFailures...)
				if localGateErr != nil {
					failureReasons = append(failureReasons, localGateErr.Error())
				}
				failureMessage := strings.Join(failureReasons, "; ")
				if failureMessage == "" {
					failureMessage = "local gate failed"
				}
				if localGateErr != nil {
					o.logWarning("Warning: local gate error for bead %s (iteration %d): %v", b.ID, iteration, localGateErr)
				} else {
					o.logWarning("Warning: local gate failed for bead %s (iteration %d)", b.ID, iteration)
				}
				baseIn.Result = &logger.IterationLog{
					Timestamp:                time.Now(),
					Iteration:                iteration,
					BeadID:                   b.ID,
					BeadTitle:                b.Title,
					Success:                  false,
					FailurePhase:             failurephase.LocalGate,
					ValidationFailures:       localGateOut.ValidationFailures,
					Error:                    failureMessage,
					Complexity:               baseIn.Complexity,
					ComplexitySource:         baseIn.ComplexitySource,
					ComplexityFallbackReason: baseIn.ComplexityFallbackReason,
				}
				baseIn.FailureOutput = strings.Join(localGateOut.ValidationFailures, "\n")
				stampBuildAttribution(baseIn.Result, buildOut)
				o.emitBeadFailedEvent(b, failureMessage)
				o.runEpilogue(ctx, baseIn, false)
				o.emitter.Emit(&events.IterationCompleteEvent{
					Iteration: iteration,
					BeadID:    b.ID,
					Success:   false,
					Duration:  time.Duration(buildOut.DurationMs) * time.Millisecond,
					Time:      time.Now(),
				})
				continue
			}
		}

		// Stage 4: Review — optional LLM code review.
		if o.cfg.Review != nil && o.cfg.Config != nil && o.cfg.Config.Review.Enabled {
			reviewOut, _ := o.cfg.Review.Run(ctx, baseIn)
			_ = reviewOut
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
			Complexity:               baseIn.Complexity,
			ComplexitySource:         baseIn.ComplexitySource,
			ComplexityFallbackReason: baseIn.ComplexityFallbackReason,
		}
		stampBuildAttribution(baseIn.Result, buildOut)
		epilogueOut := o.runEpilogue(ctx, baseIn, true)
		finalSuccess := epilogueOut.LifecycleFailure == pipeline.LifecycleFailureNone
		if baseIn.Result != nil {
			baseIn.Result.Success = finalSuccess
		}
		if epilogueOut.LifecycleFailure == pipeline.LifecycleFailureClose {
			o.logWarning("Bead %s: close failed (already marked processed, will skip on next iteration)", b.ID)
		}
		// Only log success and trigger success-only follow-on paths if lifecycle succeeded.
		if finalSuccess {
			o.logInfo("Iteration %d: bead %s completed successfully", iteration, b.ID)
			o.maybeTriggerSpecMerge(ctx, b)
		}
		// Emit IterationCompleteEvent
		o.emitter.Emit(&events.IterationCompleteEvent{
			Iteration: iteration,
			BeadID:    b.ID,
			Success:   finalSuccess,
			Duration:  time.Duration(buildOut.DurationMs) * time.Millisecond,
			Time:      time.Now(),
		})
		if finalSuccess {
			// Emit BeadCompleteEvent only for successful lifecycle iterations
			o.emitter.Emit(&events.BeadCompleteEvent{
				BeadID:    b.ID,
				BeadTitle: b.Title,
				Duration:  0, // TODO(review): thread real duration once TUI consumes this field
				Time:      time.Now(),
			})
		}
		touchedPackages = mergeTouchedPackages(touchedPackages, epilogueOut.TouchedPackages)

		// Invoke coordinator between iterations to process integration queue.
		if o.cfg.Coordinator != nil && finalSuccess {
			if err := o.cfg.Coordinator.Coordinate(ctx); err != nil {
				o.logWarning("Warning: coordinator error after iteration %d: %v", iteration, err)
			}
		}
	}

	o.logInfo("Gromit loop complete. Processed %d iterations.", iteration)

	// Merge per-run model stats into the global stats file without overwriting
	// pre-existing entries from prior runs.
	o.mergeGlobalStats()

	// Post-run completeness assertion: verify all iterations have complete efficiency data
	if err := o.assertEfficiencyCompleteness(iteration); err != nil {
		return err
	}

	// Check for control limit alerts (first-pass success regression below 70%)
	o.checkControlLimitAlerts()

	o.runAutoTriage(ctx)

	// Persist provider routing state so availability counts survive across runs.
	if o.cfg.StateSaver != nil {
		if setter, ok := o.cfg.StateSaver.(interface{ SetCleanExit(bool) }); ok {
			setter.SetCleanExit(true)
		}
		if err := o.cfg.StateSaver.Save(); err != nil {
			o.logWarning("Warning: could not save provider state: %v", err)
		}
	}

	// Emit RunCompleteEvent
	o.emitter.Emit(&events.RunCompleteEvent{
		IterationsCompleted: iteration,
		Reason:              "completed", // TODO(review): reflect actual failure reasons once TUI consumes this field
		Time:                time.Now(),
	})

	return nil
}

func (o *Orchestrator) runPreImplementationHook(ctx context.Context) error {
	if o == nil || o.cfg.StageContext == nil || o.cfg.StageContext.Stage != specflow.StageAcceptanceTests {
		return nil
	}
	if o.cfg.PreImplementationHook == nil {
		return fmt.Errorf("pre-implementation hook required for %s stage", specflow.StageAcceptanceTests)
	}
	if o.preImplementationHookRan {
		return nil
	}
	if err := o.cfg.PreImplementationHook(ctx); err != nil {
		return err
	}
	o.preImplementationHookRan = true
	stageCtx := o.cfg.StageContext
	if stageCtx.Manager != nil && stageCtx.SpecName != "" {
		if err := stageCtx.Manager.Advance(ctx, stageCtx.SpecName, specflow.StageImplementation); err != nil {
			return fmt.Errorf("advancing spec stage to implementation: %w", err)
		}
	}
	stageCtx.Stage = specflow.StageImplementation
	return nil
}

// stampBuildAttribution copies model, cost, token, duration, and tier data from the
// build stage output onto the iteration log. This shared helper ensures every exit path
// that follows a build invocation (build-fail, validation-fail, success) carries
// consistent attribution, preventing empty model/provider fields in efficiency data.
func stampBuildAttribution(log *logger.IterationLog, buildOut pipeline.Output) {
	if log == nil {
		return
	}
	log.Model = buildOut.Model
	log.CostUSD = buildOut.CostUSD
	log.InputTokens = buildOut.InputTokens
	log.OutputTokens = buildOut.OutputTokens
	log.DurationMs = buildOut.DurationMs
	log.OriginalTier = buildOut.OriginalTier
	log.ActualTier = buildOut.ActualTier
	log.CacheHit = buildOut.CacheHit
	log.CacheMiss = buildOut.CacheMiss
	log.CacheWrite = buildOut.CacheWrite
	log.CacheClass = buildOut.CacheClass
	log.CacheKey = buildOut.CacheKey
	log.CacheInvalidationReason = buildOut.CacheInvalidationReason
	log.CacheVersionMarker = buildOut.CacheVersionMarker
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

// checkControlLimitAlerts verifies that first-pass success rate stays above 80%.
// If rolling_first_pass_success_rate < 0.80 (with minimum 30 iterations in window),
// logs a warning that triggers review of recent decomposition rule changes and sets
// a flag in state to trigger retro on next run.
func (o *Orchestrator) checkControlLimitAlerts() {
	const firstPassSuccessThreshold = 0.80
	const minimumWindowSize = 30

	// Need both LogsDir and StateSaver to perform this check
	if o.cfg.LogsDir == "" || o.cfg.StateSaver == nil {
		return
	}

	// Try to read the process trend
	trend, err := logger.ReadProcessTrend(o.cfg.LogsDir)
	if err != nil {
		o.logWarning("Warning: could not read process trend for control limit check: %v", err)
		return
	}

	if trend == nil {
		return
	}

	// Check if window has enough data and first-pass success is below threshold
	if trend.LatestWindow.FirstPassSuccess < firstPassSuccessThreshold && trend.WindowSize >= minimumWindowSize {
		o.logWarning("Warning: first-pass success rate %.1f%% is below control limit of %.0f%% (window: %d iterations); review recent decomposition rule changes",
			trend.LatestWindow.FirstPassSuccess*100, firstPassSuccessThreshold*100, trend.WindowSize)

		// Set a persistent control-limit alert flag in state.
		// recordRetroState/RecordRetro clears this flag after retro completes.
		if setter, ok := o.cfg.StateSaver.(interface{ SetControlLimitAlertTriggered(bool) }); ok {
			setter.SetControlLimitAlertTriggered(true)
		}
	}
}

func (o *Orchestrator) runAutoTriage(ctx context.Context) {
	if o.cfg.AutoTriageService == nil {
		return
	}
	if err := o.cfg.AutoTriageService.EvaluateAndTriage(ctx); err != nil {
		o.logWarning("Warning: auto-triage evaluation failed: %v", err)
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
	complexity := ""
	if cfg != nil {
		complexity = cfg.SelectTier(b.Priority, b.Labels)
	}
	return pipeline.Input{
		Bead:               b,
		Config:             cfg,
		Emitter:            o.emitter,
		Iteration:          iteration,
		Deadline:           deadline,
		ValidationFailures: validationFailures,
		EscalationEnabled:  escalationEnabled,
		TouchedPackages:    touchedPackages,
		ComplexityRouting: pipeline.ComplexityRouting{
			Complexity: complexity,
		},
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
		o.logWarning("Warning: epilogue error for bead %s (iteration %d): %v", in.Bead.ID, in.Iteration, err)
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

func (o *Orchestrator) maybeTriggerSpecMerge(ctx context.Context, b *bead.Bead) {
	if o.cfg.SpecMergeController == nil || o.cfg.Config == nil {
		return
	}
	if o.cfg.Config.Methodology.Granularity != config.MethodologyGranularitySpec {
		return
	}
	specName := bead.FindSpecLabel(b.Labels)
	if specName == "" {
		return
	}

	complete, err := o.cfg.SpecMergeController.IsSpecComplete(ctx, specName)
	if err != nil {
		o.logWarning("Warning: could not check spec completion for %q: %v", specName, err)
		return
	}
	if !complete {
		return
	}
	if err := o.cfg.SpecMergeController.Trigger(ctx, specName); err != nil {
		o.logWarning("Warning: spec merge pipeline trigger for %q failed: %v", specName, err)
		return
	}
	o.logInfo("Spec %q ready for human review", specName)
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
		o.logWarning("Warning: could not read run stats for global merge: %v", err)
		return
	}
	if err := logger.UpdateGlobalStats(o.cfg.GlobalStatsPath, runStats); err != nil {
		o.logWarning("Warning: could not update global stats: %v", err)
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

	baseDiags := make([]string, 0, len(diags)+1)
	baseDiags = append(baseDiags, diags...)
	if result.ErrorMessage != "" {
		baseDiags = append(baseDiags, result.ErrorMessage)
	}
	joinDiagnostics := func(extras ...string) string {
		parts := make([]string, 0, len(baseDiags)+len(extras))
		parts = append(parts, baseDiags...)
		for _, extra := range extras {
			if extra == "" {
				continue
			}
			parts = append(parts, extra)
		}
		if len(parts) == 0 {
			return ""
		}
		return strings.Join(parts, "\n  ")
	}

	// Only fail if there were iterations recorded and data was incomplete
	if totalIterations > 0 && result.TotalIterations < totalIterations {
		missingRows := totalIterations - result.TotalIterations
		missingMsg := fmt.Sprintf("%d iteration log row(s) missing for run %s (found %d/%d entries)", missingRows, runID, result.TotalIterations, totalIterations)
		diagMsg := joinDiagnostics(missingMsg)
		if diagMsg == "" {
			diagMsg = missingMsg
		}
		return fmt.Errorf("efficiency data completeness assertion failed: %d/%d iteration logs missing\nDiagnostics:\n  %s", missingRows, totalIterations, diagMsg)
	}
	if result.TotalIterations > 0 && !result.IsComplete {
		diagMsg := joinDiagnostics()
		if diagMsg == "" {
			diagMsg = "no diagnostics available"
		}
		return fmt.Errorf("efficiency data completeness assertion failed: %d/%d iterations missing efficiency data\nDiagnostics:\n  %s", result.MissingDataCount, result.TotalIterations, diagMsg)
	}

	return nil
}

// logf writes a formatted diagnostic message to the configured output.
func (o *Orchestrator) logInfo(format string, args ...any) {
	o.emitLog("info", format, args...)
}

func (o *Orchestrator) logWarning(format string, args ...any) {
	o.emitLog("warning", format, args...)
}

func (o *Orchestrator) emitLog(level string, format string, args ...any) {
	if o.emitter != nil && o.emitter.HasSubscribers() {
		logger := &events.EmitterLogger{Emitter: o.emitter}
		logger.Log(level, format, args...)
		return
	}

	output := o.cfg.Output
	if output == nil {
		output = os.Stderr
	}
	fmt.Fprintf(output, "[%s] %s\n", level, fmt.Sprintf(format, args...))
}

func (o *Orchestrator) emitBeadFailedEvent(b *bead.Bead, errMsg string) {
	if o.emitter == nil || b == nil {
		return
	}
	o.emitter.Emit(&events.BeadFailedEvent{
		BeadID:    b.ID,
		BeadTitle: b.Title,
		Error:     errMsg,
		Time:      time.Now(),
	})
}

func (o *Orchestrator) emitBeadStuckEvent(b *bead.Bead, reason string) {
	if o.emitter == nil || b == nil {
		return
	}
	o.emitter.Emit(&events.BeadStuckEvent{
		BeadID:    b.ID,
		BeadTitle: b.Title,
		Reason:    reason,
		Time:      time.Now(),
	})
}
