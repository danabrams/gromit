package runstore

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEvents_AppendAndRead(t *testing.T) {
	dir := t.TempDir()
	el := NewEventLog(filepath.Join(dir, "events.jsonl"))

	el.Append(RunStartedEvent{
		BaseEvent: BaseEvent{Type: "run_started", Timestamp: time.Now()},
		SpecID:    "spec-1",
		ProjectID: "proj-1",
	})
	el.Append(TaskStartedEvent{
		BaseEvent: BaseEvent{Type: "task_started", Timestamp: time.Now()},
		TaskID:    "t-001",
		Cycle:     1,
	})

	events, err := el.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d", len(events))
	}
	if events[0].EventType() != "run_started" {
		t.Fatalf("want run_started, got %s", events[0].EventType())
	}
	if events[1].EventType() != "task_started" {
		t.Fatalf("want task_started, got %s", events[1].EventType())
	}
}

func TestEvents_AllEventTypes(t *testing.T) {
	dir := t.TempDir()
	el := NewEventLog(filepath.Join(dir, "events.jsonl"))
	now := time.Now()

	allEvents := []TypedEvent{
		RunStartedEvent{BaseEvent: BaseEvent{Type: "run_started", Timestamp: now}, SpecID: "s1", ProjectID: "p1"},
		SpecPacketCompiledEvent{BaseEvent: BaseEvent{Type: "spec_packet_compiled", Timestamp: now}},
		PlanCreatedEvent{BaseEvent: BaseEvent{Type: "plan_created", Timestamp: now}, TaskCount: 3},
		PlanValidationResultEvent{BaseEvent: BaseEvent{Type: "plan_validation_result", Timestamp: now}, Passed: true},
		TaskCreatedEvent{BaseEvent: BaseEvent{Type: "task_created", Timestamp: now}, TaskID: "t1"},
		TaskStartedEvent{BaseEvent: BaseEvent{Type: "task_started", Timestamp: now}, TaskID: "t1", Cycle: 1},
		TaskValidationResultEvent{BaseEvent: BaseEvent{Type: "task_validation_result", Timestamp: now}, TaskID: "t1", Passed: true},
		TaskCompletedEvent{BaseEvent: BaseEvent{Type: "task_completed", Timestamp: now}, TaskID: "t1"},
		TaskFailedEvent{BaseEvent: BaseEvent{Type: "task_failed", Timestamp: now}, TaskID: "t1", Reason: "timeout"},
		TaskNeedsSplitEvent{BaseEvent: BaseEvent{Type: "task_needs_split", Timestamp: now}, TaskID: "t1"},
		RedecompositionTriggeredEvent{BaseEvent: BaseEvent{Type: "redecomposition_triggered", Timestamp: now}},
		FinalValidationResultEvent{BaseEvent: BaseEvent{Type: "final_validation_result", Timestamp: now}, Passed: false},
		ReplanTriggeredEvent{BaseEvent: BaseEvent{Type: "replan_triggered", Timestamp: now}},
		BudgetExceededEvent{BaseEvent: BaseEvent{Type: "budget_exceeded", Timestamp: now}, AccumulatedCost: 5.50},
		TerminalStateEvent{BaseEvent: BaseEvent{Type: "terminal_state", Timestamp: now}, Status: "ready_for_review"},
	}

	for _, ev := range allEvents {
		if err := el.Append(ev); err != nil {
			t.Fatalf("append %s: %v", ev.EventType(), err)
		}
	}

	events, err := el.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 15 {
		t.Fatalf("want 15 events, got %d", len(events))
	}
	for i, ev := range events {
		if ev.EventType() != allEvents[i].EventType() {
			t.Errorf("event %d: want %s, got %s", i, allEvents[i].EventType(), ev.EventType())
		}
	}
}

func TestEvents_ReadAll_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	el := NewEventLog(filepath.Join(dir, "events.jsonl"))

	events, err := el.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("want 0 events, got %d", len(events))
	}
}
