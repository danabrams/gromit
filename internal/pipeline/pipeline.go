package pipeline

import (
	"context"
	"fmt"
)

// Paths contains filesystem paths used by the pipeline.
type Paths struct {
	GromitDir string
	SpecsDir  string
	PlansDir  string
	EpicsDir  string
}

// Deps contains all dependencies for pipeline workflows.
type Deps struct {
	AgentResolver    AgentResolver
	ClaudeClient     ClaudeClient
	BeadClient       BeadClient
	BacklogClient    BacklogClient
	PromptRenderer   PromptRenderer
	LearningsManager LearningsManager
	StateManager     StateManager
	LogWriter        LogWriter
}

// Pipeline orchestrates workflow execution.
type Pipeline struct {
	deps  *Deps
	paths *Paths
}

// New creates a new Pipeline with the given dependencies and paths.
func New(deps *Deps, paths *Paths) *Pipeline {
	return &Pipeline{
		deps:  deps,
		paths: paths,
	}
}

// AgentResolver abstracts agent resolution for interactive workflows.
type AgentResolver interface {
	Resolve(phase string, flagOverride string, choosePicker bool) (Agent, error)
}

// Agent abstracts the agent interface needed by the pipeline.
// This matches the agent.Agent interface from internal/agent.
type Agent interface {
	Name() string
	Launch(promptPath string) error
}

// ClaudeClient abstracts Claude CLI operations for non-interactive workflows.
type ClaudeClient interface {
	Run(prompt string, model string) (interface{}, error)
}

// BeadClient abstracts bead (bd) CLI operations.
type BeadClient interface {
	Ready() (interface{}, error)
	Show(id string) (interface{}, error)
	Create(title string, priority int, labels []string, outputs []string) (interface{}, error)
	Close(id string) error
}

// BacklogClient abstracts backlog operations.
type BacklogClient interface {
	List() ([]*Idea, error)
	Get(id string) (*Idea, error)
	Add(item *Idea) error
	Update(id string, fn func(*Idea)) error
}

// Idea represents a backlog idea (matches backlog.Idea).
type Idea struct {
	ID        string
	Text      string
	Type      string
	Context   string
	Status    string
	SpecName  string
}

// PromptRenderer abstracts prompt rendering operations.
type PromptRenderer interface {
	RenderRefine(input interface{}) (string, error)
	RenderPlan(input interface{}) (string, error)
	RenderDecompose(input interface{}) (string, error)
}

// LearningsManager abstracts learning persistence operations.
type LearningsManager interface {
	Add(content string) error
}

// StateManager abstracts state persistence operations.
type StateManager interface {
	GetLastReviewCommit() (string, error)
	SetLastReviewCommit(commit string) error
}

// LogWriter abstracts log writing operations.
type LogWriter interface {
	Write(entry interface{}) error
}


// Plan executes the plan workflow.
func (p *Pipeline) Plan(ctx context.Context, input PlanInput) (*PlanSession, error) {
	if p.deps == nil || p.deps.AgentResolver == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}
	// TODO: implement
	return nil, fmt.Errorf("pipeline: Plan not yet implemented")
}

// Decompose executes the decompose workflow.
func (p *Pipeline) Decompose(ctx context.Context, input DecomposeInput) (*DecomposeResult, error) {
	if p.deps == nil || p.deps.ClaudeClient == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}
	// TODO: implement
	return nil, fmt.Errorf("pipeline: Decompose not yet implemented")
}

// Review executes the review workflow.
func (p *Pipeline) Review(ctx context.Context, input ReviewInput) (*ReviewSession, error) {
	if p.deps == nil || p.deps.AgentResolver == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}
	// TODO: implement
	return nil, fmt.Errorf("pipeline: Review not yet implemented")
}

// Explore executes the explore workflow.
func (p *Pipeline) Explore(ctx context.Context, input ExploreInput) (*ExploreSession, error) {
	if p.deps == nil || p.deps.AgentResolver == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}
	// TODO: implement
	return nil, fmt.Errorf("pipeline: Explore not yet implemented")
}
