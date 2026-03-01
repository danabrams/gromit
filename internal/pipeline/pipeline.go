package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/frontmatter"
	"github.com/danabrams/gromit/internal/review"
)

const (
	backlogTypeReviewFinding = "review-finding"
	logTypeReview            = "review"
	backlogPriorityDefault   = 2
	thoroughReviewPhase      = "thorough_review"
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
	ClaudeMD   string
	Rules      string
}

type promptContextLoader interface {
	LoadClaudeMD() (string, error)
	LoadRulesForPhase(phase string) (string, error)
}

func NewThoroughReviewPromptInput(loader promptContextLoader, phase, fromCommit, diff string) *ThoroughReviewPromptInput {
	claudeMD, err := loader.LoadClaudeMD()
	if err != nil {
		claudeMD = ""
	}

	rules, err := loader.LoadRulesForPhase(phase)
	if err != nil {
		rules = ""
	}

	return &ThoroughReviewPromptInput{
		FromCommit: fromCommit,
		Diff:       diff,
		ClaudeMD:   claudeMD,
		Rules:      rules,
	}
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
	TrackerClient     TrackerClient
	BeadQueryClient   BeadQueryClient
	BacklogClient     BacklogClient
	BacklogWriter     BacklogWriter
	RefineRenderer    RefineRenderer
	PlanRenderer      PlanRenderer
	DecomposeRenderer DecomposeRenderer
	ReviewRenderer    ReviewRenderer
	ExploreRenderer   ExploreRenderer
	LearningsManager  LearningsManager
	StateManager      StateManager
	LogWriter         LogWriter
	ModelForwarder    func(agent Agent, model string) (Agent, string)
	WarningWriter     func(warning string)
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

// TrackerClient abstracts tracker-compatible bead operations.
type TrackerClient interface {
	Ready(ctx context.Context) (*BeadInfo, error)
	Show(ctx context.Context, id string) (*BeadInfo, error)
	Create(ctx context.Context, title string, priority int, labels []string, outputs []string) (*BeadInfo, error)
	CreateWithDepsAndDescription(ctx context.Context, title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error)
	Close(ctx context.Context, id string) error
	ListWithLabel(ctx context.Context, label string) ([]string, error) // Returns slice of bead IDs with the given label
}

// BeadQueryClient abstracts bead query and status operations.
type BeadQueryClient interface {
	CountByStatus(ctx context.Context, status string) (int, error)
	ListReadyIDs(ctx context.Context) ([]string, error)
	CountClosedAfter(ctx context.Context, after time.Time) (int, error)
}

// BacklogClient abstracts backlog operations.
type BacklogClient interface {
	List() ([]*Idea, error)
	Get(id string) (*Idea, error)
	Add(item *Idea) error
	Update(id string, fn func(*Idea)) error
}

// BacklogWriter abstracts write-only backlog operations.
type BacklogWriter interface {
	Add(ctx context.Context, entry *BacklogEntry) error
	Update(id string, fn func(*Idea)) error
}

// BacklogEntry describes the data required to create a backlog item.
type BacklogEntry struct {
	Title           string
	Type            string
	Description     string
	Priority        int
	Labels          []string
	ExpectedOutputs []string
}

// Idea is the canonical backlog idea type.
type Idea = backlog.Idea

// IdeaJSONKeys defines the expected snake_case JSON keys for Idea.
var IdeaJSONKeys = []string{"id", "text", "type", "context", "created_at", "status", "spec_name"}

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
	LoadClaudeMD() (string, error)
	LoadRulesForPhase(phase string) (string, error)
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
// It validates deps, renders prompt, writes temp file, resolves agent, and returns PlanSession.
func (p *Pipeline) Plan(ctx context.Context, input PlanInput) (*PlanSession, error) {
	return runPlanWorkflow(ctx, p, input)
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
	// Don't defer cleanup - session owns it now (safe for async mode)

	// Resolve agent
	agent, err := p.deps.AgentResolver.Resolve("review", input.AgentName, false)
	if err != nil {
		cleanup() // Clean up on error before returning
		return nil, fmt.Errorf("resolving agent: %w", err)
	}

	// Launch agent and return session
	// For now, we launch synchronously and return a session wrapper that owns cleanup
	// TODO: implement actual async session management
	if err := agent.LaunchInDir(promptPath, input.LaunchDir); err != nil {
		cleanup() // Clean up on error before returning
		return nil, fmt.Errorf("launching agent: %w", err)
	}

	// Return session that owns the cleanup function
	// Caller must call session.Cleanup() to remove the temp file
	return NewReviewSession(cleanup), nil
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
		labels := review.BuildReviewBeadLabels(bp.Labels)
		_, err := p.deps.TrackerClient.Create(ctx, bp.Title, bp.Priority, labels, review.ExpectedOutputsOrTitle(bp.ExpectedOutputs, bp.Title))
		if err != nil {
			return nil, fmt.Errorf("creating review bead: %w", err)
		}
		beadsCreated++
	}

	// Create backlog items with from-review and backlog labels
	backlogCreated := 0
	for _, bi := range reviewResult.BacklogItems {
		description := bi.Description
		if bi.Reason != "" {
			if description != "" {
				description += "\n\n"
			}
			description += "Reason for backlog: " + bi.Reason
		}

		entry := &BacklogEntry{
			Title:           bi.Title,
			Type:            backlogTypeReviewFinding,
			Description:     description,
			Priority:        backlogPriorityDefault,
			Labels:          review.BuildBacklogLabels(),
			ExpectedOutputs: review.ExpectedOutputsOrTitle(bi.ExpectedOutputs, bi.Title),
		}
		if err := p.deps.BacklogWriter.Add(ctx, entry); err != nil {
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

func (p *Pipeline) renderReviewPrompt(input ReviewInput) (string, error) {
	reviewCtx := NewThoroughReviewPromptInput(p.deps.ReviewRenderer, thoroughReviewPhase, input.FromCommit, input.Diff)

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
		{name: "TrackerClient", dep: p.deps.TrackerClient},
		{name: "BacklogWriter", dep: p.deps.BacklogWriter},
		{name: "LearningsManager", dep: p.deps.LearningsManager},
		{name: "LogWriter", dep: p.deps.LogWriter},
		{name: "StateManager", dep: p.deps.StateManager},
	})
}

// ResolveReviewScope resolves the starting commit for a review based on scope flags.
// Priority: --since > --spec > --epic > state file
func (p *Pipeline) ResolveReviewScope(ctx context.Context, spec string, epic string, since string) (string, error) {
	if p.deps == nil {
		return "", fmt.Errorf("pipeline: nil dependencies")
	}

	// Priority: --since flag first
	if since != "" {
		return since, nil
	}

	// --spec flag: resolve from spec beads
	if spec != "" {
		if err := requireNonNilDep("TrackerClient", p.deps.TrackerClient); err != nil {
			return "", err
		}
		return resolveSpecScope(ctx, spec, p.deps.TrackerClient)
	}

	// --epic flag: resolve from epic specs
	if epic != "" {
		if err := requireNonNilDep("TrackerClient", p.deps.TrackerClient); err != nil {
			return "", err
		}
		return resolveEpicScope(ctx, epic, p.paths.SpecsDir, p.deps.TrackerClient)
	}

	// No flags provided - fall back to state file via StateManager
	if err := requireNonNilDep("StateManager", p.deps.StateManager); err != nil {
		return "", err
	}

	commit, err := p.deps.StateManager.GetLastReviewCommit()
	if err != nil {
		return "", fmt.Errorf("getting last review commit from state: %w", err)
	}

	if commit == "" {
		return "", fmt.Errorf("no previous review found - use --since to specify a commit")
	}

	return commit, nil
}

// ListBeads lists beads matching the given query criteria.
// It validates deps, queries the BeadQueryClient, and returns ListBeadsResult.
func (p *Pipeline) ListBeads(ctx context.Context, input ListBeadsInput) (*ListBeadsResult, error) {
	if p.deps == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}

	if err := requireNonNilDep("BeadQueryClient", p.deps.BeadQueryClient); err != nil {
		return nil, err
	}

	if input.Status != "" && input.Status != "ready" {
		return nil, fmt.Errorf("pipeline: unsupported bead status %q", input.Status)
	}

	result := NewListBeadsResult()

	// ListReadyIDs is the only list method available on BeadQueryClient.
	// When status is "ready" (or empty, defaulting to ready), use it directly.
	// For other statuses, return empty result since the interface doesn't support listing by arbitrary status.
	if input.Status == "" || input.Status == "ready" {
		ids, err := p.deps.BeadQueryClient.ListReadyIDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing ready beads: %w", err)
		}
		result.BeadIDs = ids
	}

	return &result, nil
}

