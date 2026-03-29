package specloop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/next/executor"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// isBuildCheck returns true if cmd is a build or compilation check (go build,
// go vet, npm run build, cargo build, mvn compile, make build). These are
// treated as harder evidence than pattern-matching checks (grep, awk, sed).
func isBuildCheck(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false
	}
	// Match build commands by their base invocation regardless of arguments
	switch {
	case fields[0] == "go" && len(fields) >= 2 && (fields[1] == "build" || fields[1] == "vet"):
		return true
	case fields[0] == "cargo" && len(fields) >= 2 && fields[1] == "build":
		return true
	case fields[0] == "mvn" && len(fields) >= 2 && fields[1] == "compile":
		return true
	case fields[0] == "make" && len(fields) >= 2 && fields[1] == "build":
		return true
	case len(fields) >= 3 && fields[0] == "npm" && fields[1] == "run" && fields[2] == "build":
		return true
	}
	return false
}

// annotateSuspectProofChecks annotates failure messages with [suspect-proof-check]
// when the task has at least one build check that is NOT failing, but all failing
// checks are pattern-matching (grep, awk, etc.). This signals to the fix planner
// that the implementation may be correct and proof checks may need to be rewritten
// to be more behavioral rather than re-implementing already-correct code.
//
// When checkResults is non-nil and non-empty, exact command identity is used to
// determine build-check pass/fail status, eliminating false positives from
// failure messages that incidentally contain build command text. When checkResults
// is nil, the function falls back to the legacy strings.Contains heuristic for
// backward compatibility with callers that do not populate the field.
func annotateSuspectProofChecks(proofChecks []string, failures []string, checkResults []ProofCheckResult) []string {
	if len(proofChecks) == 0 || len(failures) == 0 {
		return failures
	}
	buildChecks := make([]string, 0, len(proofChecks))
	for _, pc := range proofChecks {
		if isBuildCheck(pc) {
			buildChecks = append(buildChecks, pc)
		}
	}
	if len(buildChecks) == 0 {
		return failures
	}
	// If any build check command failed, this is not a suspect-proof-check scenario.
	if checkResults != nil {
		// Structural path: use exact command identity from ProofCheckResults.
		for _, cr := range checkResults {
			if !cr.Pass && isBuildCheck(cr.Command) {
				return failures
			}
		}
	} else {
		// Legacy fallback: substring match against failure messages.
		for _, failureMsg := range failures {
			for _, proofCheckCmd := range buildChecks {
				if strings.Contains(failureMsg, proofCheckCmd) {
					return failures
				}
			}
		}
	}
	// Build checks all pass, only pattern-matching checks are failing
	const prefix = "[suspect-proof-check] All build checks pass but pattern-matching checks failed. The implementation may be correct; proof checks may be testing source structure rather than behavior. "
	annotated := make([]string, len(failures))
	for i, f := range failures {
		annotated[i] = prefix + f
	}
	return annotated
}

// TaskRunner executes a single task or repairs it after failure.
type TaskRunner interface {
	RunTask(ctx context.Context, task runstore.Task) (TaskResult, error)
	RepairTask(ctx context.Context, task runstore.Task, failures []string) (TaskResult, error)
}

// TaskInspector validates a task after execution.
type TaskInspector interface {
	Inspect(ctx context.Context, task runstore.Task) InspectResult
	SetKnownGaps(gaps string)
}

// TaskDecomposer splits a task into smaller sub-tasks.
type TaskDecomposer interface {
	Decompose(ctx context.Context, task runstore.Task) ([]runstore.Task, error)
}

// GitOps abstracts git operations needed by the task loop.
type GitOps interface {
	CheckoutFiles(workDir string, files []string) error
}

// ProofCheckResult records the pass/fail result of a single proof check command.
type ProofCheckResult struct {
	Command string
	Pass    bool
}

// InspectResult is the outcome of task inspection.
type InspectResult struct {
	Pass              bool
	Failures          []string
	ProofCheckResults []ProofCheckResult // per-check structural results; may be nil for legacy callers
}

