// Package runner provides the Gromit loop orchestrator.
package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

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
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/specbranch"
	"github.com/danabrams/gromit/internal/runner/specmerge"
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
	// GetBeadExcluding returns the next bead excluding IDs already processed in
	// this run. Optional: nil falls back to GetBead.
	GetBeadExcluding func(ctx context.Context, excludeIDs map[string]bool) (*bead.Bead, error)
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
	// The context is the run context, allowing cancellation signals to propagate.
	// Optional: nil means skip.
	StatusWriter func(ctx context.Context, iteration int, beadID, beadTitle string, dl time.Time)
	// StatusFinalizer is called before Run returns to finalize status output.
	// Optional: nil means skip.
	StatusFinalizer func(iteration int, runErr error)

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

	// Router provides provider selection logic for build and review stages.
	// Optional: nil means routing is skipped.
	Router *provider.Router

	// StageContext carries specflow metadata for spec-scoped runs, if any.
	StageContext *StageContext
	// SessionWorktree indicates the orchestrator is running in a session worktree context.
	// When true, non-spec beads should skip forced base-branch checkout.
	SessionWorktree bool

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
	startSubscribersFn       func(context.Context) (*sync.WaitGroup, error)
	preImplementationHookRan bool
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

func (o *Orchestrator) enableSessionBranchRouter() {
	if o == nil || !o.cfg.SessionWorktree || o.cfg.BranchRouter == nil {
		return
	}
	if enabler, ok := o.cfg.BranchRouter.(interface{ EnableSessionWorktreeMode() }); ok {
		enabler.EnableSessionWorktreeMode()
	}
}

// GetEmitter returns the Emitter for this Orchestrator.
func (o *Orchestrator) GetEmitter() *events.Emitter {
	return o.emitter
}

// Router exposes the Router configured for the Orchestrator.
func (o *Orchestrator) Router() *provider.Router {
	if o == nil {
		return nil
	}
	return o.cfg.Router
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
func (o *Orchestrator) Run(ctx context.Context, maxIterations int, deadline time.Time, stopCh <-chan struct{}) (runErr error) {
	if stopCh == nil {
		stopCh = make(chan struct{})
	}

	iteration := 0
	var subscriberWg *sync.WaitGroup
	var err error

	// Cleanup ordering (defers run LIFO): close emitter first to unblock
	// subscriber channel reads, then wait for subscriber goroutines to drain.
	defer func() {
		o.emitter.Close()
		if subscriberWg != nil {
			subscriberWg.Wait()
		}
	}()
	if o.cfg.TrendUpdater != nil {
		defer o.cfg.TrendUpdater.Close()
	}
	defer func() {
		if o.cfg.StatusFinalizer != nil {
			o.cfg.StatusFinalizer(iteration, runErr)
		}
		reason := "completed"
		if runErr != nil {
			reason = fmt.Sprintf("error: %v", runErr)
		}
		o.emitter.Emit(&events.RunCompleteEvent{
			IterationsCompleted: iteration,
			Reason:              reason,
			Time:                time.Now(),
		})
	}()

	startSubscribers := o.startSubscribersFn
	if startSubscribers == nil {
		startSubscribers = o.StartSubscribers
	}
	subscriberWg, err = startSubscribers(ctx)
	if err != nil {
		return fmt.Errorf("failed to start subscribers: %w", err)
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

		// Get the next bead from the work queue. When available, use the
		// excluding variant after the first processed bead to avoid repeatedly
		// fetching a re-offered item while other ready work exists.
		var b *bead.Bead
		var err error
		if o.cfg.GetBeadExcluding != nil && skipTracker.processedCount() > 0 {
			b, err = o.cfg.GetBeadExcluding(ctx, skipTracker.processedIDs())
		} else {
			b, err = o.cfg.GetBead(ctx)
		}
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
			o.cfg.StatusWriter(ctx, iteration, b.ID, b.Title, deadline)
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
			shouldResolveBranch := specbranch.HasSpecLabel(b.Labels)
			if o.cfg.SessionWorktree {
				o.enableSessionBranchRouter()
				shouldResolveBranch = true
			}
			if shouldResolveBranch {
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

	return nil
}
