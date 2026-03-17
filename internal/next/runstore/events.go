package runstore

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// TypedEvent is the interface for all event types in the event log.
type TypedEvent interface {
	EventType() string
	EventTimestamp() time.Time
}

// BaseEvent contains fields common to all events.
type BaseEvent struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

func (b BaseEvent) EventType() string         { return b.Type }
func (b BaseEvent) EventTimestamp() time.Time { return b.Timestamp }

// ValidationCheckResult holds the result of a single validation check.
type ValidationCheckResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Output string `json:"output"`
}

// --- Event types ---

type RunStartedEvent struct {
	BaseEvent
	SpecID    string `json:"spec_id"`
	ProjectID string `json:"project_id"`
}

type SpecPacketCompiledEvent struct {
	BaseEvent
}

type PlanCreatedEvent struct {
	BaseEvent
	TaskCount int `json:"task_count"`
}

type PlanValidationResultEvent struct {
	BaseEvent
	Passed bool                    `json:"passed"`
	Checks []ValidationCheckResult `json:"checks,omitempty"`
}

type TaskCreatedEvent struct {
	BaseEvent
	TaskID    string `json:"task_id"`
	Objective string `json:"objective,omitempty"`
}

type TaskStartedEvent struct {
	BaseEvent
	TaskID    string `json:"task_id"`
	Cycle     int    `json:"cycle"`
	ModelTier string `json:"model_tier,omitempty"`
	TaskIndex int    `json:"task_index,omitempty"` // 1-based index in queue
	TaskTotal int    `json:"task_total,omitempty"` // total tasks in queue
	Objective string `json:"objective,omitempty"`  // task description
}

type TaskValidationResultEvent struct {
	BaseEvent
	TaskID string                  `json:"task_id"`
	Passed bool                    `json:"passed"`
	Checks []ValidationCheckResult `json:"checks,omitempty"`
}

