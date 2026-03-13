package specloop

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/next/execpolicy"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// --- event-emitting fake stages ---

// eventInitStage emits run_started on success.
type eventInitStage struct {
	eventLog *runstore.EventLog
}

func (s *eventInitStage) Name() string { return "init" }
func (s *eventInitStage) Run(_ context.Context, rs *runstore.RunState) (NextAction, error) {
	if s.eventLog != nil {
		s.eventLog.Append(runstore.RunStartedEvent{
			BaseEvent: runstore.BaseEvent{Type: "run_started"},
			SpecID:    rs.SpecID,
			ProjectID: rs.ProjectID,
		})
	}
	return NextAction{Kind: Continue}, nil
}

// eventCompileStage emits spec_packet_compiled on success.
type eventCompileStage struct {
	eventLog *runstore.EventLog
}

func (s *eventCompileStage) Name() string { return "compile" }
func (s *eventCompileStage) Run(_ context.Context, _ *runstore.RunState) (NextAction, error) {
	if s.eventLog != nil {
		s.eventLog.Append(runstore.SpecPacketCompiledEvent{
			BaseEvent: runstore.BaseEvent{Type: "spec_packet_compiled"},
		})
	}
	return NextAction{Kind: Continue}, nil
}

// eventPlanStage emits plan_created, plan_validation_result, task_created.
type eventPlanStage struct {
	eventLog *runstore.EventLog
	tasks    []runstore.Task
	failFn   func() bool // if non-nil and returns true, return Blocked
}

func (s *eventPlanStage) Name() string { return "plan" }
func (s *eventPlanStage) Run(_ context.Context, rs *runstore.RunState) (NextAction, error) {
	if s.failFn != nil && s.failFn() {
		if s.eventLog != nil {
			s.eventLog.Append(runstore.PlanValidationResultEvent{
				BaseEvent: runstore.BaseEvent{Type: "plan_validation_result"},
				Passed:    false,
			})
		}
		return NextAction{Kind: Blocked, Context: &FailureContext{Failures: []string{"plan validation failed"}}}, nil
	}

	rs.Tasks = s.tasks
	if s.eventLog != nil {
		s.eventLog.Append(runstore.PlanValidationResultEvent{
			BaseEvent: runstore.BaseEvent{Type: "plan_validation_result"},
			Passed:    true,
		})
		s.eventLog.Append(runstore.PlanCreatedEvent{
			BaseEvent: runstore.BaseEvent{Type: "plan_created"},
			TaskCount: len(s.tasks),
		})
		for _, t := range s.tasks {
			s.eventLog.Append(runstore.TaskCreatedEvent{
				BaseEvent: runstore.BaseEvent{Type: "task_created"},
				TaskID:    t.TaskID,
				Objective: t.Objective,
			})
		}
	}
	return NextAction{Kind: Continue}, nil
}

// eventExecuteStage emits task_started, task_validation_result, task_completed/task_failed.
type eventExecuteStage struct {
	eventLog *runstore.EventLog
	allPass  bool
}

func (s *eventExecuteStage) Name() string { return "execute" }
func (s *eventExecuteStage) Run(_ context.Context, rs *runstore.RunState) (NextAction, error) {
	for i := range rs.Tasks {
		tid := rs.Tasks[i].TaskID
		if s.eventLog != nil {
			s.eventLog.Append(runstore.TaskStartedEvent{
				BaseEvent: runstore.BaseEvent{Type: "task_started"},
				TaskID:    tid,
				Cycle:     rs.Cycle,
			})
			s.eventLog.Append(runstore.TaskValidationResultEvent{
				BaseEvent: runstore.BaseEvent{Type: "task_validation_result"},
				TaskID:    tid,
				Passed:    s.allPass,
			})
		}
		if s.allPass {
			rs.Tasks[i].Status = "done"
			if s.eventLog != nil {
				s.eventLog.Append(runstore.TaskCompletedEvent{
					BaseEvent: runstore.BaseEvent{Type: "task_completed"},
					TaskID:    tid,
				})
			}
		} else {
			rs.Tasks[i].Status = "failed"
			if s.eventLog != nil {
				s.eventLog.Append(runstore.TaskFailedEvent{
					BaseEvent: runstore.BaseEvent{Type: "task_failed"},
					TaskID:    tid,
					Reason:    "execution failed",
				})
			}
		}
	}
	if !s.allPass {
		return NextAction{Kind: NeedsHuman, Context: &FailureContext{Failures: []string{"all tasks failed"}}}, nil
	}
	return NextAction{Kind: Continue}, nil
}

