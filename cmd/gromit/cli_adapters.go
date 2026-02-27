package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/review"
	"github.com/danabrams/gromit/internal/state"
)

// cliPromptRenderer adapts prompt.Renderer to pipeline.ReviewRenderer interface
// It loads ClaudeMD and Rules before rendering
type cliPromptRenderer struct {
	renderer *prompt.Renderer
}

var _ pipeline.ReviewRenderer = (*cliPromptRenderer)(nil)
var _ pipeline.PlanRenderer = (*planPromptRenderer)(nil)

func (r *cliPromptRenderer) RenderThoroughReview(input *pipeline.ThoroughReviewPromptInput) (string, error) {
	// Build ThoroughReviewContext from pipeline input
	reviewCtx := &prompt.ThoroughReviewContext{
		Diff: input.Diff,
	}

	// Load ClaudeMD and Rules (warnings only)
	var err error
	reviewCtx.ClaudeMD, err = r.renderer.LoadClaudeMD()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load CLAUDE.md: %v\n", err)
	}
	reviewCtx.Rules, err = r.renderer.LoadRulesForPhase("thorough_review")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load thorough_review rules: %v\n", err)
	}

	return r.renderer.RenderThoroughReview(reviewCtx)
}

// explorePromptRenderer adapts prompt.Renderer to pipeline.ExploreRenderer
type explorePromptRenderer struct {
	renderer *prompt.Renderer
	// lastDiagnostics captures section-level prompt token estimates for explore prompts.
	lastDiagnostics *prompt.PromptDiagnostics
}

var _ pipeline.ExploreRenderer = (*explorePromptRenderer)(nil)

func (r *explorePromptRenderer) RenderExplore(input *pipeline.ExplorePromptInput) (string, error) {
	// Extract topic from typed input
	topic := input.Query

	// Load ClaudeMD and Rules
	claudeMD, err := r.renderer.LoadClaudeMD()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load CLAUDE.md: %v\n", err)
	}

	rules, err := r.renderer.LoadRules()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load RULES.md: %v\n", err)
	}

	// Load learnings
	lf := r.renderer.GetLearningsFile()
	confirmedLearnings := formatLearnings(lf, true)
	recentLearnings := formatLearnings(lf, false)

	// Get working directory and paths
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not determine working directory: %v\n", err)
		workDir = "."
	}
	gromitDir := r.renderer.GetGromitDir()
	specsDir := r.renderer.GetSpecsDir()

	learningsSection := fmt.Sprintf(`#### Confirmed Patterns

%s

#### Recent Learnings

%s`, confirmedLearnings, recentLearnings)
	instructionsSection := `You are running an exploration session. Your goal is to brainstorm a big idea or problem space and break it down into concrete, actionable artifacts.

### What To Do

1. **Understand the problem space** — If a topic was provided above, start there. Otherwise, ask the user what they want to explore. Read relevant code and docs to ground your understanding.

2. **Brainstorm broadly** — Think through the problem from multiple angles. Consider user needs, technical constraints, edge cases, and alternative approaches. Discuss ideas with the user — this is a collaborative session.

3. **Break it down** — As ideas crystallize, capture them as the appropriate artifact type:

   - **Backlog items** — Quick ideas, rough feature requests, bugs, or chores. Add these by running: gromit add "<idea>". Optionally provide context when prompted. These flow through the refine → plan → decompose pipeline later.
   - **Specs** — For ideas that are well-understood enough to specify, write a spec file to %s/<name>.md. A spec describes what to build, why, acceptance criteria, and key decisions. See existing specs in that directory for the format.
   - **Epics** — For large initiatives that span multiple specs, write an epic file to %s/epics/<name>.md with frontmatter containing epic_id and created fields.

4. **Prefer backlog items** — When in doubt, use gromit add. Backlog items are cheap and get refined later. Only write specs for ideas you've discussed enough to specify clearly. Only create epics when the scope genuinely spans multiple independent specs.

### What NOT To Do

- Do NOT implement features or write production code
- Do NOT create beads with bd create — that happens during decomposition
- Do NOT skip the conversation — explore with the user, don't just dump a list of ideas

### Session Flow

Start by understanding the topic, then alternate between discussing ideas and capturing them. End the session when the problem space feels well-mapped and the key ideas have been captured as artifacts.`
	renderedInstructions := fmt.Sprintf(instructionsSection, specsDir, gromitDir)
	sectionTokens := prompt.EstimateSectionTokens(map[string]string{
		"topic":                topic,
		prompt.SectionClaudeMD: claudeMD,
		prompt.SectionRules:    rules,
		"learnings":            learningsSection,
		"instructions":         renderedInstructions,
	})
	r.lastDiagnostics = prompt.NewDiagnostics("explore", sectionTokens)

	// Build the system prompt
	var sb strings.Builder

	// Optional: pre-seeded topic
	if topic != "" {
		sb.WriteString(fmt.Sprintf(`Topic for exploration:

%s

`, topic))
	}

	// Context section
	sb.WriteString(fmt.Sprintf(`## Context

Working directory: %s
Epics directory: %s/epics
Specs directory: %s

`, workDir, gromitDir, specsDir))

	// Project context
	if claudeMD != "" {
		sb.WriteString(fmt.Sprintf(`### Project Context

%s

`, claudeMD))
	}

	if rules != "" {
		sb.WriteString(fmt.Sprintf(`### Rules (Non-Negotiable)

%s

`, rules))
	}

	// Learnings
	sb.WriteString(fmt.Sprintf(`### Learnings

%s

`, learningsSection))

	// Exploration instructions
	sb.WriteString("## Instructions\n\n")
	sb.WriteString(renderedInstructions)
	sb.WriteString("\n")

	return sb.String(), nil
}