type TaskCompletedEvent struct {
	BaseEvent
	TaskID     string `json:"task_id"`
	TokensUsed int    `json:"tokens_used,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

type TaskFailedEvent struct {
	BaseEvent
	TaskID string `json:"task_id"`
	Reason string `json:"reason"`
}

type TaskNeedsSplitEvent struct {
	BaseEvent
	TaskID string `json:"task_id"`
}

type RedecompositionTriggeredEvent struct {
	BaseEvent
	Reason string `json:"reason,omitempty"`
}

type FinalValidationResultEvent struct {
	BaseEvent
	Passed bool                    `json:"passed"`
	Checks []ValidationCheckResult `json:"checks,omitempty"`
}

type ReplanTriggeredEvent struct {
	BaseEvent
	Reason string `json:"reason,omitempty"`
	Source string `json:"source,omitempty"`
}

type BudgetExceededEvent struct {
	BaseEvent
	AccumulatedCost float64 `json:"accumulated_cost"`
	Budget          float64 `json:"budget,omitempty"`
}

type ReviewResultEvent struct {
	BaseEvent
	TotalFindings      int            `json:"total_findings"`
	BlockingFindings   int            `json:"blocking_findings"`
	FindingsBySeverity map[string]int `json:"findings_by_severity,omitempty"`
	FacetsReviewed     []string       `json:"facets_reviewed"`
	ErroredFacets      []string       `json:"errored_facets,omitempty"`
}

type AcceptanceResultEvent struct {
	BaseEvent
	TotalCriteria int `json:"total_criteria"`
	PassCount     int `json:"pass_count"`
	FailCount     int `json:"fail_count"`
	UnclearCount  int `json:"unclear_count"`
}

type BlockedWorktreeCleanedEvent struct {
	BaseEvent
	PriorRunID   string `json:"prior_run_id"`
	WorktreePath string `json:"worktree_path"`
}

type ContractsWrittenEvent struct {
	BaseEvent
	ScenarioCount int `json:"scenario_count"`
}

type ContractsBlockedEvent struct {
	BaseEvent
	Reason string `json:"reason"`
}

type ContractScenarioSkippedEvent struct {
	BaseEvent
	Reason string `json:"reason"`
}

type TerminalStateEvent struct {
	BaseEvent
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// --- EventLog ---

// EventLog manages append-only event logging to a JSONL file.
type EventLog struct {
	path string
}

// NewEventLog creates a new EventLog writing to the given file path.
func NewEventLog(path string) *EventLog {
	return &EventLog{path: path}
}

// Append marshals an event to JSON and appends it as a line to the log file.
// Callers intentionally ignore the returned error (fire-and-forget) consistent
// with the SpecLoop pattern — event logging must not block the pipeline. If an
// error occurs, a warning is printed to stderr for observability.
func (el *EventLog) Append(ev TypedEvent) error {
	data, err := json.Marshal(ev)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gromit: event log: marshal %s: %v\n", ev.EventType(), err)
		return fmt.Errorf("marshal event: %w", err)
	}
	f, err := os.OpenFile(el.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gromit: event log: open %s: %v\n", el.path, err)
		return fmt.Errorf("open event log: %w", err)
	}
	defer f.Close()
	if _, err = f.Write(append(data, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "gromit: event log: write %s: %v\n", el.path, err)
		return err
	}
	return nil
}

// ReadAll reads all events from the log file.
func (el *EventLog) ReadAll() ([]TypedEvent, error) {
	f, err := os.Open(el.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []TypedEvent{}, nil
		}
		return nil, fmt.Errorf("open event log: %w", err)
	}
	defer f.Close()

	var events []TypedEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		ev, err := unmarshalEvent([]byte(line))
		if err != nil {
			return nil, fmt.Errorf("unmarshal event: %w", err)
		}
		events = append(events, ev)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan event log: %w", err)
	}
	if events == nil {
		events = []TypedEvent{}
	}
	return events, nil
}

// unmarshalEvent peeks at the "type" field and unmarshals to the correct struct.
func unmarshalEvent(data []byte) (TypedEvent, error) {
	var peek struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		return nil, err
	}

	var ev TypedEvent
	switch peek.Type {
	case "run_started":
		var e RunStartedEvent
		ev = &e
	case "spec_packet_compiled":
		var e SpecPacketCompiledEvent
		ev = &e
	case "plan_created":
		var e PlanCreatedEvent
		ev = &e
	case "plan_validation_result":
		var e PlanValidationResultEvent
		ev = &e
	case "task_created":
		var e TaskCreatedEvent
		ev = &e
	case "task_started":
		var e TaskStartedEvent
		ev = &e
	case "task_validation_result":
		var e TaskValidationResultEvent
		ev = &e
	case "task_completed":
		var e TaskCompletedEvent
		ev = &e
	case "task_failed":
		var e TaskFailedEvent
		ev = &e
	case "task_needs_split":
		var e TaskNeedsSplitEvent
		ev = &e
	case "redecomposition_triggered":
		var e RedecompositionTriggeredEvent
		ev = &e
	case "final_validation_result":
		var e FinalValidationResultEvent
		ev = &e
	case "replan_triggered":
		var e ReplanTriggeredEvent
		ev = &e
	case "budget_exceeded":
		var e BudgetExceededEvent
		ev = &e
	case "review_result":
		var e ReviewResultEvent
		ev = &e
	case "acceptance_result":
		var e AcceptanceResultEvent
		ev = &e
	case "blocked_worktree_cleaned":
		var e BlockedWorktreeCleanedEvent
		ev = &e
	case "terminal_state":
		var e TerminalStateEvent
		ev = &e
	case "contracts_written":
		var e ContractsWrittenEvent
		ev = &e
	case "contracts_blocked":
		var e ContractsBlockedEvent
		ev = &e
	case "contract_scenario_skipped":
		var e ContractScenarioSkippedEvent
		ev = &e
	default:
		return nil, fmt.Errorf("unknown event type: %s", peek.Type)
	}

	if err := json.Unmarshal(data, ev); err != nil {
		return nil, err
	}
	return ev, nil
}
