package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/spf13/cobra"
)

var exploreCmd = &cobra.Command{
	Use:   "explore [topic]",
	Short: "Launch interactive exploration session",
	Long: fmt.Sprintf(`Launch an interactive brainstorming session for exploring a big idea or problem space.

The session receives full project context and guides collaborative exploration
to break down ideas into concrete artifacts: backlog items (via gromit add),
specs, or epics — whatever granularity makes sense.

Examples:
  gromit explore                              # Open-ended brainstorm
  gromit explore "Improve developer onboarding" # Pre-seeded topic
  gromit explore --model sonnet "Add dark mode" # Override model
%s
%s

%s`, exploreCodexHelpExample, exploreChooseAgentHelpExample, exploreAgentSelectionHelpSentence),
	Args: cobra.MaximumNArgs(1),
	RunE: runExplore,
}

var exploreModel string
var exploreSessionLauncherFn = runWithSessionWorktreeWithConflictSettings
var exploreRunInDirFn = runInDir

const exploreCodexHelpExample = `  gromit explore --agent codex "Audit onboarding flow" # Use Codex for the session`
const exploreChooseAgentHelpExample = `  gromit explore --choose-agent "Audit onboarding flow" # Pick an agent interactively`
const exploreAgentSelectionHelpSentence = "Agent selection priority: --agent, --choose-agent, agents.phases.explore, then the configured default agent."
const exploreAgentOverrideFlag = "--from-override"
const explorePhaseConfigFlag = "--from-phase-config"
const exploreAgentFlagName = "agent"
const exploreChooseAgentFlagName = "choose-agent"
const (
	exploreSectionTopic        = "topic"
	exploreSectionLearnings    = "learnings"
	exploreSectionInstructions = "instructions"
	exploreSessionCommand      = "explore"
)

func init() {
	exploreCmd.Flags().StringVar(&exploreModel, "model", "opus", "Model to use when the Claude agent is selected (opus, sonnet, haiku)")
	exploreCmd.Flags().String(exploreAgentFlagName, "", "Override the default agent for this explore session")
	exploreCmd.Flags().Bool(exploreChooseAgentFlagName, false, "Show interactive picker to choose agent")
	rootCmd.AddCommand(exploreCmd)
}

func runExplore(cmd *cobra.Command, args []string) error {
	// Get config and directories
	cfg, err := loadConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("loading config: %w", err)
		}
		cfg = nil
	}

	// Extract topic from args
	var topic string
	if len(args) > 0 {
		topic = args[0]
	}

	// Build pipeline
	p, err := buildExplorePipeline(cfg)
	if err != nil {
		return fmt.Errorf("building pipeline: %w", err)
	}

	// Execute explore workflow
	ctx := context.Background()
	agentFlag, _ := cmd.Flags().GetString("agent")
	chooseAgent, _ := cmd.Flags().GetBool("choose-agent")
	input := pipeline.ExploreInput{
		Topic:       topic,
		AgentName:   agentFlag,
		ChooseAgent: chooseAgent,
		Model:       exploreModel,
	}

	result, err := runExploreInSession(ctx, cfg, resolveGromitDir(cfg), p, input)
	if err != nil {
		return fmt.Errorf("explore workflow: %w", err)
	}

	// Display results
	if len(result.CreatedEpics) > 0 {
		fmt.Printf("\nEpics created:\n")
		for _, epic := range result.CreatedEpics {
			fmt.Printf("  - %s\n", epic)
		}
	}

	if len(result.CreatedSpecs) > 0 {
		fmt.Printf("\nSpecs created:\n")
		for _, spec := range result.CreatedSpecs {
			fmt.Printf("  - %s\n", spec)
		}
	}

	if len(result.CreatedBacklogItems) > 0 {
		fmt.Printf("\nBacklog items created: %d\n", len(result.CreatedBacklogItems))
	}

	return nil
}