// QueryBeads queries beads based on filter criteria.
// It validates deps, queries the BeadQueryClient, and returns QueryBeadsResult.
func (p *Pipeline) QueryBeads(ctx context.Context, input QueryBeadsInput) (*QueryBeadsResult, error) {
	if p.deps == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}

	if err := requireNonNilDep("BeadQueryClient", p.deps.BeadQueryClient); err != nil {
		return nil, err
	}

	if input.StatusFilter != "" && input.StatusFilter != "ready" {
		return nil, fmt.Errorf("pipeline: unsupported bead status %q", input.StatusFilter)
	}

	result := NewQueryBeadsResult()

	// QueryBeads returns bead metadata. The BeadQueryClient only supports
	// listing ready IDs, so we list ready IDs and return them as BeadInfo
	// when the status filter matches (or is empty).
	if input.StatusFilter == "" || input.StatusFilter == "ready" {
		ids, err := p.deps.BeadQueryClient.ListReadyIDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("querying beads: %w", err)
		}
		beads := make([]BeadInfo, len(ids))
		for i, id := range ids {
			beads[i] = BeadInfo{ID: id}
		}
		result.Beads = beads
	}

	return &result, nil
}

// CountBeads counts beads with a given status.
// It validates deps, queries the BeadQueryClient, and returns CountBeadsResult.
func (p *Pipeline) CountBeads(ctx context.Context, input CountBeadsInput) (*CountBeadsResult, error) {
	if p.deps == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}

	if err := requireNonNilDep("BeadQueryClient", p.deps.BeadQueryClient); err != nil {
		return nil, err
	}

	count, err := p.deps.BeadQueryClient.CountByStatus(ctx, input.Status)
	if err != nil {
		return nil, fmt.Errorf("counting beads by status %q: %w", input.Status, err)
	}

	result := &CountBeadsResult{Count: count}
	return result, nil
}