// eventValidateStage emits final_validation_result.
type eventValidateStage struct {
	eventLog *runstore.EventLog
	passFn   func() bool
}

func (s *eventValidateStage) Name() string { return "validate" }
func (s *eventValidateStage) Run(_ context.Context, rs *runstore.RunState) (NextAction, error) {
	pass := true
	if s.passFn != nil {
		pass = s.passFn()
	}
	if s.eventLog != nil {
		s.eventLog.Append(runstore.FinalValidationResultEvent{
			BaseEvent: runstore.BaseEvent{Type: "final_validation_result"},
			Passed:    pass,
		})
	}
	if pass {
		rs.FinalValidationPassed = true
		return NextAction{Kind: Continue}, nil
	}
	return NextAction{
		Kind:    ReplanFrom,
		Context: &FailureContext{Failures: []string{"validation failed"}},
	}, nil
}

// eventFinalizeStage emits terminal_state.
type eventFinalizeStage struct {
	eventLog *runstore.EventLog
}

func (s *eventFinalizeStage) Name() string { return "finalize" }
func (s *eventFinalizeStage) Run(_ context.Context, rs *runstore.RunState) (NextAction, error) {
	if rs.Status == "" || rs.Status == runstore.StatusRunning {
		allDone := true
		for _, t := range rs.Tasks {
			if t.Status != "done" {
				allDone = false
				break
			}
		}
		if allDone && rs.FinalValidationPassed {
			rs.Status = runstore.StatusReadyForReview
		} else {
			rs.Status = runstore.StatusNeedsHuman
		}
	}
	if s.eventLog != nil {
		s.eventLog.Append(runstore.TerminalStateEvent{
			BaseEvent: runstore.BaseEvent{Type: "terminal_state"},
			Status:    rs.Status,
		})
	}
	return NextAction{Kind: Continue}, nil
}

// eventEvidenceStage is a no-op evidence stage.
type eventEvidenceStage struct{}

func (s *eventEvidenceStage) Name() string { return "evidence" }
func (s *eventEvidenceStage) Run(_ context.Context, _ *runstore.RunState) (NextAction, error) {
	return NextAction{Kind: Continue}, nil
}

// --- helpers ---

func newTestEventLog(t *testing.T) (*runstore.EventLog, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	return runstore.NewEventLog(path), path
}

func readEventTypes(t *testing.T, el *runstore.EventLog) []string {
	t.Helper()
	events, err := el.ReadAll()
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	var types []string
	for _, ev := range events {
		types = append(types, ev.EventType())
	}
	return types
}

func assertEventTypes(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("event count mismatch:\n  want (%d): %v\n  got  (%d): %v", len(want), want, len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] mismatch: want %q, got %q\n  full got: %v", i, want[i], got[i], got)
		}
	}
}

func assertContainsEvent(t *testing.T, types []string, want string) {
	t.Helper()
	for _, et := range types {
		if et == want {
			return
		}
	}
	t.Fatalf("expected event %q not found in %v", want, types)
}

// eventReviewStage emits review_result on success.
type eventReviewStage struct {
	eventLog         *runstore.EventLog
	totalFindings    int
	blockingFindings int
	facetsReviewed   []string
	erroredFacets    []string
	replan           bool
}

func (s *eventReviewStage) Name() string { return "review" }
func (s *eventReviewStage) Run(_ context.Context, rs *runstore.RunState) (NextAction, error) {
	if s.eventLog != nil {
		s.eventLog.Append(runstore.ReviewResultEvent{
			BaseEvent:        runstore.BaseEvent{Type: "review_result"},
			TotalFindings:    s.totalFindings,
			BlockingFindings: s.blockingFindings,
			FacetsReviewed:   s.facetsReviewed,
			ErroredFacets:    s.erroredFacets,
		})
	}
	if s.replan {
		return NextAction{Kind: ReplanFrom, Context: &FailureContext{Failures: []string{"review findings"}}}, nil
	}
	rs.FinalReviewPassed = true
	return NextAction{Kind: Continue}, nil
}

// eventAcceptanceStage emits acceptance_result on success.
type eventAcceptanceStage struct {
	eventLog      *runstore.EventLog
	totalCriteria int
	passCount     int
	failCount     int
	unclearCount  int
	replan        bool
}