// CheckSummary tracks pass/fail counts for a set of checks.
type CheckSummary struct {
	Pass int
	Fail int
}

// TaskResult is the outcome of running or repairing a task.
type TaskResult struct {
	TaskID          string // set by RunTaskLoop to the originating task's ID
	Status          string // done, failed, needs_split
	Attempts        int
	TokensUsed      int
	Cost            float64
	DurationMs      int64
	FilesChanged    []string
	Failures        []string // annotated failure messages when Status == "failed"
	Model           string
	Tier            string
	TargetedChecks  CheckSummary
	AlwaysRunChecks CheckSummary
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices to empty values for JSON consistency.
func (tr *TaskResult) NormalizeNilFields() {
	if tr.FilesChanged == nil {
		tr.FilesChanged = []string{}
	}
	if tr.Failures == nil {
		tr.Failures = []string{}
	}
}

// FilesChangedFunc detects files changed in a working directory.
// It returns the list of changed file paths (relative to workDir), or an error.
// Implementations should handle non-git directories gracefully (return empty list, no error).
type FilesChangedFunc func(workDir string) ([]string, error)

// TaskLoopConfig configures the task loop.
type TaskLoopConfig struct {
	MaxRetries             int
	Inspector              TaskInspector
	MaxRedecompositions    int
	Decomposer             TaskDecomposer
	GitOps                 GitOps
	Budget                 *Budget
	WorkDir                string
	MaxTaskDurationSeconds int
	EventLog               *runstore.EventLog
	Cycle                  int
	DetectFilesChanged     FilesChangedFunc // optional; if nil, FilesChanged stays as returned by runner
}

// RunTaskLoop executes all tasks, with inspection and retry support.
func RunTaskLoop(ctx context.Context, tasks []runstore.Task, runner TaskRunner, cfg TaskLoopConfig) ([]TaskResult, error) {
	var results []TaskResult

	decompositionsUsed := 0

	queue := make([]taskEntry, len(tasks))
	for i, t := range tasks {
		queue[i] = taskEntry{task: t, canDecompose: true}
	}

	for i := 0; i < len(queue); i++ {
		entry := queue[i]

		// Check budget before each task
		if cfg.Budget != nil && cfg.Budget.HardBudgetExceeded() {
			results = append(results, TaskResult{TaskID: entry.task.TaskID, Status: "blocked", Attempts: 0})
			continue
		}

		// Emit task_started
		emitTaskEvent(cfg.EventLog, runstore.TaskStartedEvent{
			BaseEvent: runstore.BaseEvent{Type: "task_started", Timestamp: time.Now()},
			TaskID:    entry.task.TaskID,
			Cycle:     cfg.Cycle,
			TaskIndex: i + 1,
			TaskTotal: len(queue),
			Objective: entry.task.Objective,
		})

		// Create per-task context with timeout if configured
		taskCtx := ctx
		var taskCancel context.CancelFunc
		if cfg.MaxTaskDurationSeconds > 0 {
			taskCtx, taskCancel = context.WithTimeout(ctx, time.Duration(cfg.MaxTaskDurationSeconds)*time.Second)
		}

		if cfg.DetectFilesChanged != nil && cfg.WorkDir != "" {
			// First call: stateful closure captures baseline; return value discarded.
			cfg.DetectFilesChanged(cfg.WorkDir) //nolint:errcheck
		}

		result, err := runner.RunTask(taskCtx, entry.task)
		if taskCancel != nil {
			taskCancel()
		}
		// Track cumulative cost across all attempts for accurate reporting
		cumulativeCost := result.Cost
		// Update budget with cost from this invocation
		if cfg.Budget != nil {
			cfg.Budget.AddCost(result.Cost)
		}
		if err != nil {
			// Remove any failure metadata when the runner itself errored; it will be
			// surfaced through the generic failure handling instead of reusing
			// previously captured proof-check data.
			result.Failures = nil
			// Drain the stateful detector so the next task starts with a fresh baseline.
			// Capture the result so the planner can see what was written before the error.
			if cfg.DetectFilesChanged != nil && cfg.WorkDir != "" {
				if changed, dErr := cfg.DetectFilesChanged(cfg.WorkDir); dErr == nil {
					result.FilesChanged = changed
				}
			}
			result.TaskID = entry.task.TaskID
			result.Status = "failed"
			result.Attempts = 1
			emitTaskEvent(cfg.EventLog, runstore.TaskFailedEvent{
				BaseEvent: runstore.BaseEvent{Type: "task_failed", Timestamp: time.Now()},
				TaskID:    entry.task.TaskID,
				Reason:    err.Error(),
			})
			emitPhantomTaskFailureIfNeeded(cfg.EventLog, cfg.WorkDir, entry.task, result.FilesChanged)
			results = append(results, result)
			continue
		}
		result.TaskID = entry.task.TaskID
		attempts := 1

		// Detect files changed (second call: delta from baseline captured before RunTask).
		// Populate result.FilesChanged early so the needs_split handler can revert them.
		if cfg.DetectFilesChanged != nil && cfg.WorkDir != "" {
			if changed, dErr := cfg.DetectFilesChanged(cfg.WorkDir); dErr == nil {
				result.FilesChanged = changed
			}
		}

		// Promote "done" to "needs_split" if the file change heuristic fires.
		if result.Status == "done" {
			if executor.NeedsSplit(result.FilesChanged, entry.task.ExpectedTouchedArea) {
				result.Status = "needs_split"
			}
		}

		// Handle needs_split
		if result.Status == "needs_split" {
			emitTaskEvent(cfg.EventLog, runstore.TaskNeedsSplitEvent{
				BaseEvent: runstore.BaseEvent{Type: "task_needs_split", Timestamp: time.Now()},
				TaskID:    entry.task.TaskID,
			})

			if entry.canDecompose && cfg.Decomposer != nil && decompositionsUsed < cfg.MaxRedecompositions {
				// Revert touched files
				if cfg.GitOps != nil && len(result.FilesChanged) > 0 {
					cfg.GitOps.CheckoutFiles(cfg.WorkDir, result.FilesChanged)
				}
				subTasks, dErr := cfg.Decomposer.Decompose(ctx, entry.task)
				if dErr == nil {
					// Validate sub-tasks before enqueuing
					if validationErr := validateSubTasks(subTasks); validationErr != nil {
						emitTaskEvent(cfg.EventLog, runstore.DecompositionRejectedEvent{
							BaseEvent:       runstore.BaseEvent{Type: "decomposition_rejected", Timestamp: time.Now()},
							ParentTaskID:    entry.task.TaskID,
							RejectionReason: validationErr.Error(),
						})
						// Decomposition rejected, treat as failed
						result.Status = "failed"
						result.Failures = []string{validationErr.Error()}
					} else {
						decompositionsUsed++
						emitTaskEvent(cfg.EventLog, runstore.RedecompositionTriggeredEvent{
							BaseEvent: runstore.BaseEvent{Type: "redecomposition_triggered", Timestamp: time.Now()},
							Reason:    "task " + entry.task.TaskID + " needs split",
						})
						// Renumber sub-tasks to avoid ID collisions with tasks already in
						// the queue. IDs continue from the current maximum task ID.
						maxID := maxTaskIDInQueue(queue)
						subTasks = renumberSubTasks(subTasks, maxID+1)
						for i := range subTasks {
							// Inherit SpecConstraints from parent if the decomposer didn't set it.
							if subTasks[i].SpecConstraints == "" {
								subTasks[i].SpecConstraints = entry.task.SpecConstraints
							}
							queue = append(queue, taskEntry{task: subTasks[i], canDecompose: false})
						}
						// Add the parent to results as "decomposed" so execute.go can update
						// rs.Tasks and prevent it from being re-queued in the next cycle.
						results = append(results, TaskResult{
							TaskID:   entry.task.TaskID,
							Status:   "decomposed",
							Attempts: attempts,
						})
						continue
					}
				}
			}
			// Decomposition not possible or failed — treat as failed
			result.Status = "failed"
		}

		// Inspect
		if cfg.Inspector != nil && result.Status == "done" {
			ir := cfg.Inspector.Inspect(ctx, entry.task)
			// Structural safety-net: if the inspector passed, verify every *_test.go
			// listed in expected_touched_area was actually changed. This catches cases
			// where the planner forgot to include a content-verification proof check
			// for a test file.
			// Structural safety-net only applies when the agent actually changed
			// file contents. If FilesChanged is empty (e.g. git-only operation like
			// staging untracked files), there is nothing to enforce — skip the check.
			if ir.Pass && len(result.FilesChanged) > 0 {
				for _, expected := range entry.task.ExpectedTouchedArea {
					if strings.HasSuffix(expected, "_test.go") {
						found := false
						for _, changed := range result.FilesChanged {
							if changed == expected {
								found = true
								break
							}
						}
						if !found {
							ir.Pass = false
							ir.Failures = append(ir.Failures,
								fmt.Sprintf("expected to modify %s but it was not changed", expected))
						}
					}
				}
			}
			emitTaskEvent(cfg.EventLog, runstore.TaskValidationResultEvent{
				BaseEvent: runstore.BaseEvent{Type: "task_validation_result", Timestamp: time.Now()},
				TaskID:    entry.task.TaskID,
				Passed:    ir.Pass,
			})
			if !ir.Pass {
				// Retry loop
				for retry := 0; retry < cfg.MaxRetries; retry++ {
					repairCtx := ctx
					var repairCancel context.CancelFunc
					if cfg.MaxTaskDurationSeconds > 0 {
						repairCtx, repairCancel = context.WithTimeout(ctx, time.Duration(cfg.MaxTaskDurationSeconds)*time.Second)
					}
					repairResult, rErr := runner.RepairTask(repairCtx, entry.task, ir.Failures)
					repairResult.Failures = nil
					if repairCancel != nil {
						repairCancel()
					}
					// Update budget and cumulative cost from repair invocation
					cumulativeCost += repairResult.Cost
					if cfg.Budget != nil {
						cfg.Budget.AddCost(repairResult.Cost)
					}
					attempts++
					if rErr != nil {
						result.Status = "failed"
						break
					}
					result = repairResult
					result.TaskID = entry.task.TaskID
					ir = cfg.Inspector.Inspect(ctx, entry.task)
					// Apply structural test-file coverage check after repair inspection too.
					// Structural safety-net only applies when the agent actually changed
					// file contents. Skip when FilesChanged is empty (e.g. git-only ops).
					if ir.Pass && len(result.FilesChanged) > 0 {
						for _, expected := range entry.task.ExpectedTouchedArea {
							if strings.HasSuffix(expected, "_test.go") {
								found := false
								for _, changed := range result.FilesChanged {
									if changed == expected {
										found = true
										break
									}
								}
								if !found {
									ir.Pass = false
									ir.Failures = append(ir.Failures,
										fmt.Sprintf("expected to modify %s but it was not changed", expected))
								}
							}
						}
					}
					emitTaskEvent(cfg.EventLog, runstore.TaskValidationResultEvent{
						BaseEvent: runstore.BaseEvent{Type: "task_validation_result", Timestamp: time.Now()},
						TaskID:    entry.task.TaskID,
						Passed:    ir.Pass,
					})
					if ir.Pass {
						break
					}
				}
				// If inspection still fails after all retries, mark as failed.
				// Annotate failures with [suspect-proof-check] if only pattern-matching
				// checks are failing while build checks all pass.
				if !ir.Pass {
					ir.Failures = annotateSuspectProofChecks(entry.task.ProofChecks, ir.Failures, ir.ProofCheckResults)
					result.Failures = ir.Failures
					result.Status = "failed"
				}
			}
		}

		result.Attempts = attempts
		result.Cost = cumulativeCost

		// Emit task_completed or task_failed
		if result.Status == "done" {
			emitTaskEvent(cfg.EventLog, runstore.TaskCompletedEvent{
				BaseEvent:  runstore.BaseEvent{Type: "task_completed", Timestamp: time.Now()},
				TaskID:     entry.task.TaskID,
				TokensUsed: result.TokensUsed,
				DurationMs: result.DurationMs,
			})
		} else if result.Status == "failed" {
			emitTaskEvent(cfg.EventLog, runstore.TaskFailedEvent{
				BaseEvent: runstore.BaseEvent{Type: "task_failed", Timestamp: time.Now()},
				TaskID:    entry.task.TaskID,
				Reason:    "task execution failed",
			})
			emitPhantomTaskFailureIfNeeded(cfg.EventLog, cfg.WorkDir, entry.task, result.FilesChanged)
		}

		results = append(results, result)
	}

	return results, nil
}

// emitPhantomTaskFailureIfNeeded emits a phantom_task_failure event when a task
// failed without writing any files and at least one expected deliverable is missing
// from disk. This helps the planner distinguish "code that needs fixing" from
// "file that was never written".
func emitPhantomTaskFailureIfNeeded(el *runstore.EventLog, workDir string, task runstore.Task, filesChanged []string) {
	if len(filesChanged) > 0 || workDir == "" {
		return
	}
	var missing []string
	for _, path := range task.ExpectedTouchedArea {
		fullPath := filepath.Join(workDir, path)
		if _, statErr := os.Stat(fullPath); os.IsNotExist(statErr) {
			missing = append(missing, path)
		}
	}
	if len(missing) > 0 {
		emitTaskEvent(el, runstore.PhantomTaskFailureEvent{
			BaseEvent:    runstore.BaseEvent{Type: "phantom_task_failure", Timestamp: time.Now()},
			TaskID:       task.TaskID,
			MissingFiles: missing,
		})
	}
}

// emitTaskEvent appends an event to the log if the log is non-nil.
func emitTaskEvent(el *runstore.EventLog, ev runstore.TypedEvent) {
	if el != nil {
		el.Append(ev)
	}
}

// taskEntry tracks a task in the queue with metadata about decomposition eligibility.
type taskEntry struct {
	task         runstore.Task
	canDecompose bool
}

// maxTaskIDInQueue scans queue for the highest numeric suffix in task IDs
// formatted as "t-NNN" and returns that maximum value. Returns 0 if no
// such IDs are found.
func maxTaskIDInQueue(queue []taskEntry) int {
	max := 0
	for _, e := range queue {
		id := e.task.TaskID
		if !strings.HasPrefix(id, "t-") {
			continue
		}
		n, err := strconv.Atoi(id[2:])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max
}

// renumberSubTasks renumbers sub-tasks so their IDs continue from startAt,
// incrementing by 1 for each sub-task. IDs are formatted as "t-NNN" with
// zero-padding to at least 3 digits.
func renumberSubTasks(subTasks []runstore.Task, startAt int) []runstore.Task {
	result := make([]runstore.Task, len(subTasks))
	for i, st := range subTasks {
		st.TaskID = fmt.Sprintf("t-%03d", startAt+i)
		result[i] = st
	}
	return result
}

// validateSubTasks returns an error if any sub-task has an empty or whitespace-only
// Objective field. If valid, returns nil. If invalid, the error message includes
// the task IDs of all offending sub-tasks.
func validateSubTasks(subTasks []runstore.Task) error {
	var invalid []string
	for _, st := range subTasks {
		if strings.TrimSpace(st.Objective) == "" {
			invalid = append(invalid, st.TaskID)
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf("sub-tasks with empty objectives: %s", strings.Join(invalid, ", "))
	}
	return nil
}
