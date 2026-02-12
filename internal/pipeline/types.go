package pipeline

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
	CreatedSpecs  []string `json:"created_specs"`
	RefinedItems  []string `json:"refined_items"`
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
	PlanName string // Name of plan to decompose
	Force    bool   // Re-decompose even if already done
	Review   bool   // Return proposed beads for review before creating
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

// DecomposeResult contains the output from the Decompose workflow.
// Use NewDecomposeResult() to create instances with properly initialized slices.
type DecomposeResult struct {
	CreatedBeads []CreatedBead `json:"created_beads"`
	PlanUpdated  bool          `json:"plan_updated"`
}

// NewDecomposeResult creates a DecomposeResult with initialized slices.
func NewDecomposeResult() DecomposeResult {
	return DecomposeResult{
		CreatedBeads: []CreatedBead{},
	}
}

// ReviewInput contains parameters for the Review workflow.
type ReviewInput struct {
	Since     string // Git commit ref to review since
	Spec      string // Spec label to scope review
	Epic      string // Epic label to scope review
	AgentName string // Optional agent override
}

// ReviewResult contains the output from the Review workflow.
// Use NewReviewResult() to create instances with properly initialized slices.
type ReviewResult struct {
	CreatedBeads        []string `json:"created_beads"`
	CreatedBacklogItems []string `json:"created_backlog_items"`
	PersistedLearnings  bool     `json:"persisted_learnings"`
}

// NewReviewResult creates a ReviewResult with initialized slices.
func NewReviewResult() ReviewResult {
	return ReviewResult{
		CreatedBeads:        []string{},
		CreatedBacklogItems: []string{},
	}
}

// ExploreInput contains parameters for the Explore workflow.
type ExploreInput struct {
	Topic string // Topic to explore
	Model string // Model to use for exploration
}

// ExploreResult contains the output from the Explore workflow.
// Use NewExploreResult() to create instances with properly initialized slices.
type ExploreResult struct {
	CreatedSpecs        []string `json:"created_specs"`
	CreatedEpics        []string `json:"created_epics"`
	CreatedBacklogItems []string `json:"created_backlog_items"`
}

// NewExploreResult creates a ExploreResult with initialized slices.
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