func (s *eventAcceptanceStage) Name() string { return "accept" }
func (s *eventAcceptanceStage) Run(_ context.Context, rs *runstore.RunState) (NextAction, error) {
	if s.eventLog != nil {
		s.eventLog.Append(runstore.AcceptanceResultEvent{
			BaseEvent:     runstore.BaseEvent{Type: "acceptance_result"},
			TotalCriteria: s.totalCriteria,
			PassCount:     s.passCount,
			FailCount:     s.failCount,
			UnclearCount:  s.unclearCount,
		})
	}
	if s.replan {
		return NextAction{Kind: ReplanFrom, Context: &FailureContext{Failures: []string{"acceptance failed"}}}, nil
	}
	rs.FinalAcceptancePassed = true
	return NextAction{Kind: Continue}, nil
}

// --- contract tests ---

func TestEventContract_HappyPath_All15EventTypes(t *testing.T) {
	el, _ := newTestEventLog(t)

	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "do thing 1"},
		{TaskID: "t-002", Status: "pending", Objective: "do thing 2"},
	}

	stages := []Stage{
		&eventInitStage{eventLog: el},
		&eventCompileStage{eventLog: el},
		&eventPlanStage{eventLog: el, tasks: tasks},
		&eventExecuteStage{eventLog: el, allPass: true},
		&eventValidateStage{eventLog: el, passFn: func() bool { return true }},
		&eventFinalizeStage{eventLog: el},
		&eventEvidenceStage{},
	}

	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget, EventLog: el})
	rs := runstore.NewRunState("s1", "p1")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}

	types := readEventTypes(t, el)

	// All 15 event types should appear. For the happy path with 2 tasks:
	want := []string{
		"run_started",
		"spec_packet_compiled",
		"plan_validation_result",
		"plan_created",
		"task_created",
		"task_created",
		// execute stage: task_started, task_validation_result, task_completed x2
		"task_started",
		"task_validation_result",
		"task_completed",
		"task_started",
		"task_validation_result",
		"task_completed",
		// validate
		"final_validation_result",
		// finalize
		"terminal_state",
	}
	assertEventTypes(t, types, want)

	// Verify all 15 distinct event types appear across happy + other scenarios
	// Happy path covers: run_started, spec_packet_compiled, plan_validation_result,
	// plan_created, task_created, task_started, task_validation_result,
	// task_completed, final_validation_result, terminal_state
	// (10 of 15; remaining 5 are failure-path events)
	distinctTypes := map[string]bool{}
	for _, et := range types {
		distinctTypes[et] = true
	}
	happyPathTypes := []string{
		"run_started", "spec_packet_compiled", "plan_validation_result",
		"plan_created", "task_created", "task_started",
		"task_validation_result", "task_completed",
		"final_validation_result", "terminal_state",
	}
	for _, et := range happyPathTypes {
		if !distinctTypes[et] {
			t.Fatalf("missing event type %q in happy path", et)
		}
	}
}

func TestEventContract_FailurePath_ReplanAndBudgetExhausted(t *testing.T) {
	el, _ := newTestEventLog(t)

	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "do thing 1"},
	}

	validateCalls := 0
	stages := []Stage{
		&eventInitStage{eventLog: el},
		&eventCompileStage{eventLog: el},
		&eventPlanStage{eventLog: el, tasks: tasks},
		&eventExecuteStage{eventLog: el, allPass: true},
		&eventValidateStage{eventLog: el, passFn: func() bool {
			validateCalls++
			return false // always fail validation
		}},
		&eventFinalizeStage{eventLog: el},
		&eventEvidenceStage{},
	}

	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 2, MaxRunCostUSD: 99})
	loop := NewSpecLoop(stages, SpecLoopConfig{
		Budget:      budget,
		ReplanStage: "plan",
		EventLog:    el,
	})
	rs := runstore.NewRunState("s1", "p1")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}

	types := readEventTypes(t, el)

	// Should see:
	// Cycle 1: run_started, spec_packet_compiled, plan events, execute events, final_validation_result
	// Then: replan_triggered (from SpecLoop)
	// Cycle 2: plan events, execute events, final_validation_result
	// Then: replan_triggered, budget cycle increment makes cycles=2 which exhausts budget
	// Then: terminal_state with needs_human

	assertContainsEvent(t, types, "replan_triggered")
	assertContainsEvent(t, types, "terminal_state")

	// Verify terminal_state is the last event
	lastType := types[len(types)-1]
	if lastType != "terminal_state" {
		t.Fatalf("expected last event to be terminal_state, got %q", lastType)
	}

	// Verify needs_human status
	if rs.Status != runstore.StatusNeedsHuman {
		t.Fatalf("expected needs_human, got %s", rs.Status)
	}
}

