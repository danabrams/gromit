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
type RefineResult struct {
	CreatedSpecs  []string // Paths to new spec files
	RefinedItems  []string // Backlog item IDs marked as refined
}

// NewRefineResult creates a RefineResult with initialized slices.
func NewRefineResult() *RefineResult {
	return &RefineResult{
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
type PlanResult struct {
	CreatedPlans []string // Paths to new plan files
}

// DecomposeInput contains parameters for the Decompose workflow.
type DecomposeInput struct {
	PlanName string // Name of plan to decompose
	Force    bool   // Re-decompose even if already done
	Review   bool   // Return proposed beads for review before creating
}

// CreatedBead contains information about a bead that was created.
type CreatedBead struct {
	ID       string
	Title    string
	Priority int
	Labels   []string
}

// DecomposeResult contains the output from the Decompose workflow.
type DecomposeResult struct {
	CreatedBeads []CreatedBead // Beads that were created
	PlanUpdated  bool          // Whether plan frontmatter was updated
}

// ReviewInput contains parameters for the Review workflow.
type ReviewInput struct {
	Since     string // Git commit ref to review since
	Spec      string // Spec label to scope review
	Epic      string // Epic label to scope review
	AgentName string // Optional agent override
}

// ReviewResult contains the output from the Review workflow.
type ReviewResult struct {
	CreatedBeads        []string // Bead IDs created from review
	CreatedBacklogItems []string // Backlog item IDs created from review
	PersistedLearnings  bool     // Whether learnings were persisted
}

// ExploreInput contains parameters for the Explore workflow.
type ExploreInput struct {
	Topic string // Topic to explore
	Model string // Model to use for exploration
}

// ExploreResult contains the output from the Explore workflow.
type ExploreResult struct {
	CreatedSpecs        []string // Paths to new spec files
	CreatedEpics        []string // Paths to new epic files
	CreatedBacklogItems []string // Backlog item IDs created
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