func (r *explorePromptRenderer) LastDiagnostics() *prompt.PromptDiagnostics {
	if r == nil {
		return nil
	}
	return r.lastDiagnostics
}

// planPromptRenderer adapts prompt.Renderer to pipeline.PlanRenderer
type planPromptRenderer struct {
	renderer *prompt.Renderer
}

var _ pipeline.PlanRenderer = (*planPromptRenderer)(nil)

func (r *planPromptRenderer) RenderPlan(input *pipeline.PlanPromptInput) (string, error) {
	specName := ""
	if input != nil {
		specName = input.IdeaText
	}
	if specName == "" {
		return "Plan prompt placeholder", nil
	}
	return fmt.Sprintf("Plan prompt placeholder for %s", specName), nil
}

// refinePromptRenderer adapts prompt.Renderer to pipeline.RefineRenderer interface
type refinePromptRenderer struct {
	renderer *prompt.Renderer
}

var _ pipeline.RefineRenderer = (*refinePromptRenderer)(nil)

func (r *refinePromptRenderer) RenderRefine(input *pipeline.RefinePromptInput) (string, error) {
	ideaText := ""
	if input != nil {
		ideaText = input.IdeaText
	}
	if ideaText == "" {
		return "Refine prompt placeholder", nil
	}
	return fmt.Sprintf("Refine prompt placeholder for %s", ideaText), nil
}

// decomposePromptRenderer adapts prompt.Renderer to pipeline.DecomposeRenderer interface
type decomposePromptRenderer struct {
	renderer *prompt.Renderer
}

var _ pipeline.DecomposeRenderer = (*decomposePromptRenderer)(nil)

func (r *decomposePromptRenderer) RenderDecompose(input *pipeline.DecomposePromptInput) (string, error) {
	if input == nil || input.PlanName == "" {
		return "Decompose prompt placeholder", nil
	}
	return fmt.Sprintf("Decompose prompt placeholder for %s", input.PlanName), nil
}

// cliBacklogClient adapts bead operations to pipeline.BacklogWriter interface
type cliBacklogClient struct {
	beadClient *bead.Client
}

var _ pipeline.BacklogWriter = (*cliBacklogClient)(nil)

func (c *cliBacklogClient) Add(entry *pipeline.BacklogEntry) error {
	if entry == nil {
		return fmt.Errorf("backlog entry is nil")
	}

	labels := entry.Labels
	if len(labels) == 0 {
		labels = review.BuildBacklogLabels()
	}

	expectedOutputs := entry.ExpectedOutputs
	if expectedOutputs == nil {
		expectedOutputs = []string{}
	}

	priority := entry.Priority
	if priority == 0 {
		priority = 2
	}

	_, err := c.beadClient.Create(context.Background(), entry.Title, priority, labels, expectedOutputs)
	return err
}

