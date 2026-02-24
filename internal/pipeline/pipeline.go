package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/review"
)

const (
	backlogTypeReviewFinding = "review-finding"
	logTypeReview            = "review"
)

// Paths contains filesystem paths used by the pipeline.
type Paths struct {
	GromitDir string
	SpecsDir  string
	PlansDir  string
	EpicsDir  string
}

// LLMRunResult holds the fields the pipeline needs from an LLM invocation.
type LLMRunResult struct {
	Success  bool
	ExitCode int
	Output   string
}

// BeadInfo holds the fields the pipeline needs from a bead operation.
type BeadInfo struct {
	ID       string
	Title    string
	Priority int
	Labels   []string
}

// RefinePromptInput holds the fields needed for rendering refine prompts.
type RefinePromptInput struct {
	IdeaText string
}

// PlanPromptInput holds the fields needed for rendering plan prompts.
type PlanPromptInput struct {
	IdeaText string
}

// DecomposePromptInput holds the fields needed for rendering decompose prompts.
type DecomposePromptInput struct {
	PlanName string
	PlanBody string
}

// ExplorePromptInput holds the fields needed for rendering explore prompts.
type ExplorePromptInput struct {
	Query string
}

// ThoroughReviewPromptInput holds the fields needed for rendering thorough review prompts.
type ThoroughReviewPromptInput struct {
	FromCommit string
	Diff       string
}

// LogEntry holds the fields for a log entry.
type LogEntry struct {
	Type           string
	BeadID         string
	Passed         bool
	FixesApplied   int
	BeadsCreated   int
	BacklogCreated int
	Model          string
}

// Deps contains all dependencies for pipeline workflows.
type Deps struct {
	AgentResolver     AgentResolver
	LLMClient         LLMClient
	ReviewInvoker     ReviewInvoker
	BeadClient        BeadClient
	BacklogClient     BacklogClient
	RefineRenderer    RefineRenderer
	PlanRenderer      PlanRenderer
	DecomposeRenderer DecomposeRenderer
	ReviewRenderer    ReviewRenderer
	ExploreRenderer   ExploreRenderer
	LearningsManager  LearningsManager
	StateManager      StateManager
	LogWriter         LogWriter
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
	LaunchInDir(promptPath, dir string) error
}

// LLMClient abstracts LLM CLI operations for non-interactive workflows.
type LLMClient interface {
	Run(prompt string, model string) (*LLMRunResult, error)
}

// ReviewInvoker abstracts non-interactive review invocation.
type ReviewInvoker interface {
	Run(prompt string, model string) (*LLMRunResult, error)
}

// BeadClient abstracts bead (bd) CLI operations.
type BeadClient interface {
	Ready() (*BeadInfo, error)
	Show(id string) (*BeadInfo, error)
	Create(title string, priority int, labels []string, outputs []string) (*BeadInfo, error)
	CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error)
	Close(id string) error
}

// BacklogClient abstracts backlog operations.
type BacklogClient interface {
	List() ([]*Idea, error)
	Get(id string) (*Idea, error)
	Add(item *Idea) error
	Update(id string, fn func(*Idea)) error
}

// Idea represents a backlog idea.
type Idea = backlog.Idea

// IdeaJSONKeys defines the expected snake_case JSON keys for Idea.
var IdeaJSONKeys = []string{"id", "text", "type", "context", "status", "spec_name"}

// RefineRenderer abstracts refine prompt rendering operations.
type RefineRenderer interface {
	RenderRefine(input *RefinePromptInput) (string, error)
}

// PlanRenderer abstracts plan prompt rendering operations.
type PlanRenderer interface {
	RenderPlan(input *PlanPromptInput) (string, error)
}

// DecomposeRenderer abstracts decompose prompt rendering operations.
type DecomposeRenderer interface {
	RenderDecompose(input *DecomposePromptInput) (string, error)
}

// ReviewRenderer abstracts review prompt rendering operations.
type ReviewRenderer interface {
	RenderThoroughReview(input *ThoroughReviewPromptInput) (string, error)
}

// ExploreRenderer abstracts explore prompt rendering operations.
type ExploreRenderer interface {
	RenderExplore(input *ExplorePromptInput) (string, error)
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
	Write(entry *LogEntry) error
}

// Plan executes the plan workflow.
func (p *Pipeline) Plan(ctx context.Context, input PlanInput) (*PlanSession, error) {
	if p.deps == nil || p.deps.AgentResolver == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}
	// TODO: implement
	return nil, fmt.Errorf("pipeline: Plan not yet implemented")
}

// ReviewInteractive executes the interactive review workflow.
// It validates deps, builds ThoroughReviewContext, renders prompt, writes temp file,
// resolves agent, gets command, and returns ReviewSession with no post-processing.
func (p *Pipeline) ReviewInteractive(ctx context.Context, input ReviewInput) (*ReviewSession, error) {
	if p.deps == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}

	if err := validateRequiredDeps([]namedDependency{
		{name: "AgentResolver", dep: p.deps.AgentResolver},
		{name: "ReviewRenderer", dep: p.deps.ReviewRenderer},
	}); err != nil {
		return nil, err
	}

	renderedPrompt, err := p.renderReviewPrompt(input)
	if err != nil {
		return nil, err
	}

	// Write temp file
	tmpDir := filepath.Join(p.paths.GromitDir, "tmp")
	promptPath, cleanup, err := writeTempPromptWithPattern(tmpDir, "review-prompt-*.md", renderedPrompt)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// Resolve agent
	agent, err := p.deps.AgentResolver.Resolve("review", input.AgentName, false)
	if err != nil {
		return nil, fmt.Errorf("resolving agent: %w", err)
	}

	// Launch agent and return session
	// For now, we launch synchronously and return a simple session wrapper
	// TODO: implement actual async session management
	if err := agent.LaunchInDir(promptPath, input.LaunchDir); err != nil {
		return nil, fmt.Errorf("launching agent: %w", err)
	}

	// Return empty session wrapper (interactive mode has no post-processing)
	return &ReviewSession{}, nil
}

