package runstore

import (
	"encoding/json"
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
		BlockedWorktreeCleanedEvent{BaseEvent: BaseEvent{Type: "blocked_worktree_cleaned", Timestamp: now}, PriorRunID: "run-old", WorktreePath: "/old"},
		TerminalStateEvent{BaseEvent: BaseEvent{Type: "terminal_state", Timestamp: now}, Status: "ready_for_review"},
		DiffUnavailableEvent{BaseEvent: BaseEvent{Type: "diff_unavailable", Timestamp: time.Now()}, Reason: "test error", Message: "test message"},
		&ContractDeferredEvent{BaseEvent: BaseEvent{Type: "contract_deferred", Timestamp: now}, ScenarioName: "auth-flow", FilePath: "internal/auth/contract.go", Pattern: "login_success", TaskID: "t-042"},
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
	if len(events) != 18 {
		t.Fatalf("want 18 events, got %d", len(events))
	}
	for i, ev := range events {
		if ev.EventType() != allEvents[i].EventType() {
			t.Errorf("event %d: want %s, got %s", i, allEvents[i].EventType(), ev.EventType())
		}
	}
}

func TestReviewResultEvent_JSON(t *testing.T) {
	evt := ReviewResultEvent{
		BaseEvent:        BaseEvent{Type: "review_result", Timestamp: time.Now()},
		TotalFindings:    3,
		BlockingFindings: 1,
		FacetsReviewed:   []string{"spec_alignment", "code_quality"},
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["type"] != "review_result" {
		t.Errorf("type = %v, want review_result", got["type"])
	}
}

func TestReviewResultEvent_FindingsBySeverity_JSON(t *testing.T) {
	evt := ReviewResultEvent{
		BaseEvent:        BaseEvent{Type: "review_result", Timestamp: time.Now()},
		TotalFindings:    5,
		BlockingFindings: 2,
		FindingsBySeverity: map[string]int{
			"critical": 2,
			"warning":  3,
		},
		FacetsReviewed: []string{"spec_alignment"},
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var roundTripped ReviewResultEvent
	if err := json.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if roundTripped.FindingsBySeverity == nil {
		t.Fatal("FindingsBySeverity should not be nil after round-trip")
	}
	if roundTripped.FindingsBySeverity["critical"] != 2 {
		t.Errorf("critical = %d, want 2", roundTripped.FindingsBySeverity["critical"])
	}
	if roundTripped.FindingsBySeverity["warning"] != 3 {
		t.Errorf("warning = %d, want 3", roundTripped.FindingsBySeverity["warning"])
	}
}

func TestReviewResultEvent_FindingsBySeverity_OmittedWhenNil(t *testing.T) {
	evt := ReviewResultEvent{
		BaseEvent:      BaseEvent{Type: "review_result", Timestamp: time.Now()},
		TotalFindings:  0,
		FacetsReviewed: []string{},
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, exists := got["findings_by_severity"]; exists {
		t.Error("findings_by_severity should be omitted when nil")
	}
}

func TestAcceptanceResultEvent_JSON(t *testing.T) {
	evt := AcceptanceResultEvent{
		BaseEvent:     BaseEvent{Type: "acceptance_result", Timestamp: time.Now()},
		TotalCriteria: 5,
		PassCount:     4,
		FailCount:     1,
		UnclearCount:  0,
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["type"] != "acceptance_result" {
		t.Errorf("type = %v, want acceptance_result", got["type"])
	}
}

func TestReplanTriggeredEvent_Source_JSON(t *testing.T) {
	evt := ReplanTriggeredEvent{
		BaseEvent: BaseEvent{Type: "replan_triggered", Timestamp: time.Now()},
		Reason:    "blocking findings",
		Source:    "review",
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["source"] != "review" {
		t.Errorf("source = %v, want review", got["source"])
	}
}

func TestBlockedWorktreeCleanedEvent_JSON(t *testing.T) {
	evt := BlockedWorktreeCleanedEvent{
		BaseEvent:    BaseEvent{Type: "blocked_worktree_cleaned", Timestamp: time.Now()},
		PriorRunID:   "run-abc-123",
		WorktreePath: "/path/to/old-worktree",
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["type"] != "blocked_worktree_cleaned" {
		t.Errorf("type = %v, want blocked_worktree_cleaned", got["type"])
	}
	if got["prior_run_id"] != "run-abc-123" {
		t.Errorf("prior_run_id = %v, want run-abc-123", got["prior_run_id"])
	}
}

func TestUnmarshalEvent_BlockedWorktreeCleaned(t *testing.T) {
	jsonStr := `{"type":"blocked_worktree_cleaned","timestamp":"2026-03-12T00:00:00Z","prior_run_id":"run-xyz","worktree_path":"/old"}`
	evt, err := unmarshalEvent([]byte(jsonStr))
	if err != nil {
		t.Fatalf("unmarshalEvent: %v", err)
	}
	bwc, ok := evt.(*BlockedWorktreeCleanedEvent)
	if !ok {
		t.Fatalf("expected *BlockedWorktreeCleanedEvent, got %T", evt)
	}
	if bwc.PriorRunID != "run-xyz" {
		t.Errorf("PriorRunID = %q, want run-xyz", bwc.PriorRunID)
	}
}

func TestContractsWrittenEvent_JSON(t *testing.T) {
	evt := ContractsWrittenEvent{
		BaseEvent:     BaseEvent{Type: "contracts_written", Timestamp: time.Now()},
		ScenarioCount: 5,
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["type"] != "contracts_written" {
		t.Errorf("type = %v, want contracts_written", got["type"])
	}
	if got["scenario_count"] != float64(5) {
		t.Errorf("scenario_count = %v, want 5", got["scenario_count"])
	}
}

func TestContractsBlockedEvent_JSON(t *testing.T) {
	evt := ContractsBlockedEvent{
		BaseEvent: BaseEvent{Type: "contracts_blocked", Timestamp: time.Now()},
		Reason:    "spec not ready",
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["type"] != "contracts_blocked" {
		t.Errorf("type = %v, want contracts_blocked", got["type"])
	}
	if got["reason"] != "spec not ready" {
		t.Errorf("reason = %v, want 'spec not ready'", got["reason"])
	}
}

func TestUnmarshalEvent_ContractsWritten(t *testing.T) {
	jsonStr := `{"type":"contracts_written","timestamp":"2026-03-16T00:00:00Z","scenario_count":3}`
	evt, err := unmarshalEvent([]byte(jsonStr))
	if err != nil {
		t.Fatalf("unmarshalEvent: %v", err)
	}
	cw, ok := evt.(*ContractsWrittenEvent)
	if !ok {
		t.Fatalf("expected *ContractsWrittenEvent, got %T", evt)
	}
	if cw.ScenarioCount != 3 {
		t.Errorf("ScenarioCount = %d, want 3", cw.ScenarioCount)
	}
}

func TestUnmarshalEvent_ContractsBlocked(t *testing.T) {
	jsonStr := `{"type":"contracts_blocked","timestamp":"2026-03-16T00:00:00Z","reason":"no scenarios"}`
	evt, err := unmarshalEvent([]byte(jsonStr))
	if err != nil {
		t.Fatalf("unmarshalEvent: %v", err)
	}
	cb, ok := evt.(*ContractsBlockedEvent)
	if !ok {
		t.Fatalf("expected *ContractsBlockedEvent, got %T", evt)
	}
	if cb.Reason != "no scenarios" {
		t.Errorf("Reason = %q, want 'no scenarios'", cb.Reason)
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

func TestDiffUnavailableEvent_RoundTrip(t *testing.T) {
	evt := DiffUnavailableEvent{
		BaseEvent: BaseEvent{Type: "diff_unavailable", Timestamp: time.Now()},
		Reason:    "binary files",
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	roundTripped, err := unmarshalEvent(data)
	if err != nil {
		t.Fatalf("unmarshalEvent: %v", err)
	}

	due, ok := roundTripped.(*DiffUnavailableEvent)
	if !ok {
		t.Fatalf("expected *DiffUnavailableEvent, got %T", roundTripped)
	}
	if due.Reason != "binary files" {
		t.Errorf("Reason = %q, want %q", due.Reason, "binary files")
	}
	if due.Type != "diff_unavailable" {
		t.Errorf("Type = %q, want %q", due.Type, "diff_unavailable")
	}
}

func TestContractDeferredEvent_RoundTrip(t *testing.T) {
	evt := ContractDeferredEvent{
		BaseEvent:    BaseEvent{Type: "contract_deferred", Timestamp: time.Now()},
		ScenarioName: "auth-flow",
		FilePath:     "internal/auth/contract.go",
		Pattern:      "login_success",
		TaskID:       "t-042",
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	roundTripped, err := unmarshalEvent(data)
	if err != nil {
		t.Fatalf("unmarshalEvent: %v", err)
	}

	cde, ok := roundTripped.(*ContractDeferredEvent)
	if !ok {
		t.Fatalf("expected *ContractDeferredEvent, got %T", roundTripped)
	}
	if cde.ScenarioName != "auth-flow" {
		t.Errorf("ScenarioName = %q, want %q", cde.ScenarioName, "auth-flow")
	}
	if cde.FilePath != "internal/auth/contract.go" {
		t.Errorf("FilePath = %q, want %q", cde.FilePath, "internal/auth/contract.go")
	}
	if cde.Pattern != "login_success" {
		t.Errorf("Pattern = %q, want %q", cde.Pattern, "login_success")
	}
	if cde.TaskID != "t-042" {
		t.Errorf("TaskID = %q, want %q", cde.TaskID, "t-042")
	}
	if cde.Type != "contract_deferred" {
		t.Errorf("Type = %q, want %q", cde.Type, "contract_deferred")
	}
}
