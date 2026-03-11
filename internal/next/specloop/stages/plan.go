package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/internal/next/planner"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
)

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
	runDir := s.store.RunDir(rs.RunID)
	specPacket, err := os.ReadFile(filepath.Join(runDir, "spec-packet.md"))
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("read spec packet: %w", err)
	}

	isFixCycle := rs.Cycle > 1 && len(rs.ReplanContext) > 0

	var plan planner.Plan
	var validationErr error

	if isFixCycle && s.fixPlanner != nil {
		fixReq := planner.FixPlanRequest{
			Failures: rs.ReplanContext,
			Cycle:    rs.Cycle,
		}
		// Try up to 2 times (initial + 1 retry)
		for attempt := 0; attempt < 2; attempt++ {
			plan, err = s.fixPlanner.CreateFixPlan(ctx, fixReq)
			if err != nil {
				return specloop.NextAction{}, fmt.Errorf("create fix plan: %w", err)
			}

			validationErr = planner.ValidatePlan(plan)
			if validationErr == nil {
				break
			}
			fixReq.Failures = append(fixReq.Failures, validationErr.Error())
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

	// Write tasks.json
	tasksJSON, err := json.MarshalIndent(plan.Tasks, "", "  ")
	if err != nil {
		return specloop.NextAction{}, fmt.Errorf("marshal tasks: %w", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "tasks.json"), tasksJSON, 0o644); err != nil {
		return specloop.NextAction{}, fmt.Errorf("write tasks.json: %w", err)
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
			Kind:                kind,
			Cycle:               rs.Cycle,
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
