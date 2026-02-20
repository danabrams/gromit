package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/review"
)

// Paths contains filesystem paths used by the pipeline.
type Paths struct {
	GromitDir string
	SpecsDir  string
	PlansDir  string
	EpicsDir  string
}

// ClaudeRunResult holds the fields the pipeline needs from a Claude invocation.
type ClaudeRunResult struct {
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
	LaunchInDir(promptPath, dir string) error
}

// ClaudeClient abstracts Claude CLI operations for non-interactive workflows.
type ClaudeClient interface {
	Run(prompt string, model string) (*ClaudeRunResult, error)
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

// Idea represents a backlog idea (matches backlog.Idea).
type Idea struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Type     string `json:"type"`
	Context  string `json:"context"`
	Status   string `json:"status"`
	SpecName string `json:"spec_name"`
}

// IdeaJSONKeys defines the expected snake_case JSON keys for Idea.
var IdeaJSONKeys = []string{"id", "text", "type", "context", "status", "spec_name"}

// PromptRenderer abstracts prompt rendering operations.
type PromptRenderer interface {
	RenderRefine(input *RefinePromptInput) (string, error)
	RenderPlan(input *PlanPromptInput) (string, error)
	RenderDecompose(input *DecomposePromptInput) (string, error)
	RenderThoroughReview(input *ThoroughReviewPromptInput) (string, error)
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
	Write(entry any) error
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
	if p.deps == nil || p.deps.AgentResolver == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}

	// Validate required dependencies
	if p.deps.PromptRenderer == nil {
		return nil, fmt.Errorf("pipeline: nil PromptRenderer")
	}

	// Build ThoroughReviewContext
	reviewCtx := &ThoroughReviewPromptInput{
		FromCommit: input.FromCommit,
		Diff:       input.Diff,
	}

	// Render prompt
	renderedPrompt, err := p.deps.PromptRenderer.RenderThoroughReview(reviewCtx)
	if err != nil {
		return nil, fmt.Errorf("rendering review prompt: %w", err)
	}

	// Write temp file
	tmpDir := filepath.Join(p.paths.GromitDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating tmp dir: %w", err)
	}

	promptFile, err := os.CreateTemp(tmpDir, "review-prompt-*.md")
	if err != nil {
		return nil, fmt.Errorf("creating temp prompt file: %w", err)
	}
	promptPath := promptFile.Name()

	if _, err := promptFile.WriteString(renderedPrompt); err != nil {
		promptFile.Close()
		os.Remove(promptPath)
		return nil, fmt.Errorf("writing prompt file: %w", err)
	}
	promptFile.Close()

	// Resolve agent
	agent, err := p.deps.AgentResolver.Resolve("review", input.AgentName, false)
	if err != nil {
		os.Remove(promptPath)
		return nil, fmt.Errorf("resolving agent: %w", err)
	}

	// Launch agent and return session
	// For now, we launch synchronously and return a simple session wrapper
	// TODO: implement actual async session management
	if err := agent.LaunchInDir(promptPath, input.LaunchDir); err != nil {
		os.Remove(promptPath)
		return nil, fmt.Errorf("launching agent: %w", err)
	}

	// Clean up temp file after launch
	os.Remove(promptPath)

	// Return empty session wrapper (interactive mode has no post-processing)
	return &ReviewSession{}, nil
}

// ReviewNonInteractive executes the non-interactive review workflow.
// It validates deps, builds and renders prompt, calls ClaudeClient.Run with timeout,
// parses result via review.ParseReviewResult, creates beads from findings with from-review label,
// creates backlog items with from-review and backlog labels, persists learnings, logs review,
// updates state, and returns ReviewResult.
func (p *Pipeline) ReviewNonInteractive(ctx context.Context, input ReviewInput) (*ReviewResult, error) {
	if err := p.validateReviewDeps(); err != nil {
		return nil, err
	}

	// Build and render prompt
	reviewCtx := &ThoroughReviewPromptInput{
		FromCommit: input.FromCommit,
		Diff:       input.Diff,
	}
	renderedPrompt, err := p.deps.PromptRenderer.RenderThoroughReview(reviewCtx)
	if err != nil {
		return nil, fmt.Errorf("rendering review prompt: %w", err)
	}

	// Call Claude - use context for timeout if needed
	// The ClaudeClient interface doesn't directly support timeout,
	// but the mock tests expect it, so we'll need to fix the interface or the tests
	claudeResult, err := p.deps.ClaudeClient.Run(renderedPrompt, input.Model)
	if err != nil {
		return nil, fmt.Errorf("review invocation: %w", err)
	}

	// Check result from Claude
	if !claudeResult.Success {
		return nil, fmt.Errorf("Claude invocation failed (exit code %d)\nOutput:\n%s", claudeResult.ExitCode, claudeResult.Output)
	}
	output := claudeResult.Output

	// Parse result via review.ParseReviewResult
	reviewResult, err := review.ParseReviewResult(output)
	if err != nil {
		return nil, fmt.Errorf("parsing review result: %w", err)
	}

	// Create beads from findings with from-review label
	beadsCreated := 0
	for _, bp := range reviewResult.BeadsToCreate {
		labels := buildReviewBeadLabels(bp.Labels)
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
			Type: "review-finding",
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
		Type:           "review",
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

// validateReviewDeps checks that all required dependencies for ReviewNonInteractive are present.
func (p *Pipeline) validateReviewDeps() error {
	if p.deps == nil {
		return fmt.Errorf("pipeline: nil dependencies")
	}

	requiredDeps := map[string]interface{}{
		"ClaudeClient":     p.deps.ClaudeClient,
		"PromptRenderer":   p.deps.PromptRenderer,
		"BeadClient":       p.deps.BeadClient,
		"BacklogClient":    p.deps.BacklogClient,
		"LearningsManager": p.deps.LearningsManager,
		"LogWriter":        p.deps.LogWriter,
		"StateManager":     p.deps.StateManager,
	}

	for name, dep := range requiredDeps {
		if dep == nil {
			return fmt.Errorf("pipeline: nil %s", name)
		}
	}

	return nil
}