// ReviewNonInteractive executes the non-interactive review workflow.
// It validates deps, builds and renders prompt, calls ReviewInvoker.Run with timeout,
// parses result via review.ParseReviewResult, creates beads from findings with from-review label,
// creates backlog items with from-review and backlog labels, persists learnings, logs review,
// updates state, and returns ReviewResult.
func (p *Pipeline) ReviewNonInteractive(ctx context.Context, input ReviewInput) (*ReviewResult, error) {
	if err := p.validateReviewDeps(); err != nil {
		return nil, err
	}

	renderedPrompt, err := p.renderReviewPrompt(input)
	if err != nil {
		return nil, err
	}

	// Invoke the configured non-interactive review runner.
	invocationResult, err := p.deps.ReviewInvoker.Run(renderedPrompt, input.Model)
	if err != nil {
		return nil, fmt.Errorf("review invocation: %w", err)
	}

	// Check invocation result.
	if !invocationResult.Success {
		return nil, fmt.Errorf("Claude invocation failed (exit code %d)\nOutput:\n%s", invocationResult.ExitCode, invocationResult.Output)
	}
	output := invocationResult.Output

	// Parse result via review.ParseReviewResult
	reviewResult, err := review.ParseReviewResult(output)
	if err != nil {
		return nil, fmt.Errorf("parsing review result: %w", err)
	}

	// Create beads from findings with from-review label
	beadsCreated := 0
	for _, bp := range reviewResult.BeadsToCreate {
		labels := BuildFromReviewLabels(bp.Labels)
		_, err := p.deps.BeadClient.Create(bp.Title, bp.Priority, labels, expectedOutputsOrTitle(bp.ExpectedOutputs, bp.Title))
		if err != nil {
			return nil, fmt.Errorf("creating review bead: %w", err)
		}
		beadsCreated++
	}

	// Create backlog items with from-review and backlog labels
	backlogCreated := 0
	for _, bi := range reviewResult.BacklogItems {
		idea := &Idea{
			Text: bi.Title,
			Type: backlogTypeReviewFinding,
		}
		if err := p.deps.BacklogClient.Add(idea); err != nil {
			return nil, fmt.Errorf("creating backlog item: %w", err)
		}
		backlogCreated++
	}

	// Persist learnings
	for _, learning := range reviewResult.Learnings {
		if err := p.deps.LearningsManager.Add(learning); err != nil {
			return nil, fmt.Errorf("persisting learning: %w", err)
		}
	}

	// Log review
	logEntry := &LogEntry{
		Type:           logTypeReview,
		Passed:         reviewResult.Passed,
		FixesApplied:   len(reviewResult.FixesApplied),
		BeadsCreated:   beadsCreated,
		BacklogCreated: backlogCreated,
		Model:          input.Model,
	}
	if err := p.deps.LogWriter.Write(logEntry); err != nil {
		return nil, fmt.Errorf("writing review log: %w", err)
	}

	// Update state - StateManager.SetLastReviewCommit gets current HEAD internally
	if err := p.deps.StateManager.SetLastReviewCommit(input.FromCommit); err != nil {
		return nil, fmt.Errorf("updating state: %w", err)
	}

	// Build and return ReviewResult
	result := NewReviewResult()
	result.Passed = reviewResult.Passed
	result.Summary = reviewResult.Summary
	result.FixesApplied = len(reviewResult.FixesApplied)
	result.BeadsCreated = beadsCreated
	result.BacklogCreated = backlogCreated

	return &result, nil
}

func expectedOutputsOrTitle(outputs []string, title string) []string {
	if len(outputs) > 0 {
		return outputs
	}
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return []string{}
	}
	return []string{trimmedTitle}
}

func (p *Pipeline) renderReviewPrompt(input ReviewInput) (string, error) {
	reviewCtx := &ThoroughReviewPromptInput{
		FromCommit: input.FromCommit,
		Diff:       input.Diff,
	}

	renderedPrompt, err := p.deps.ReviewRenderer.RenderThoroughReview(reviewCtx)
	if err != nil {
		return "", fmt.Errorf("rendering review prompt: %w", err)
	}
	return renderedPrompt, nil
}

// validateReviewDeps checks that all required dependencies for ReviewNonInteractive are present.
func (p *Pipeline) validateReviewDeps() error {
	if p.deps == nil {
		return fmt.Errorf("pipeline: nil dependencies")
	}

	return validateRequiredDeps([]namedDependency{
		{name: "ReviewInvoker", dep: p.deps.ReviewInvoker},
		{name: "ReviewRenderer", dep: p.deps.ReviewRenderer},
		{name: "BeadClient", dep: p.deps.BeadClient},
		{name: "BacklogClient", dep: p.deps.BacklogClient},
		{name: "LearningsManager", dep: p.deps.LearningsManager},
		{name: "LogWriter", dep: p.deps.LogWriter},
		{name: "StateManager", dep: p.deps.StateManager},
	})
}

func requireNonNilDep(name string, dep any) error {
	if dep == nil || isTypedNil(dep) {
		return fmt.Errorf("pipeline: nil %s", name)
	}
	return nil
}

func isTypedNil(value any) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

type namedDependency struct {
	name string
	dep  any
}

func validateRequiredDeps(deps []namedDependency) error {
	for _, dep := range deps {
		if err := requireNonNilDep(dep.name, dep.dep); err != nil {
			return err
		}
	}
	return nil
}