func TestEventContract_FailurePath_TaskFailed(t *testing.T) {
	el, _ := newTestEventLog(t)

	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "do thing 1"},
	}

	stages := []Stage{
		&eventInitStage{eventLog: el},
		&eventCompileStage{eventLog: el},
		&eventPlanStage{eventLog: el, tasks: tasks},
		&eventExecuteStage{eventLog: el, allPass: false}, // all tasks fail
		&eventValidateStage{eventLog: el},
		&eventFinalizeStage{eventLog: el},
		&eventEvidenceStage{},
	}

	budget2 := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget2, EventLog: el})
	rs := runstore.NewRunState("s1", "p1")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}

	types := readEventTypes(t, el)
	assertContainsEvent(t, types, "task_failed")
	assertContainsEvent(t, types, "terminal_state")

	if rs.Status != runstore.StatusNeedsHuman {
		t.Fatalf("expected needs_human, got %s", rs.Status)
	}
}

func TestEventContract_BlockedPath_PlanStageError(t *testing.T) {
	el, _ := newTestEventLog(t)

	stages := []Stage{
		&eventInitStage{eventLog: el},
		&eventCompileStage{eventLog: el},
		&errorStage{name: "plan", err: fmt.Errorf("infra failure")},
		&eventFinalizeStage{eventLog: el},
		&eventEvidenceStage{},
	}

	budget3 := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget3, EventLog: el})
	rs := runstore.NewRunState("s1", "p1")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}

	types := readEventTypes(t, el)

	assertContainsEvent(t, types, "run_started")
	assertContainsEvent(t, types, "spec_packet_compiled")
	assertContainsEvent(t, types, "terminal_state")

	if rs.Status != runstore.StatusBlocked {
		t.Fatalf("expected blocked, got %s", rs.Status)
	}

	// Verify terminal_state is the last event
	lastType := types[len(types)-1]
	if lastType != "terminal_state" {
		t.Fatalf("expected last event to be terminal_state, got %q", lastType)
	}
}

func TestEventContract_BlockedPath_PlanValidationFailed(t *testing.T) {
	el, _ := newTestEventLog(t)

	stages := []Stage{
		&eventInitStage{eventLog: el},
		&eventCompileStage{eventLog: el},
		&eventPlanStage{eventLog: el, tasks: nil, failFn: func() bool { return true }},
		&eventFinalizeStage{eventLog: el},
		&eventEvidenceStage{},
	}

	budget4 := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget4, EventLog: el})
	rs := runstore.NewRunState("s1", "p1")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}

	types := readEventTypes(t, el)

	assertContainsEvent(t, types, "plan_validation_result")
	assertContainsEvent(t, types, "terminal_state")

	if rs.Status != runstore.StatusBlocked {
		t.Fatalf("expected blocked, got %s", rs.Status)
	}
}

func TestEventContract_BudgetExceeded(t *testing.T) {
	el, _ := newTestEventLog(t)

	budget := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 1.0, MaxSpecCycles: 99})
	budget.AddCost(2.0) // exceed cost budget

	stages := []Stage{
		&eventInitStage{eventLog: el},
		&eventEvidenceStage{},
	}

	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget, EventLog: el})
	rs := runstore.NewRunState("s1", "p1")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}

	types := readEventTypes(t, el)
	assertContainsEvent(t, types, "budget_exceeded")
	assertContainsEvent(t, types, "terminal_state")

	if rs.Status != runstore.StatusBlocked {
		t.Fatalf("expected blocked, got %s", rs.Status)
	}
	if rs.TerminalReason != "budget_exceeded" {
		t.Fatalf("expected budget_exceeded reason, got %s", rs.TerminalReason)
	}
}

