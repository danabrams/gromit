package runstore

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/danabrams/gromit/internal/next/validator"
)

const (
	StatusRunning        = "running"
	StatusReadyForReview = "ready_for_review"
	StatusNeedsHuman     = "needs_human"
	StatusBlocked        = "blocked"
	// StatusCompleted indicates that human review has accepted the work.
	// This status will be actively used when Spec 0003 adds VISION review outcome labels.
	StatusCompleted = "completed"
)

// ReplanContext holds context for replan operations, including escalated failures
// for targeted task escalation in the execute stage.
type ReplanContext struct {
	Failures          []string `json:"failures"`
	EscalatedFailures []string `json:"escalated_failures,omitempty"`
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices to empty values for JSON consistency.
func (rc *ReplanContext) NormalizeNilFields() {
	if rc.Failures == nil {
		rc.Failures = []string{}
	}
	if rc.EscalatedFailures == nil {
		rc.EscalatedFailures = []string{}
	}
}

// TaskLineageEntry tracks the chain of task IDs for a particular lineage path.
type TaskLineageEntry struct {
	ChainIDs          []string `json:"chain_ids,omitempty"`
	ConsecutiveFails  int      `json:"consecutive_fails,omitempty"`
	LastError         string   `json:"last_error,omitempty"`
	OriginalTaskID    string   `json:"original_task_id,omitempty"`
	LastFailingTaskID string   `json:"last_failing_task_id,omitempty"`
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices to empty values for consistent JSON serialization.
func (tle *TaskLineageEntry) NormalizeNilFields() {
	if tle.ChainIDs == nil {
		tle.ChainIDs = []string{}
	}
}

// RunState represents the full state of an execution run.
type RunState struct {
	RunID                   string                      `json:"run_id"`
	SpecID                  string                      `json:"spec_id"`
	ProjectID               string                      `json:"project_id"`
	Status                  string                      `json:"status"`
	Cycle                   int                         `json:"cycle"`
	StartedAt               time.Time                   `json:"started_at"`
	EndedAt                 time.Time                   `json:"ended_at,omitempty"`
	Tasks                   []Task                      `json:"tasks"`
	WorktreePath            string                      `json:"worktree_path,omitempty"`
	BlockerSummary          string                      `json:"blocker_summary,omitempty"`
	AccumulatedCost         float64                     `json:"accumulated_cost"`
	TerminalReason          string                      `json:"terminal_reason,omitempty"`
	FinalValidationPassed   bool                        `json:"final_validation_passed"`
	FinalReviewPassed       bool                        `json:"final_review_passed"`
	FinalAcceptancePassed   bool                        `json:"final_acceptance_passed"`
	ReplanContext           *ReplanContext              `json:"replan_context,omitempty"`
	LastValidationResult    *string                     `json:"last_validation_result,omitempty"`
	LastFinalValidation     *validator.FinalResult      `json:"last_final_validation,omitempty"`
	LastContractFailures    []string                    `json:"last_contract_failures,omitempty"`
	ReviewFindings          []string                    `json:"review_findings,omitempty"`
	ReviewThrashCounts      map[string]int              `json:"review_thrash_counts,omitempty"`
	AcceptanceResults       []string                    `json:"acceptance_results,omitempty"`
	PriorReviewFindings     json.RawMessage             `json:"prior_review_findings,omitempty"`
	TotalReplans            int                         `json:"total_replans"`
	SpecConstraints         string                      `json:"spec_constraints,omitempty"`
	Resumed                 bool                        `json:"resumed,omitempty"`
	ContractsWritten        bool                        `json:"contracts_written"`
	ScenarioTestsWritten    bool                        `json:"scenario_tests_written"`
	FailureHistory          map[string]int              `json:"failure_history,omitempty"`
	TaskLineage             map[string]TaskLineageEntry `json:"task_lineage,omitempty"`
	ArchitectureConstraints []string                    `json:"architecture_constraints,omitempty"`
	BaselineFailures        map[string]string           `json:"baseline_failures,omitempty"`
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices to empty values for consistent JSON serialization.
func (rs *RunState) NormalizeNilFields() {
	if rs.Tasks == nil {
		rs.Tasks = []Task{}
	}
	if rs.ReplanContext == nil {
		rs.ReplanContext = &ReplanContext{}
	}
	rs.ReplanContext.NormalizeNilFields()
	if rs.ReviewFindings == nil {
		rs.ReviewFindings = []string{}
	}
	if rs.ReviewThrashCounts == nil {
		rs.ReviewThrashCounts = map[string]int{}
	}
	if rs.AcceptanceResults == nil {
		rs.AcceptanceResults = []string{}
	}
	if rs.LastContractFailures == nil {
		rs.LastContractFailures = []string{}
	}
	if rs.ArchitectureConstraints == nil {
		rs.ArchitectureConstraints = []string{}
	}
	if rs.FailureHistory == nil {
		rs.FailureHistory = map[string]int{}
	}
	if rs.TaskLineage == nil {
		rs.TaskLineage = make(map[string]TaskLineageEntry)
	}
	if rs.BaselineFailures == nil {
		rs.BaselineFailures = map[string]string{}
	}
	for i, entry := range rs.TaskLineage {
		entry.NormalizeNilFields()
		rs.TaskLineage[i] = entry
	}
	for i := range rs.Tasks {
		rs.Tasks[i].NormalizeNilFields()
	}
}

// UnmarshalJSON provides backward-compatible unmarshaling for RunState.
// It handles both legacy array-shaped replan_context (legacy format: ["failure1", "failure2"])
// and new object-shaped replan_context (current format: {"failures": [...], "escalated_failures": [...]}).
func (rs *RunState) UnmarshalJSON(data []byte) error {
	// Temporary struct that captures replan_context as raw JSON for custom parsing.
	type Alias RunState
	aux := &struct {
		ReplanContextRaw json.RawMessage `json:"replan_context"`
		*Alias
	}{
		Alias: (*Alias)(rs),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Handle replan_context: support both legacy array format and new object format.
	if len(aux.ReplanContextRaw) > 0 {
		// Try to detect format by attempting to unmarshal as array first.
		var legacyArray []string
		if err := json.Unmarshal(aux.ReplanContextRaw, &legacyArray); err == nil && aux.ReplanContextRaw[0] == '[' {
			// It's an array (legacy format).
			rs.ReplanContext = &ReplanContext{
				Failures:          legacyArray,
				EscalatedFailures: []string{},
			}
		} else {
			// Try new object format.
			var rc ReplanContext
			if err := json.Unmarshal(aux.ReplanContextRaw, &rc); err != nil {
				return fmt.Errorf("unmarshal replan_context: %w", err)
			}
			rs.ReplanContext = &rc
		}
	}

	return nil
}

// IsTerminal returns true if the run is in a terminal state.
func (rs *RunState) IsTerminal() bool {
	switch rs.Status {
	case StatusReadyForReview, StatusNeedsHuman, StatusBlocked, StatusCompleted:
		return true
	}
	return false
}

// Task represents a single task within a run.
type Task struct {
	TaskID              string   `json:"task_id"`
	Objective           string   `json:"objective"`
	Status              string   `json:"status"` // pending, running, done, failed, needs_split
	Attempts            int      `json:"attempts"`
	ExpectedTouchedArea []string `json:"expected_touched_area"`
	ProofChecks         []string `json:"proof_checks"`
	FilesChanged        []string `json:"files_changed"`
	TokensUsed          int      `json:"tokens_used"`
	DurationMs          int64    `json:"duration_ms"`
	ModelTier           string   `json:"model_tier"`
	Cycle               int      `json:"cycle"`
	Kind                string   `json:"kind"` // "original" or "fix"
	ParentCycle         int      `json:"parent_cycle,omitempty"`
	FailuresAddressed   []string `json:"failures_addressed,omitempty"`
	SpecConstraints     string   `json:"spec_constraints,omitempty"`
	Fixes               string   `json:"fixes,omitempty"`
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices to empty values for consistent JSON serialization.
func (tk *Task) NormalizeNilFields() {
	if tk.ExpectedTouchedArea == nil {
		tk.ExpectedTouchedArea = []string{}
	}
	if tk.ProofChecks == nil {
		tk.ProofChecks = []string{}
	}
	if tk.FilesChanged == nil {
		tk.FilesChanged = []string{}
	}
	if tk.FailuresAddressed == nil {
		tk.FailuresAddressed = []string{}
	}
}

// InvocationRecord captures metadata for a single LLM invocation.
// Defined here (not in evidence) so that packages like specloop and llmadapter
// can reference it without importing evidence (which imports runstore).
type InvocationRecord struct {
	Phase      string  `json:"phase"`
	Tier       string  `json:"tier"`
	Model      string  `json:"model"`
	Provider   string  `json:"provider"`
	TokensIn   int     `json:"tokens_in"`
	TokensOut  int     `json:"tokens_out"`
	DurationMs int64   `json:"duration_ms"`
	CostUSD    float64 `json:"cost_usd"`
	Success    bool    `json:"success"`
}

// NewRunState creates a new RunState with a generated ID and running status.
func NewRunState(specID, projectID string) *RunState {
	return &RunState{
		RunID:            generateID("run"),
		SpecID:           specID,
		ProjectID:        projectID,
		Status:           StatusRunning,
		StartedAt:        time.Now(),
		Tasks:            []Task{},
		BaselineFailures: map[string]string{},
	}
}

// generateID creates a unique ID with the given prefix and 8 random bytes as hex.
func generateID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return prefix + "-" + hex.EncodeToString(b)
}
