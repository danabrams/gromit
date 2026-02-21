package pipeline

import "github.com/danabrams/gromit/internal/prompt"

// EventType represents the type of event in an interactive session.
type EventType int

const (
	EventOutput EventType = iota
	EventSessionStarted
	EventSessionEnded
	EventError
)

// Event represents a single event in an interactive session.
type Event struct {
	Type    EventType
	Content string
}

// Session represents an interactive workflow session.
type Session interface {
	Events() <-chan Event
	SendInput(text string) error
	Cancel()
	Wait() error
}

// RefineInput contains parameters for the Refine workflow.
type RefineInput struct {
	IdeaText  string // Ad-hoc idea text (mutually exclusive with IdeaID)
	IdeaID    string // Backlog item ID (mutually exclusive with IdeaText)
	AgentName string // Optional agent override
}

// RefineResult contains the output from the Refine workflow.
// Use NewRefineResult() to create instances with properly initialized slices.
type RefineResult struct {
	CreatedSpecs []string `json:"created_specs"`
	RefinedItems []string `json:"refined_items"`
}

// NewRefineResult creates a RefineResult with initialized slices.
func NewRefineResult() RefineResult {
	return RefineResult{
		CreatedSpecs: []string{},
		RefinedItems: []string{},
	}
}

// PlanInput contains parameters for the Plan workflow.
type PlanInput struct {
	SpecName  string // Name of spec to plan
	AgentName string // Optional agent override
	Force     bool   // Re-plan even if already done
}

// PlanResult contains the output from the Plan workflow.
// Use NewPlanResult() to create instances with properly initialized slices.
type PlanResult struct {
	CreatedPlans []string `json:"created_plans"`
}

// NewPlanResult creates a PlanResult with initialized slices.
func NewPlanResult() PlanResult {
	return PlanResult{
		CreatedPlans: []string{},
	}
}

// DecomposeInput contains parameters for the Decompose workflow.
type DecomposeInput struct {
	PlanName             string // Name of plan to decompose
	Force                bool   // Re-decompose even if already done
	Review               bool   // Return proposed beads for review before creating
	SkipValidation       bool   // Skip validation checks on decomposed bead candidates
	MaxValidationRetries int    // Max retries after validation failures
}

// CreatedBead contains information about a bead that was created.
// Use NewCreatedBead() to create instances with properly initialized slices.
type CreatedBead struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Priority int      `json:"priority"`
	Labels   []string `json:"labels"`
}

// NewCreatedBead creates a CreatedBead with initialized slices.
func NewCreatedBead() CreatedBead {
	return CreatedBead{
		Labels: []string{},
	}
}

// ValidationStats tracks validation retry metrics for a decompose run.
type ValidationStats struct {
	Attempts       int  `json:"attempts"`        // Total provider invocations (1 = no retries)
	ViolationCount int  `json:"violation_count"` // Total violations found across all attempts
	Improved       bool `json:"improved"`        // True if retry reduced violations vs first attempt
}

// DecomposeResult contains the output from the Decompose workflow.
// Use NewDecomposeResult() to create instances with properly initialized slices.
type DecomposeResult struct {
	CreatedBeads      []CreatedBead             `json:"created_beads"`
	PlanUpdated       bool                      `json:"plan_updated"`
	PromptDiagnostics *prompt.PromptDiagnostics `json:"prompt_diagnostics,omitempty"`
	ValidationStats   *ValidationStats          `json:"validation_stats,omitempty"`
}

// NewDecomposeResult creates a DecomposeResult with initialized slices.
func NewDecomposeResult() DecomposeResult {
	return DecomposeResult{
		CreatedBeads: []CreatedBead{},
	}
}

// ReviewInput contains parameters for the Review workflow.
type ReviewInput struct {
	FromCommit string // Git commit ref to review from
	Diff       string // Diff content to review
	Model      string // Model to use for review
	Timeout    int    // Timeout in seconds
	AgentName  string // Optional agent override (for interactive mode)
	LaunchDir  string // Working directory for interactive agent launch
	Spec       string // Spec label to scope review (used by CLI to resolve FromCommit)
	Epic       string // Epic label to scope review (used by CLI to resolve FromCommit)
}

// ReviewResult contains the output from the Review workflow.
// Use NewReviewResult() to create instances with properly initialized slices.
type ReviewResult struct {
	Passed         bool   `json:"passed"`
	Summary        string `json:"summary"`
	FixesApplied   int    `json:"fixes_applied"`
	BeadsCreated   int    `json:"beads_created"`
	BacklogCreated int    `json:"backlog_created"`
}

// NewReviewResult creates a ReviewResult with initialized slices.
func NewReviewResult() ReviewResult {
	return ReviewResult{}
}

// ExploreInput contains parameters for the Explore workflow.
type ExploreInput struct {
	Topic       string // Topic to explore (optional)
	AgentName   string // Optional agent override
	ChooseAgent bool   // Show interactive picker to choose agent
	Model       string // Model to use for exploration
}

// ExploreResult contains the output from the Explore workflow.
// Use NewExploreResult() to create instances with properly initialized slices.
type ExploreResult struct {
	CreatedSpecs        []string `json:"created_specs"`
	CreatedEpics        []string `json:"created_epics"`
	CreatedBacklogItems []string `json:"created_backlog_items"`
}

// NewExploreResult creates an ExploreResult with initialized slices.
func NewExploreResult() ExploreResult {
	return ExploreResult{
		CreatedSpecs:        []string{},
		CreatedEpics:        []string{},
		CreatedBacklogItems: []string{},
	}
}

// RefineSession is a typed wrapper for interactive Refine sessions.
type RefineSession struct {
	Session
}

// PlanSession is a typed wrapper for interactive Plan sessions.
type PlanSession struct {
	Session
}

// ReviewSession is a typed wrapper for interactive Review sessions.
type ReviewSession struct {
	Session
}

// ExploreSession is a typed wrapper for interactive Explore sessions.
type ExploreSession struct {
	Session
}