func TestEventContract_All15EventTypesCoveredAcrossScenarios(t *testing.T) {
	// Collect all event types from all scenarios and verify all 15 are covered.
	dir := t.TempDir()
	allTypes := map[string]bool{}

	// Scenario 1: happy path (covers 10 types)
	{
		el := runstore.NewEventLog(filepath.Join(dir, "happy.jsonl"))
		tasks := []runstore.Task{
			{TaskID: "t-001", Status: "pending", Objective: "do thing 1"},
		}
		stages := []Stage{
			&eventInitStage{eventLog: el},
			&eventCompileStage{eventLog: el},
			&eventPlanStage{eventLog: el, tasks: tasks},
			&eventExecuteStage{eventLog: el, allPass: true},
			&eventValidateStage{eventLog: el, passFn: func() bool { return true }},
			&eventFinalizeStage{eventLog: el},
			&eventEvidenceStage{},
		}
		b1 := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
		loop := NewSpecLoop(stages, SpecLoopConfig{Budget: b1, EventLog: el})
		rs := runstore.NewRunState("s1", "p1")
		loop.Run(context.Background(), rs)
		for _, et := range readEventTypes(t, el) {
			allTypes[et] = true
		}
	}

	// Scenario 2: task failure (covers task_failed)
	{
		el := runstore.NewEventLog(filepath.Join(dir, "fail.jsonl"))
		tasks := []runstore.Task{
			{TaskID: "t-001", Status: "pending", Objective: "do thing 1"},
		}
		stages := []Stage{
			&eventInitStage{eventLog: el},
			&eventCompileStage{eventLog: el},
			&eventPlanStage{eventLog: el, tasks: tasks},
			&eventExecuteStage{eventLog: el, allPass: false},
			&eventValidateStage{eventLog: el},
			&eventFinalizeStage{eventLog: el},
			&eventEvidenceStage{},
		}
		b2 := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
		loop := NewSpecLoop(stages, SpecLoopConfig{Budget: b2, EventLog: el})
		rs := runstore.NewRunState("s1", "p1")
		loop.Run(context.Background(), rs)
		for _, et := range readEventTypes(t, el) {
			allTypes[et] = true
		}
	}

	// Scenario 3: replan + budget (covers replan_triggered, budget_exceeded via cycles_exhausted)
	{
		el := runstore.NewEventLog(filepath.Join(dir, "replan.jsonl"))
		tasks := []runstore.Task{
			{TaskID: "t-001", Status: "pending", Objective: "do thing 1"},
		}
		stages := []Stage{
			&eventInitStage{eventLog: el},
			&eventCompileStage{eventLog: el},
			&eventPlanStage{eventLog: el, tasks: tasks},
			&eventExecuteStage{eventLog: el, allPass: true},
			&eventValidateStage{eventLog: el, passFn: func() bool { return false }},
			&eventFinalizeStage{eventLog: el},
			&eventEvidenceStage{},
		}
		budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 2, MaxRunCostUSD: 99})
		loop := NewSpecLoop(stages, SpecLoopConfig{
			Budget: budget, ReplanStage: "plan", EventLog: el,
		})
		rs := runstore.NewRunState("s1", "p1")
		loop.Run(context.Background(), rs)
		for _, et := range readEventTypes(t, el) {
			allTypes[et] = true
		}
	}

	// Scenario 4: budget_exceeded via cost
	{
		el := runstore.NewEventLog(filepath.Join(dir, "budget.jsonl"))
		budget := NewBudget(execpolicy.Budgets{MaxRunCostUSD: 1.0, MaxSpecCycles: 99})
		budget.AddCost(2.0)
		stages := []Stage{
			&eventInitStage{eventLog: el},
			&eventEvidenceStage{},
		}
		loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget, EventLog: el})
		rs := runstore.NewRunState("s1", "p1")
		loop.Run(context.Background(), rs)
		for _, et := range readEventTypes(t, el) {
			allTypes[et] = true
		}
	}

	// Scenario 5: needs_split + redecomposition (these are task-loop events, emitted via RunTaskLoop)
	// We need a direct RunTaskLoop call with EventLog for these
	{
		el := runstore.NewEventLog(filepath.Join(dir, "decomp.jsonl"))
		runner := &fakeTaskRunner{fn: func(_ context.Context, task runstore.Task) (TaskResult, error) {
			if task.TaskID == "t-001" {
				return TaskResult{Status: "needs_split"}, nil
			}
			return TaskResult{Status: "done"}, nil
		}}
		decomposer := &fakeDecomposer{subTasks: []runstore.Task{
			{TaskID: "t-001a", Status: "pending"},
		}}
		inspector := &fakeInspector{pass: true}
		tasks := []runstore.Task{{TaskID: "t-001", Status: "pending"}}
		RunTaskLoop(context.Background(), tasks, runner, TaskLoopConfig{
			MaxRetries: 1, Inspector: inspector, Decomposer: decomposer,
			MaxRedecompositions: 1, EventLog: el, Cycle: 1,
		})
		for _, et := range readEventTypes(t, el) {
			allTypes[et] = true
		}
	}

	// Check all 15 types are covered
	all15 := []string{
		"run_started", "spec_packet_compiled", "plan_created",
		"plan_validation_result", "task_created", "task_started",
		"task_validation_result", "task_completed", "task_failed",
		"task_needs_split", "redecomposition_triggered",
		"final_validation_result", "replan_triggered",
		"budget_exceeded", "terminal_state",
	}
	var missing []string
	for _, et := range all15 {
		if !allTypes[et] {
			missing = append(missing, et)
		}
	}
	if len(missing) > 0 {
		// Print what we did get for debugging
		var got []string
		for k := range allTypes {
			got = append(got, k)
		}
		t.Fatalf("missing event types: %v\n  got: %v", missing, got)
	}
}