// QueryUndecomposedPlans queries for undecomposed plans in the plans directory.
// It validates deps and returns QueryUndecomposedPlansResult.
func (p *Pipeline) QueryUndecomposedPlans(ctx context.Context, input QueryUndecomposedPlansInput) (*QueryUndecomposedPlansResult, error) {
	if p.deps == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}

	if err := requireNonNilDep("TrackerClient", p.deps.TrackerClient); err != nil {
		return nil, err
	}

	result := NewQueryUndecomposedPlansResult()

	plansDir := p.paths.PlansDir
	if plansDir == "" {
		return &result, nil
	}

	// Read directory
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &result, nil
		}
		return nil, fmt.Errorf("reading plans directory: %w", err)
	}

	var plans []PlanQueryInfo
	for _, entry := range entries {
		// Skip directories and non-.md files
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		planPath := filepath.Join(plansDir, entry.Name())
		planName := strings.TrimSuffix(entry.Name(), ".md")

		// Read frontmatter to check decomposed status
		planFrontmatter, _, err := frontmatter.ReadFile(planPath)
		if err != nil {
			// Skip files that can't be read
			continue
		}

		// Filter by decomposed status (unless force is true)
		if !input.Force {
			if decomposed, ok := planFrontmatter["decomposed"].(bool); ok && decomposed {
				continue
			}
		}

		// Extract title from plan file
		title := extractPlanTitle(planPath)
		if title == "" {
			title = planName // Fallback to name if no title found
		}

		plans = append(plans, PlanQueryInfo{
			Name:  planName,
			Title: title,
			Path:  planPath,
		})
	}

	// Sort by name for consistent ordering
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].Name < plans[j].Name
	})

	result.Plans = plans
	return &result, nil
}

// extractPlanTitle extracts the first H1 heading from a plan file.
// Returns empty string if no heading found.
func extractPlanTitle(planPath string) string {
	content, err := os.ReadFile(planPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}

	return ""
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