func runExploreInSession(
	ctx context.Context,
	cfg *config.Config,
	gromitDir string,
	p *pipeline.Pipeline,
	input pipeline.ExploreInput,
) (*pipeline.ExploreResult, error) {
	if p == nil {
		return nil, fmt.Errorf("pipeline is nil")
	}

	if cfg != nil && !cfg.Worktree.IsEnabled() {
		return p.Explore(ctx, input)
	}

	conflictSettings := sessionConflictSettingsFromConfig(cfg)
	var result *pipeline.ExploreResult
	_, err := exploreSessionLauncherFn(gromitDir, exploreSessionCommand, conflictSettings, func(sessionDir string) error {
		return exploreRunInDirFn(sessionDir, func() error {
			var runErr error
			result, runErr = p.Explore(ctx, input)
			return runErr
		})
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// buildExplorePipeline constructs a Pipeline configured for the explore workflow
func buildExplorePipeline(cfg *config.Config) (*pipeline.Pipeline, error) {
	gromitDir := resolveGromitDir(cfg)
	epicsDir := filepath.Join(gromitDir, "epics")
	specsDir := resolveSpecsDir(cfg)
	templatesDir := resolveTemplatesDir(cfg)
	projectClaudeMDPath := resolveProjectClaudeMD(cfg)

	// Ensure directories exist
	if err := os.MkdirAll(epicsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating epics dir: %w", err)
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating specs dir: %w", err)
	}

	// Create renderer
	renderer, err := prompt.NewRenderer(
		templatesDir,
		specsDir,
		projectClaudeMDPath,
		gromitDir,
	)
	if err != nil {
		return nil, fmt.Errorf("creating renderer: %w", err)
	}

	// Create backlog client
	backlogFile, err := backlog.NewFile(gromitDir)
	if err != nil {
		return nil, fmt.Errorf("creating backlog file: %w", err)
	}

	// Create adapters
	agentResolver := &cmdAgentResolver{cfg: cfg}
	promptRenderer := &explorePromptRenderer{renderer: renderer}
	backlogClient := &exploreBacklogClient{file: backlogFile}

	deps := &pipeline.Deps{
		AgentResolver:   agentResolver,
		ExploreRenderer: promptRenderer,
		BacklogClient:   backlogClient,
	}

	paths := &pipeline.Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		EpicsDir:  epicsDir,
	}

	return pipeline.New(deps, paths), nil
}

// Adapter types

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
	workDir, _ := os.Getwd()
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
		exploreSectionTopic:        topic,
		prompt.SectionClaudeMD:     claudeMD,
		prompt.SectionRules:        rules,
		exploreSectionLearnings:    learningsSection,
		exploreSectionInstructions: renderedInstructions,
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

// exploreBacklogClient adapts backlog.File to pipeline.BacklogClient
type exploreBacklogClient struct {
	file *backlog.File
}

var _ pipeline.BacklogClient = (*exploreBacklogClient)(nil)

func (c *exploreBacklogClient) List() ([]*pipeline.Idea, error) {
	items, err := c.file.List()
	if err != nil {
		return nil, err
	}

	ideas := make([]*pipeline.Idea, 0, len(items))
	for _, item := range items {
		ideas = append(ideas, &pipeline.Idea{
			ID:       item.ID,
			Text:     item.Text,
			Type:     item.Type,
			Context:  item.Context,
			CreatedAt: item.CreatedAt,
			Status:   item.Status,
			SpecName: item.SpecName,
		})
	}

	return ideas, nil
}

func (c *exploreBacklogClient) Get(id string) (*pipeline.Idea, error) {
	return nil, fmt.Errorf("not implemented")
}

func (c *exploreBacklogClient) Add(item *pipeline.Idea) error {
	return fmt.Errorf("not implemented")
}

func (c *exploreBacklogClient) Update(id string, fn func(*pipeline.Idea)) error {
	return fmt.Errorf("not implemented")
}

// formatLearnings formats learnings into a markdown string.
// If confirmed is true, returns confirmed learnings; otherwise returns recent learnings (last 24 months).
// Returns "*None*" if the file is nil or no learnings exist.
func formatLearnings(lf *learnings.File, confirmed bool) string {
	if lf == nil {
		return "*None*"
	}

	var items []learnings.Learning
	if confirmed {
		items = lf.GetConfirmed()
	} else {
		items = lf.GetRecent(24)
	}

	if len(items) == 0 {
		return "*None*"
	}

	var sb strings.Builder
	for _, l := range items {
		sb.WriteString(fmt.Sprintf("- **[%s]** %s\n", l.Category, l.Content))
	}
	return sb.String()
}