func TestEventContract_EventOrderIsPreserved(t *testing.T) {
	el, _ := newTestEventLog(t)

	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "do thing 1"},
	}

	stages := []Stage{
		&eventInitStage{eventLog: el},
		&eventCompileStage{eventLog: el},
		&eventPlanStage{eventLog: el, tasks: tasks},
		&eventExecuteStage{eventLog: el, allPass: true},
		&eventValidateStage{eventLog: el, passFn: func() bool { return true }},
		&eventFinalizeStage{eventLog: el},
		&eventEvidenceStage{},
	}

	budgetOrder := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budgetOrder, EventLog: el})
	rs := runstore.NewRunState("s1", "p1")
	loop.Run(context.Background(), rs)

	types := readEventTypes(t, el)

	// Verify ordering constraints:
	// run_started must come before spec_packet_compiled
	// spec_packet_compiled must come before plan_created
	// plan_created must come before task_started
	// task_started must come before task_completed
	// task_completed must come before final_validation_result
	// final_validation_result must come before terminal_state
	order := map[string]int{}
	for i, et := range types {
		if _, exists := order[et]; !exists {
			order[et] = i
		}
	}

	pairs := [][2]string{
		{"run_started", "spec_packet_compiled"},
		{"spec_packet_compiled", "plan_created"},
		{"plan_created", "task_started"},
		{"task_started", "task_completed"},
		{"task_completed", "final_validation_result"},
		{"final_validation_result", "terminal_state"},
	}
	for _, p := range pairs {
		a, b := p[0], p[1]
		if order[a] >= order[b] {
			t.Fatalf("expected %q (at %d) before %q (at %d)\n  full: %v",
				a, order[a], b, order[b], types)
		}
	}
}

func TestEventContract_ReviewResultEventType(t *testing.T) {
	el, _ := newTestEventLog(t)

	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "do thing 1"},
	}

	stages := []Stage{
		&eventInitStage{eventLog: el},
		&eventCompileStage{eventLog: el},
		&eventPlanStage{eventLog: el, tasks: tasks},
		&eventExecuteStage{eventLog: el, allPass: true},
		&eventValidateStage{eventLog: el, passFn: func() bool { return true }},
		&eventReviewStage{
			eventLog:         el,
			totalFindings:    3,
			blockingFindings: 1,
			facetsReviewed:   []string{"correctness", "style"},
			erroredFacets:    []string{"perf"},
		},
		&eventFinalizeStage{eventLog: el},
		&eventEvidenceStage{},
	}

	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget, EventLog: el})
	rs := runstore.NewRunState("s1", "p1")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}

	types := readEventTypes(t, el)
	assertContainsEvent(t, types, "review_result")

	// Verify review_result payload via ReadAll
	events, err := el.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, ev := range events {
		if ev.EventType() == "review_result" {
			rr, ok := ev.(*runstore.ReviewResultEvent)
			if !ok {
				t.Fatal("review_result event is wrong type")
			}
			if rr.TotalFindings != 3 {
				t.Fatalf("want TotalFindings=3, got %d", rr.TotalFindings)
			}
			if rr.BlockingFindings != 1 {
				t.Fatalf("want BlockingFindings=1, got %d", rr.BlockingFindings)
			}
			if len(rr.FacetsReviewed) != 2 {
				t.Fatalf("want 2 facets reviewed, got %d", len(rr.FacetsReviewed))
			}
			if len(rr.ErroredFacets) != 1 {
				t.Fatalf("want 1 errored facet, got %d", len(rr.ErroredFacets))
			}
			found = true
		}
	}
	if !found {
		t.Fatal("review_result event not found in log")
	}
}

