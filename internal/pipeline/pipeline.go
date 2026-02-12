package pipeline

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
	Resolve(phase string, flagOverride string, choosePicker bool) (interface{}, error)
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
	List() ([]interface{}, error)
	Get(id string) (interface{}, error)
	Add(item interface{}) error
	Update(id string, fn func(interface{})) error
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
