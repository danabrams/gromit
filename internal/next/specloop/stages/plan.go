package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

// maxTaskID returns the highest task ID from a slice of tasks in the form
// "t-NNN". If the slice is empty or no IDs can be parsed, it returns "".
func maxTaskID(tasks []runstore.Task) string {
	max := -1
	for _, t := range tasks {
		id := t.TaskID
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
	if max < 0 {
		return ""
	}
	return fmt.Sprintf("t-%03d", max)
}

// PlanCreator abstracts plan generation for testability.
type PlanCreator interface {
	CreatePlan(ctx context.Context, req planner.PlanRequest) (planner.Plan, error)
}

// FixPlanCreator abstracts fix-plan generation for testability.
type FixPlanCreator interface {
	CreateFixPlan(ctx context.Context, req planner.FixPlanRequest) (planner.Plan, error)
}

// PlanStage reads the spec packet, invokes the planner, validates the plan,
// and populates rs.Tasks on success.
type PlanStage struct {
	planner    PlanCreator
	fixPlanner FixPlanCreator
	store      *runstore.Store
	eventLog   *runstore.EventLog
}

// NewPlanStage creates a new PlanStage.
func NewPlanStage(p PlanCreator, store *runstore.Store, eventLog *runstore.EventLog) *PlanStage {
	return &PlanStage{planner: p, store: store, eventLog: eventLog}
}

// SetFixPlanner sets the fix plan creator for fix cycles.
func (s *PlanStage) SetFixPlanner(fp FixPlanCreator) {
	s.fixPlanner = fp
}

// Name returns the stage name.
func (s *PlanStage) Name() string { return "plan" }

// Run executes the plan stage.
func (s *PlanStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
	hasPendingTasks := false
	for _, t := range rs.Tasks {
		if t.Status == "pending" {
			hasPendingTasks = true
			break
		}
	}

	isFixCycle := (rs.Cycle > 1 && len(rs.ReplanContext) > 0) ||
		(rs.Resumed && len(rs.ReplanContext) > 0 && !hasPendingTasks)
	if rs.Resumed && len(rs.Tasks) > 0 && !isFixCycle && hasPendingTasks {
		// Resumed run with existing pending tasks: skip planning so execution
		// continues from where it left off without overwriting tasks.
		return specloop.NextAction{Kind: specloop.Continue}, nil
	}

	runDir := s.store.RunDir(rs.RunID)
	specPacket, err := os.ReadFile(filepath.Join(runDir, "spec-packet.md"))
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("read spec packet: %w", err)
	}

	var plan planner.Plan
	var validationErr error

	if isFixCycle && s.fixPlanner != nil {
		fixReq := planner.FixPlanRequest{
			Failures:        rs.ReplanContext,
			Cycle:           rs.Cycle,
			PriorMaxTaskID:  maxTaskID(rs.Tasks),
			SpecConstraints: rs.SpecConstraints,
			SpecPacket:      string(specPacket),
			CompletedTasks:  completedTaskSummaries(rs.Tasks),
			CurrentDiff:     worktreeDiff(rs.WorktreePath),
		}
		// Try up to 2 times (initial + 1 retry)
		allFiltered := false
		for attempt := 0; attempt < 2; attempt++ {
			plan, err = s.fixPlanner.CreateFixPlan(ctx, fixReq)
			if err != nil {
				// Fix plan generation failed (LLM couldn't produce a valid plan, or
				// API/system error). Treat as no viable fix this cycle so cycles
				// exhaust naturally → needs_human rather than hard-blocking.
				allFiltered = true
				break
			}

			// Structurally filter out tasks that would touch files forbidden by
			// spec constraints (e.g., test files when spec says "Do NOT modify
			// any existing test files"). This enforces constraints regardless of
			// whether the LLM respects them in its plan output.
			plan.Tasks = filterForbiddenFixTasks(plan.Tasks, rs.SpecConstraints)
			if len(plan.Tasks) == 0 {
				// All generated tasks were forbidden. No progress can be made
				// this cycle; let cycles exhaust naturally → needs_human.
				allFiltered = true
				break
			}

			validationErr = planner.ValidatePlan(plan)
			if validationErr == nil {
				break
			}
			fixReq.Failures = append(fixReq.Failures, validationErr.Error())
		}
		if allFiltered {
			// Skip task execution this cycle; specloop will replan until exhausted.
			return specloop.NextAction{Kind: specloop.Continue}, nil
		}
		if validationErr != nil {
			// Fix plan tasks are structurally invalid after retries — no viable fix
			// this cycle. Let cycles exhaust naturally → needs_human.
			return specloop.NextAction{Kind: specloop.Continue}, nil
		}
	} else {
		req := planner.PlanRequest{
			SpecPacket: string(specPacket),
			Cycle:      rs.Cycle,
			Failures:   rs.ReplanContext,
		}
		// Try up to 2 times (initial + 1 retry)
		for attempt := 0; attempt < 2; attempt++ {
			plan, err = s.planner.CreatePlan(ctx, req)
			if err != nil {
				return specloop.NextAction{}, fmt.Errorf("create plan: %w", err)
			}

			validationErr = planner.ValidatePlan(plan)
			if validationErr == nil {
				break
			}
			// On first failure, add validation errors to request for retry
			req.Failures = append(req.Failures, validationErr.Error())
		}
	}

	// Emit plan_validation_result event
	if s.eventLog != nil {
		s.eventLog.Append(runstore.PlanValidationResultEvent{
			BaseEvent: runstore.BaseEvent{Type: "plan_validation_result", Timestamp: time.Now()},
			Passed:    validationErr == nil,
		})
	}

	if validationErr != nil {
		return specloop.NextAction{
			Kind: specloop.Blocked,
			Context: &specloop.FailureContext{
				Failures: []string{"plan validation failed after retry: " + validationErr.Error()},
				Cycle:    rs.Cycle,
			},
		}, nil
	}

	// Write plan.md
	planMD := fmt.Sprintf("# Plan (Cycle %d)\n\n", rs.Cycle)
	for _, t := range plan.Tasks {
		planMD += fmt.Sprintf("## %s\n\n%s\n\n", t.TaskID, t.Objective)
	}
	if err := os.WriteFile(filepath.Join(runDir, "plan.md"), []byte(planMD), 0o644); err != nil {
		return specloop.NextAction{}, fmt.Errorf("write plan.md: %w", err)
	}

	// Populate rs.Tasks: on cycle 1 replace, on fix cycles append to preserve history
	newTasks := make([]runstore.Task, len(plan.Tasks))
	for i, td := range plan.Tasks {
		kind := plan.Kind
		if isFixCycle && kind == "" {
			kind = "fix"
		}
		task := runstore.Task{
			TaskID:              td.TaskID,
			Objective:           td.Objective,
			Status:              "pending",
			ExpectedTouchedArea: td.ExpectedTouchedArea,
			ProofChecks:         td.ProofChecks,
			Fixes:               td.Fixes,
			Kind:                kind,
			Cycle:               rs.Cycle,
			SpecConstraints:     rs.SpecConstraints,
		}
		if isFixCycle {
			task.ParentCycle = td.ParentCycle
			task.FailuresAddressed = td.FailuresAddressed
			if task.ParentCycle == 0 {
				task.ParentCycle = rs.Cycle - 1
			}
		}
		task.NormalizeNilFields()
		newTasks[i] = task
	}
	if isFixCycle {
		rs.Tasks = append(rs.Tasks, newTasks...)
	} else {
		rs.Tasks = newTasks
	}

	// Write tasks.json with the full accumulated task list (rs.Tasks) so that
	// the file mirrors in-memory state across all cycles, not just this cycle's
	// new tasks.
	tasksJSON, err := json.MarshalIndent(rs.Tasks, "", "  ")
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("marshal tasks: %w", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "tasks.json"), tasksJSON, 0o644); err != nil {
		return specloop.NextAction{}, fmt.Errorf("write tasks.json: %w", err)
	}

	// Emit events
	if s.eventLog != nil {
		s.eventLog.Append(runstore.PlanCreatedEvent{
			BaseEvent: runstore.BaseEvent{Type: "plan_created", Timestamp: time.Now()},
			TaskCount: len(newTasks),
		})
		for _, task := range newTasks {
			s.eventLog.Append(runstore.TaskCreatedEvent{
				BaseEvent: runstore.BaseEvent{Type: "task_created", Timestamp: time.Now()},
				TaskID:    task.TaskID,
				Objective: task.Objective,
			})
		}
	}

	return specloop.NextAction{Kind: specloop.Continue}, nil
}