func TestEventContract_AcceptanceResultEventType(t *testing.T) {
	el, _ := newTestEventLog(t)

	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "do thing 1"},
	}

	stages := []Stage{
		&eventInitStage{eventLog: el},
		&eventCompileStage{eventLog: el},
		&eventPlanStage{eventLog: el, tasks: tasks},
		&eventExecuteStage{eventLog: el, allPass: true},
		&eventValidateStage{eventLog: el, passFn: func() bool { return true }},
		&eventReviewStage{eventLog: el, totalFindings: 0, facetsReviewed: []string{"correctness"}},
		&eventAcceptanceStage{
			eventLog:      el,
			totalCriteria: 5,
			passCount:     4,
			failCount:     1,
			unclearCount:  0,
		},
		&eventFinalizeStage{eventLog: el},
		&eventEvidenceStage{},
	}

	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget, EventLog: el})
	rs := runstore.NewRunState("s1", "p1")

	err := loop.Run(context.Background(), rs)
	if err != nil {
		t.Fatal(err)
	}

	types := readEventTypes(t, el)
	assertContainsEvent(t, types, "acceptance_result")

	// Verify acceptance_result payload via ReadAll
	events, err := el.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, ev := range events {
		if ev.EventType() == "acceptance_result" {
			ar, ok := ev.(*runstore.AcceptanceResultEvent)
			if !ok {
				t.Fatal("acceptance_result event is wrong type")
			}
			if ar.TotalCriteria != 5 {
				t.Fatalf("want TotalCriteria=5, got %d", ar.TotalCriteria)
			}
			if ar.PassCount != 4 {
				t.Fatalf("want PassCount=4, got %d", ar.PassCount)
			}
			if ar.FailCount != 1 {
				t.Fatalf("want FailCount=1, got %d", ar.FailCount)
			}
			if ar.UnclearCount != 0 {
				t.Fatalf("want UnclearCount=0, got %d", ar.UnclearCount)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("acceptance_result event not found in log")
	}
}

func TestEventContract_ReplanTriggeredEvent_SourceCovers_Validation_Review_Acceptance(t *testing.T) {
	sources := map[string]bool{}

	// Scenario 1: validation replan
	{
		el := runstore.NewEventLog(fmt.Sprintf("%s/val.jsonl", t.TempDir()))
		tasks := []runstore.Task{{TaskID: "t-001", Status: "pending", Objective: "do thing"}}
		stages := []Stage{
			&eventInitStage{eventLog: el},
			&eventCompileStage{eventLog: el},
			&eventPlanStage{eventLog: el, tasks: tasks},
			&eventExecuteStage{eventLog: el, allPass: true},
			&eventValidateStage{eventLog: el, passFn: func() bool { return false }},
			&eventFinalizeStage{eventLog: el},
			&eventEvidenceStage{},
		}
		budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 2, MaxRunCostUSD: 99})
		loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget, ReplanStage: "plan", EventLog: el})
		rs := runstore.NewRunState("s1", "p1")
		loop.Run(context.Background(), rs)

		events, _ := el.ReadAll()
		for _, ev := range events {
			if ev.EventType() == "replan_triggered" {
				rp := ev.(*runstore.ReplanTriggeredEvent)
				sources[rp.Source] = true
			}
		}
	}

	// Scenario 2: review replan
	{
		el := runstore.NewEventLog(fmt.Sprintf("%s/rev.jsonl", t.TempDir()))
		tasks := []runstore.Task{{TaskID: "t-001", Status: "pending", Objective: "do thing"}}
		reviewCalls := 0
		stages := []Stage{
			&eventInitStage{eventLog: el},
			&eventCompileStage{eventLog: el},
			&eventPlanStage{eventLog: el, tasks: tasks},
			&eventExecuteStage{eventLog: el, allPass: true},
			&eventValidateStage{eventLog: el, passFn: func() bool { return true }},
			&mockStage{name: "review", runFn: func(_ context.Context, rs *runstore.RunState) (NextAction, error) {
				reviewCalls++
				if reviewCalls == 1 {
					return NextAction{Kind: ReplanFrom, Context: &FailureContext{Failures: []string{"review findings"}}}, nil
				}
				return NextAction{Kind: Continue}, nil
			}},
			&eventFinalizeStage{eventLog: el},
			&eventEvidenceStage{},
		}
		budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 3, MaxRunCostUSD: 99})
		loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget, ReplanStage: "plan", EventLog: el})
		rs := runstore.NewRunState("s1", "p1")
		loop.Run(context.Background(), rs)

		events, _ := el.ReadAll()
		for _, ev := range events {
			if ev.EventType() == "replan_triggered" {
				rp := ev.(*runstore.ReplanTriggeredEvent)
				sources[rp.Source] = true
			}
		}
	}

	// Scenario 3: acceptance replan
	{
		el := runstore.NewEventLog(fmt.Sprintf("%s/acc.jsonl", t.TempDir()))
		tasks := []runstore.Task{{TaskID: "t-001", Status: "pending", Objective: "do thing"}}
		acceptCalls := 0
		stages := []Stage{
			&eventInitStage{eventLog: el},
			&eventCompileStage{eventLog: el},
			&eventPlanStage{eventLog: el, tasks: tasks},
			&eventExecuteStage{eventLog: el, allPass: true},
			&eventValidateStage{eventLog: el, passFn: func() bool { return true }},
			&eventReviewStage{eventLog: el, totalFindings: 0, facetsReviewed: []string{"correctness"}},
			&mockStage{name: "acceptance", runFn: func(_ context.Context, rs *runstore.RunState) (NextAction, error) {
				acceptCalls++
				if acceptCalls == 1 {
					return NextAction{Kind: ReplanFrom, Context: &FailureContext{Failures: []string{"acceptance failed"}}}, nil
				}
				return NextAction{Kind: Continue}, nil
			}},
			&eventFinalizeStage{eventLog: el},
			&eventEvidenceStage{},
		}
		budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 3, MaxRunCostUSD: 99})
		loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget, ReplanStage: "plan", EventLog: el})
		rs := runstore.NewRunState("s1", "p1")
		loop.Run(context.Background(), rs)

		events, _ := el.ReadAll()
		for _, ev := range events {
			if ev.EventType() == "replan_triggered" {
				rp := ev.(*runstore.ReplanTriggeredEvent)
				sources[rp.Source] = true
			}
		}
	}

	// Verify all three sources
	for _, src := range []string{"validate", "review", "acceptance"} {
		if !sources[src] {
			t.Fatalf("replan_triggered source %q not found; got sources: %v", src, sources)
		}
	}
}

