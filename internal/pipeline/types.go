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

// ComplexityRouting carries normalized complexity metadata between pipeline stages.
// Zero values are valid and indicate unset/unknown metadata.
type ComplexityRouting struct {
	Complexity               string
	ComplexitySource         string
	ComplexityFallbackReason string
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
	IdeaText    string // Ad-hoc idea text (mutually exclusive with IdeaID)
	IdeaID      string // Backlog item ID (mutually exclusive with IdeaText)
	AgentName   string // Optional agent override
	ChooseAgent bool   // Show interactive picker to choose agent
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

	ChooseAgent bool   // Show interactive picker to choose agent
	LaunchDir   string // Working directory for interactive agent launch
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
	Tier                 string // Tier to use for decomposition provider call
	Force                bool   // Re-decompose even if already done
	Review               bool   // Return proposed beads for review before creating
	SkipValidation       bool   // Skip validation checks on decomposed bead candidates
	MaxValidationRetries int    // Max retries after validation failures
	MaxSubBeads          int    // Max sub-beads to allow (<=0 disables max-size enforcement)
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
	Attempts                           int  `json:"attempts"`                               // Total provider invocations (1 = no retries)
	ViolationCount                     int  `json:"violation_count"`                        // Total violations found across all attempts
	Improved                           bool `json:"improved"`                               // True if retry reduced violations vs first attempt
	RetryCapReached                    bool `json:"retry_cap_reached"`                      // True when loop exits after exhausting configured retries
	SucceededAfterRetry                bool `json:"succeeded_after_retry"`                  // True when a retry attempt converges before hitting retry cap
	NonImprovingAtRetryCap             bool `json:"non_improving_at_retry_cap"`             // True when retry cap is reached and no improvement is observed
	ProceededWithHighComplexityWarning bool `json:"proceeded_with_high_complexity_warning"` // True when loop exits with remaining high-complexity beads
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
// It owns the lifecycle of the temp prompt file to support async mode.
type PlanSession struct {
	Session
	cleanup func()
}

// NewPlanSession creates a PlanSession that owns the given cleanup function.
func NewPlanSession(cleanup func()) *PlanSession {
	return &PlanSession{
		cleanup: cleanup,
	}
}

// Cleanup removes the temp file associated with this session.
// Safe to call multiple times - subsequent calls are no-ops.
func (ps *PlanSession) Cleanup() {
	if ps.cleanup != nil {
		ps.cleanup()
		ps.cleanup = nil // Prevent double cleanup
	}
}

// ReviewSession is a typed wrapper for interactive Review sessions.
// It owns the lifecycle of the temp prompt file to support async mode.
type ReviewSession struct {
	Session
	cleanup func()
}

// NewReviewSession creates a ReviewSession that owns the given cleanup function.
func NewReviewSession(cleanup func()) *ReviewSession {
	return &ReviewSession{
		cleanup: cleanup,
	}
}

// Cleanup removes the temp file associated with this session.
// Safe to call multiple times - subsequent calls are no-ops.
func (rs *ReviewSession) Cleanup() {
	if rs.cleanup != nil {
		rs.cleanup()
		rs.cleanup = nil // Prevent double cleanup
	}
}

// ExploreSession is a typed wrapper for interactive Explore sessions.
type ExploreSession struct {
	Session
}

// ListBeadsInput contains parameters for listing beads.
type ListBeadsInput struct {
	Status string // Filter by bead status (e.g., "ready", "done")
}

// ListBeadsResult contains the output from listing beads.
// Use NewListBeadsResult() to create instances with properly initialized slices.
type ListBeadsResult struct {
	BeadIDs []string `json:"bead_ids"`
}

// NewListBeadsResult creates a ListBeadsResult with initialized slices.
func NewListBeadsResult() ListBeadsResult {
	return ListBeadsResult{
		BeadIDs: []string{},
	}
}

// QueryBeadsInput contains parameters for querying beads.
type QueryBeadsInput struct {
	StatusFilter string // Filter beads by status
}

// QueryBeadsResult contains the output from querying beads.
// Use NewQueryBeadsResult() to create instances with properly initialized slices.
type QueryBeadsResult struct {
	Beads []BeadInfo `json:"beads"`
}

// NewQueryBeadsResult creates a QueryBeadsResult with initialized slices.
func NewQueryBeadsResult() QueryBeadsResult {
	return QueryBeadsResult{
		Beads: []BeadInfo{},
	}
}

// CountBeadsInput contains parameters for counting beads.
type CountBeadsInput struct {
	Status string // Status to count (e.g., "ready", "done")
}

// CountBeadsResult contains the output from counting beads.
type CountBeadsResult struct {
	Count int `json:"count"`
}

// GetBoardInput contains parameters for assembling board data.
type GetBoardInput struct {
	// Placeholder for future filters if needed
}

// BoardData contains beads organized by status for display.
// Use NewBoardData() to create instances with properly initialized slices.
type BoardData struct {
	Open   []BeadInfo `json:"open"`
	Closed []BeadInfo `json:"closed"`
}

// NewBoardData creates a BoardData with initialized slices.
func NewBoardData() BoardData {
	return BoardData{
		Open:   []BeadInfo{},
		Closed: []BeadInfo{},
	}
}

// GetQueueInput contains parameters for assembling queue data.
type GetQueueInput struct {
	ReadyBeads     []BeadInfo `json:"ready_beads"`
	AllBeads       []BeadInfo `json:"all_beads"`
	StuckThreshold int        `json:"stuck_threshold"`
}

// QueuePartition contains beads organized by processing status.
// Use NewQueuePartition() to create instances with properly initialized slices.
type QueuePartition struct {
	Ready   []BeadInfo `json:"ready"`
	Blocked []BeadInfo `json:"blocked"`
	Stuck   []BeadInfo `json:"stuck"`
}

// NewQueuePartition creates a QueuePartition with initialized slices.
func NewQueuePartition() QueuePartition {
	return QueuePartition{
		Ready:   []BeadInfo{},
		Blocked: []BeadInfo{},
		Stuck:   []BeadInfo{},
	}
}
