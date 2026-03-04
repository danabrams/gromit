package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/prompt"
)

// Adapters in this file are CLI-specific and wrap internal CLI packages (learnings, logger, state, bead).
// They transform pipeline.Deps interface types to CLI package types and vice versa.
// The adapter pattern enables clean separation between pipeline logic and CLI orchestration:
// - Each adapter wraps CLI-specific functionality (prompt rendering, learnings, state, logging)
// - Each adapter implements exactly one pipeline interface per type
// - Adapters are instantiated by NewPipelineDeps() in adapter_deps.go
// - All adapters include compile-time interface assertions
//
// === CLI Adapter Categories ===
//
// Prompt Renderers (all wrap prompt.Renderer):
// - refinePromptRenderer: Implements RefineRenderer
// - planPromptRenderer: Implements PlanRenderer
// - decomposePromptRenderer: Implements DecomposeRenderer
// - cliPromptRenderer: Implements ReviewRenderer (thorough code review)
// - explorePromptRenderer: Implements ExploreRenderer
//   NOTE: Currently contains business logic (~130 lines) that should be refactored to pure delegation.
//   See adapter_refactor_explore_test.go for refactoring documentation.
//
// State Management:
// - cliBacklogClient: Wraps bead.Client, implements BacklogWriter
// - cliLearningsManager: Wraps learnings.File, implements LearningsManager
// - cliStateManager: Wraps state.File, implements StateManager
// - cliLogWriter: Wraps logger facilities, implements LogWriter
//
// === Adapter Contract ===
// Each adapter MUST:
// 1. Wrap exactly one internal dependency (single primary field)
// 2. Implement exactly one pipeline.* interface
// 3. Delegate entirely to wrapped dependency - contain no business logic
// 4. Include compile-time interface assertion
// 5. Be instantiated in NewPipelineDeps()
//
// Naming conventions:
// - CLI-specific adapters use "cli" prefix when wrapping CLI packages (cliLogWriter, cliStateManager)
// - Prompt renderers use "PromptRenderer" suffix (refinePromptRenderer, cliPromptRenderer)
// - All adapters delegate to their wrapped dependencies without business logic

// cliPromptRenderer adapts prompt.Renderer to pipeline.ReviewRenderer interface
// Pure delegation adapter - all prompt context assembly happens in pipeline
type cliPromptRenderer struct {
	renderer *prompt.Renderer
}

var _ pipeline.ReviewRenderer = (*cliPromptRenderer)(nil)
var _ pipeline.PlanRenderer = (*planPromptRenderer)(nil)

func (r *cliPromptRenderer) LoadClaudeMD() (string, error) {
	return r.renderer.LoadClaudeMD()
}

func (r *cliPromptRenderer) LoadRulesForPhase(phase string) (string, error) {
	return r.renderer.LoadRulesForPhase(phase)
}

func (r *cliPromptRenderer) RenderThoroughReview(input *pipeline.ThoroughReviewPromptInput) (string, error) {
	// Build ThoroughReviewContext from pipeline input
	// Pipeline populates ClaudeMD and Rules, adapter just uses them
	reviewCtx := &prompt.ThoroughReviewContext{
		Diff:     input.Diff,
		ClaudeMD: input.ClaudeMD,
		Rules:    input.Rules,
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

func (c *cliBacklogClient) Add(ctx context.Context, entry *pipeline.BacklogEntry) error {
	if c == nil || c.beadClient == nil {
		return fmt.Errorf("backlog client is not properly initialized")
	}
	if entry == nil {
		return fmt.Errorf("backlog entry is nil")
	}

	_, err := c.beadClient.Create(ctx, entry.Title, entry.Priority, entry.Labels, entry.ExpectedOutputs)
	return err
}

// cliLearningsManager adapts learnings operations to pipeline.LearningsManager interface
type cliLearningsManager struct {
	file *learnings.File
}

var _ pipeline.LearningsManager = (*cliLearningsManager)(nil)

func (m *cliLearningsManager) Add(content string) error {
	if m == nil || m.file == nil {
		return fmt.Errorf("learnings manager is not properly initialized")
	}

	_, err := m.file.Add(reviewSessionCommand, content, learnings.CategoryPatterns)
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
	logType                   string
	logReviewType             string
	defaultModel              string
	promptDiagnosticsProvider func() *prompt.PromptDiagnostics
	logger                    *logger.Logger
	loggerOnce                sync.Once
	loggerErr                 error
}

var _ pipeline.LogWriter = (*cliLogWriter)(nil)

func (w *cliLogWriter) Write(entry *pipeline.LogEntry) error {
	log, err := w.ensureLogger()
	if err != nil {
		return err
	}

	model := entry.Model
	if model == "" {
		model = w.defaultModel
	}

	reviewLog := &logger.ReviewLog{
		Timestamp:      time.Now(),
		Type:           w.logType,
		ReviewType:     w.logReviewType,
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

func (w *cliLogWriter) ensureLogger() (*logger.Logger, error) {
	w.loggerOnce.Do(func() {
		if w.logsDir == "" {
			w.loggerErr = fmt.Errorf("logs directory is not set")
			return
		}
		w.logger, w.loggerErr = logger.NewLogger(w.logsDir)
	})
	return w.logger, w.loggerErr
}

// stateFileAdapter abstracts state.File operations for the adapter
type stateFileAdapter interface {
	LastReviewCommit() string
	RecordReview(commit string, duration int) error
}

// cliStateManager adapts state operations to pipeline.StateManager interface
type cliStateManager struct {
	stateFile stateFileAdapter
}

var _ pipeline.StateManager = (*cliStateManager)(nil)
var _ learnings.ClaudeRunner = (*pipelineLearningsRunnerAdapter)(nil)

func (m *cliStateManager) GetLastReviewCommit() (string, error) {
	if m == nil || m.stateFile == nil {
		return "", fmt.Errorf("state manager is not properly initialized")
	}

	return m.stateFile.LastReviewCommit(), nil
}

func (m *cliStateManager) SetLastReviewCommit(commit string) error {
	if m == nil || m.stateFile == nil {
		return fmt.Errorf("state manager is not properly initialized")
	}

	return m.stateFile.RecordReview(commit, 0)
}
