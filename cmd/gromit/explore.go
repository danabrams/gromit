package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/agent"
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
%s`, exploreCodexHelpExample),
	Args: cobra.MaximumNArgs(1),
	RunE: runExplore,
}

var exploreModel string

const exploreCodexHelpExample = `  gromit explore --agent codex "Audit onboarding flow" # Use Codex for the session`

func init() {
	exploreCmd.Flags().StringVar(&exploreModel, "model", "opus", "Claude model to use (opus, sonnet, haiku)")
	exploreCmd.Flags().String("agent", "", "Override the default agent for this explore session")
	exploreCmd.Flags().Bool("choose-agent", false, "Show interactive picker to choose agent")
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

	result, err := p.Explore(ctx, input)
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

// buildExplorePipeline constructs a Pipeline configured for the explore workflow
func buildExplorePipeline(cfg *config.Config) (*pipeline.Pipeline, error) {
	gromitDir := resolveGromitDir(cfg)
	epicsDir := filepath.Join(gromitDir, "epics")
	specsDir := resolveSpecsDir(cfg)

	// Ensure directories exist
	if err := os.MkdirAll(epicsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating epics dir: %w", err)
	}
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating specs dir: %w", err)
	}

	// Create renderer
	renderer, err := prompt.NewRenderer(
		cfg.Paths.Templates,
		cfg.Paths.Specs,
		cfg.Paths.ProjectClaudeMD,
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
	agentResolver := &exploreAgentResolver{cfg: cfg}
	promptRenderer := &explorePromptRenderer{renderer: renderer}
	backlogClient := &exploreBacklogClient{file: backlogFile}

	deps := &pipeline.Deps{
		AgentResolver:  agentResolver,
		PromptRenderer: promptRenderer,
		BacklogClient:  backlogClient,
	}

	paths := &pipeline.Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		EpicsDir:  epicsDir,
	}

	return pipeline.New(deps, paths), nil
}

// Adapter types

// exploreAgentResolver adapts agent.Resolve to pipeline.AgentResolver
type exploreAgentResolver struct {
	cfg *config.Config
}

func (r *exploreAgentResolver) Resolve(phase string, flagOverride string, choosePicker bool) (pipeline.Agent, error) {
	return agent.Resolve(r.cfg, phase, flagOverride, choosePicker, os.Stdin, os.Stdout)
}

// explorePromptRenderer adapts prompt.Renderer to pipeline.PromptRenderer
type explorePromptRenderer struct {
	renderer *prompt.Renderer
}

func (r *explorePromptRenderer) RenderRefine(input *pipeline.RefinePromptInput) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (r *explorePromptRenderer) RenderPlan(input *pipeline.PlanPromptInput) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (r *explorePromptRenderer) RenderDecompose(input *pipeline.DecomposePromptInput) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (r *explorePromptRenderer) RenderThoroughReview(input *pipeline.ThoroughReviewPromptInput) (string, error) {
	return "", fmt.Errorf("not implemented")
}

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

#### Confirmed Patterns

%s

#### Recent Learnings

%s

`, confirmedLearnings, recentLearnings))

	// Exploration instructions
	sb.WriteString("## Instructions\n\n")
	sb.WriteString("You are running an exploration session. Your goal is to brainstorm a big idea or problem space and break it down into concrete, actionable artifacts.\n\n")
	sb.WriteString("### What To Do\n\n")
	sb.WriteString("1. **Understand the problem space** — If a topic was provided above, start there. Otherwise, ask the user what they want to explore. Read relevant code and docs to ground your understanding.\n\n")
	sb.WriteString("2. **Brainstorm broadly** — Think through the problem from multiple angles. Consider user needs, technical constraints, edge cases, and alternative approaches. Discuss ideas with the user — this is a collaborative session.\n\n")
	sb.WriteString("3. **Break it down** — As ideas crystallize, capture them as the appropriate artifact type:\n\n")
	sb.WriteString(fmt.Sprintf("   - **Backlog items** — Quick ideas, rough feature requests, bugs, or chores. Add these by running: gromit add \"<idea>\". Optionally provide context when prompted. These flow through the refine → plan → decompose pipeline later.\n"))
	sb.WriteString(fmt.Sprintf("   - **Specs** — For ideas that are well-understood enough to specify, write a spec file to %s/<name>.md. A spec describes what to build, why, acceptance criteria, and key decisions. See existing specs in that directory for the format.\n", specsDir))
	sb.WriteString(fmt.Sprintf("   - **Epics** — For large initiatives that span multiple specs, write an epic file to %s/epics/<name>.md with frontmatter containing epic_id and created fields.\n\n", gromitDir))
	sb.WriteString("4. **Prefer backlog items** — When in doubt, use gromit add. Backlog items are cheap and get refined later. Only write specs for ideas you've discussed enough to specify clearly. Only create epics when the scope genuinely spans multiple independent specs.\n\n")
	sb.WriteString("### What NOT To Do\n\n")
	sb.WriteString("- Do NOT implement features or write production code\n")
	sb.WriteString("- Do NOT create beads with bd create — that happens during decomposition\n")
	sb.WriteString("- Do NOT skip the conversation — explore with the user, don't just dump a list of ideas\n\n")
	sb.WriteString("### Session Flow\n\n")
	sb.WriteString("Start by understanding the topic, then alternate between discussing ideas and capturing them. End the session when the problem space feels well-mapped and the key ideas have been captured as artifacts.\n")

	return sb.String(), nil
}

// exploreBacklogClient adapts backlog.File to pipeline.BacklogClient
type exploreBacklogClient struct {
	file *backlog.File
}

func (c *exploreBacklogClient) List() ([]*pipeline.Idea, error) {
	items, err := c.file.List()
	if err != nil {
		return nil, err
	}

	var ideas []*pipeline.Idea
	for _, item := range items {
		ideas = append(ideas, &pipeline.Idea{
			ID:       item.ID,
			Text:     item.Text,
			Type:     item.Type,
			Context:  item.Context,
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