func TestEventContract_EventOrder_ReviewAfterValidation_AcceptanceAfterReview(t *testing.T) {
	el, _ := newTestEventLog(t)

	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "pending", Objective: "do thing 1"},
	}

	stages := []Stage{
		&eventInitStage{eventLog: el},
		&eventCompileStage{eventLog: el},
		&eventPlanStage{eventLog: el, tasks: tasks},
		&eventExecuteStage{eventLog: el, allPass: true},
		&eventValidateStage{eventLog: el, passFn: func() bool { return true }},
		&eventReviewStage{eventLog: el, totalFindings: 0, facetsReviewed: []string{"correctness"}},
		&eventAcceptanceStage{eventLog: el, totalCriteria: 3, passCount: 3},
		&eventFinalizeStage{eventLog: el},
		&eventEvidenceStage{},
	}

	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
	loop := NewSpecLoop(stages, SpecLoopConfig{Budget: budget, EventLog: el})
	rs := runstore.NewRunState("s1", "p1")
	loop.Run(context.Background(), rs)

	types := readEventTypes(t, el)

	// Build first-occurrence index
	order := map[string]int{}
	for i, et := range types {
		if _, exists := order[et]; !exists {
			order[et] = i
		}
	}

	// review_result must come after final_validation_result
	if order["review_result"] <= order["final_validation_result"] {
		t.Fatalf("expected review_result (at %d) after final_validation_result (at %d)\n  full: %v",
			order["review_result"], order["final_validation_result"], types)
	}

	// acceptance_result must come after review_result
	if order["acceptance_result"] <= order["review_result"] {
		t.Fatalf("expected acceptance_result (at %d) after review_result (at %d)\n  full: %v",
			order["acceptance_result"], order["review_result"], types)
	}

	// terminal_state must come after acceptance_result
	if order["terminal_state"] <= order["acceptance_result"] {
		t.Fatalf("expected terminal_state (at %d) after acceptance_result (at %d)\n  full: %v",
			order["terminal_state"], order["acceptance_result"], types)
	}
}

func TestEventContract_BlockedWorktreeCleanedEvent(t *testing.T) {
	el, _ := newTestEventLog(t)

	evt := runstore.BlockedWorktreeCleanedEvent{
		BaseEvent:    runstore.BaseEvent{Type: "blocked_worktree_cleaned"},
		PriorRunID:   "run-abc123",
		WorktreePath: "/tmp/worktrees/spec-001",
	}
	el.Append(evt)

	events, err := el.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	got, ok := events[0].(*runstore.BlockedWorktreeCleanedEvent)
	if !ok {
		t.Fatalf("expected *BlockedWorktreeCleanedEvent, got %T", events[0])
	}
	if got.EventType() != "blocked_worktree_cleaned" {
		t.Errorf("EventType() = %q, want %q", got.EventType(), "blocked_worktree_cleaned")
	}
	if got.PriorRunID != "run-abc123" {
		t.Errorf("PriorRunID = %q, want %q", got.PriorRunID, "run-abc123")
	}
	if got.WorktreePath != "/tmp/worktrees/spec-001" {
		t.Errorf("WorktreePath = %q, want %q", got.WorktreePath, "/tmp/worktrees/spec-001")
	}
}