// completedTaskSummaries builds CompletedTask entries from tasks that finished
// successfully (status "done" or "passed").
func completedTaskSummaries(tasks []runstore.Task) []planner.CompletedTask {
	var out []planner.CompletedTask
	for _, t := range tasks {
		if t.Status != "done" && t.Status != "passed" {
			continue
		}
		out = append(out, planner.CompletedTask{
			TaskID:            t.TaskID,
			Attempts:          t.Attempts,
			FilesChanged:      t.FilesChanged,
			ValidationOutcome: t.Status,
		})
	}
	return out
}

// worktreeDiff returns the output of `git diff HEAD` in the given directory.
// If the directory is empty or the command fails, it returns "".
func worktreeDiff(dir string) string {
	if dir == "" {
		return ""
	}
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// filterForbiddenFixTasks removes fix plan tasks whose expected_touched_area
// includes files that are prohibited by spec constraints. Currently detects
// the "Do NOT modify existing test files" constraint and removes any task
// targeting a *_test.go file. Returns the filtered slice (may be empty).
func filterForbiddenFixTasks(tasks []planner.TaskDef, specConstraints string) []planner.TaskDef {
	if specConstraints == "" || len(tasks) == 0 {
		return tasks
	}
	lower := strings.ToLower(specConstraints)
	testFilesForbidden := strings.Contains(lower, "test file") &&
		(strings.Contains(lower, "do not modify") ||
			strings.Contains(lower, "must not be modified") ||
			strings.Contains(lower, "not modify"))
	if !testFilesForbidden {
		return tasks
	}
	filtered := tasks[:0:0]
	for _, t := range tasks {
		touchesTestFile := false
		for _, area := range t.ExpectedTouchedArea {
			if strings.HasSuffix(area, "_test.go") {
				touchesTestFile = true
				break
			}
		}
		if !touchesTestFile {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