func (c *cliBacklogClient) Update(id string, fn func(*pipeline.Idea)) error {
	return fmt.Errorf("not implemented")
}

// cliLearningsManager adapts learnings operations to pipeline.LearningsManager interface
type cliLearningsManager struct {
	gromitDir string
	runner    learnings.ClaudeRunner
}

var _ pipeline.LearningsManager = (*cliLearningsManager)(nil)

func (m *cliLearningsManager) Add(content string) error {
	learningsFile, err := learnings.NewFile(m.gromitDir)
	if err != nil {
		return err
	}

	// Wire filter into learnings file
	if m.runner != nil {
		learningsFile.SetFilter(learnings.NewLLMFilter(m.runner, "gromit", learnings.ProjectDescriptions.Gromit))
	}

	if err := learningsFile.Load(); err != nil {
		return err
	}

	_, err = learningsFile.Add(reviewSessionCommand, content, learnings.CategoryPatterns)
	return err
}

// pipelineLearningsRunnerAdapter implements learnings.ClaudeRunner using pipeline.LLMClient.
type pipelineLearningsRunnerAdapter struct {
	client pipeline.LLMClient
}

func (r *pipelineLearningsRunnerAdapter) Run(ctx context.Context, prompt string, model string) (*learnings.Result, error) {
	_ = ctx // pipeline.LLMClient handles its own timeout/context.
	if r == nil || r.client == nil {
		return nil, fmt.Errorf("review invoker is nil")
	}

	runResult, err := r.client.Run(prompt, model)
	if err != nil {
		return nil, err
	}

	if runResult == nil {
		return nil, fmt.Errorf("review invoker returned nil result")
	}

	// Convert invocation result to learnings.Result
	return &learnings.Result{
		Success: runResult.Success,
		Output:  runResult.Output,
	}, nil
}

// cliLogWriter adapts logger operations to pipeline.LogWriter interface
type cliLogWriter struct {
	logsDir                   string
	promptDiagnosticsProvider func() *prompt.PromptDiagnostics
}

var _ pipeline.LogWriter = (*cliLogWriter)(nil)

func (w *cliLogWriter) Write(entry *pipeline.LogEntry) error {
	log, err := logger.NewLogger(w.logsDir)
	if err != nil {
		return err
	}
	defer log.Close()

	model := entry.Model
	if model == "" {
		model = reviewDefaultModel
	}

	reviewLog := &logger.ReviewLog{
		Timestamp:      time.Now(),
		Type:           reviewLogType,
		ReviewType:     reviewLogReviewType,
		Iteration:      0,
		Model:          model,
		Passed:         entry.Passed,
		FixesApplied:   entry.FixesApplied,
		BeadsCreated:   entry.BeadsCreated,
		BacklogCreated: entry.BacklogCreated,
		DurationMs:     0,
	}
	if w.promptDiagnosticsProvider != nil {
		reviewLog.PromptDiagnostics = w.promptDiagnosticsProvider()
	}

	return log.LogReview(reviewLog)
}

// cliStateManager adapts state operations to pipeline.StateManager interface
type cliStateManager struct {
	gromitDir string
}

var _ pipeline.StateManager = (*cliStateManager)(nil)
var _ learnings.ClaudeRunner = (*pipelineLearningsRunnerAdapter)(nil)

func (m *cliStateManager) GetLastReviewCommit() (string, error) {
	if tagCommit, err := state.LatestReviewTagCommitInRepo(reviewRepoDirFromGromitDir(m.gromitDir)); err == nil && tagCommit != "" {
		return tagCommit, nil
	}

	sf, err := state.NewInteractiveFile(m.gromitDir)
	if err != nil {
		return "", err
	}

	if err := sf.Load(); err != nil {
		return "", err
	}

	return sf.LastReviewCommit(), nil
}

func (m *cliStateManager) SetLastReviewCommit(commit string) error {
	sf, err := state.NewInteractiveFile(m.gromitDir)
	if err != nil {
		return err
	}

	if err := sf.Load(); err != nil {
		return err
	}

	currentCommit, err := getGitHeadForReview()
	if err != nil {
		currentCommit = commit
	}

	return sf.RecordReview(currentCommit, 0)
}
