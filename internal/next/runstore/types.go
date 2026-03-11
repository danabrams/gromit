package runstore

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	StatusRunning        = "running"
	StatusReadyForReview = "ready_for_review"
	StatusNeedsHuman     = "needs_human"
	StatusBlocked        = "blocked"
)

// RunState represents the full state of an execution run.
type RunState struct {
	RunID                 string    `json:"run_id"`
	SpecID                string    `json:"spec_id"`
	ProjectID             string    `json:"project_id"`
	Status                string    `json:"status"`
	Cycle                 int       `json:"cycle"`
	StartedAt             time.Time `json:"started_at"`
	EndedAt               time.Time `json:"ended_at,omitempty"`
	Tasks                 []Task    `json:"tasks"`
	WorktreePath          string    `json:"worktree_path,omitempty"`
	BlockerSummary        string    `json:"blocker_summary,omitempty"`
	AccumulatedCost       float64   `json:"accumulated_cost"`
	TerminalReason        string    `json:"terminal_reason,omitempty"`
	FinalValidationPassed bool      `json:"final_validation_passed"`
}

// NormalizeNilFields maps nil slices to empty values for consistent JSON serialization.
func (rs *RunState) NormalizeNilFields() {
	if rs.Tasks == nil {
		rs.Tasks = []Task{}
	}
	for i := range rs.Tasks {
		rs.Tasks[i].NormalizeNilFields()
	}
}

// IsTerminal returns true if the run is in a terminal state.
func (rs *RunState) IsTerminal() bool {
	switch rs.Status {
	case StatusReadyForReview, StatusNeedsHuman, StatusBlocked:
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
}

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

// NewRunState creates a new RunState with a generated ID and running status.
func NewRunState(specID, projectID string) *RunState {
	return &RunState{
		RunID:     generateID("run"),
		SpecID:    specID,
		ProjectID: projectID,
		Status:    StatusRunning,
		StartedAt: time.Now(),
		Tasks:     []Task{},
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
