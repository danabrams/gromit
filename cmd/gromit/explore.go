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
%s
%s

%s`, exploreCodexHelpExample, exploreChooseAgentHelpExample, exploreAgentSelectionHelpSentence),
	Args: cobra.MaximumNArgs(1),
	RunE: runExplore,
}

var exploreSessionLauncherFn = runWithSessionWorktreeWithConflictSettings
var exploreRunInDirFn = runInDir

// exploreRunner defines the interface for running explore workflows
type exploreRunner interface {
	Explore(ctx context.Context, input pipeline.ExploreInput) (*pipeline.ExploreResult, error)
}

// exploreRunnerFactoryFn is a package-level seam for dependency injection
var exploreRunnerFactoryFn = buildExploreRunner

const exploreModelFlagName = "model"

const exploreCodexHelpExample = `  gromit explore --agent codex "Audit onboarding flow" # Use Codex for the session`
const exploreChooseAgentHelpExample = `  gromit explore --choose-agent "Audit onboarding flow" # Pick an agent interactively`
const exploreAgentSelectionHelpSentence = "Agent selection priority: --agent, --choose-agent, agents.phases.explore, then the configured default agent."
const exploreAgentFlagName = "agent"
const exploreChooseAgentFlagName = "choose-agent"
const (
	exploreSectionTopic        = "topic"
	exploreSectionLearnings    = "learnings"
	exploreSectionInstructions = "instructions"
	exploreSessionCommand      = "explore"
)

func init() {
	exploreCmd.Flags().String(exploreModelFlagName, "opus", "Model to use when the Claude agent is selected (opus, sonnet, haiku)")
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

	// Build runner via factory seam
	runner, err := exploreRunnerFactoryFn(cmd, cfg)
	if err != nil {
		return fmt.Errorf("building runner: %w", err)
	}

	// Execute explore workflow
	ctx := context.Background()
	agentFlag, _ := cmd.Flags().GetString("agent")
	chooseAgent, _ := cmd.Flags().GetBool("choose-agent")
	input := pipeline.ExploreInput{
		Topic:       topic,
		AgentName:   agentFlag,
		ChooseAgent: chooseAgent,
		Model:       resolveEffectiveInteractiveModel(cmd, cfg, exploreSessionCommand, exploreModelFlagName),
	}

	result, err := runExploreInSession(ctx, cfg, resolveGromitDir(cfg), runner, input)
	if err != nil {
		return fmt.Errorf("explore workflow: %w", err)
	}

	// Display results
	handleExploreOutput(result)

	return nil
}

func handleExploreOutput(result *pipeline.ExploreResult) {
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
}

func runExploreInSession(
	ctx context.Context,
	cfg *config.Config,
	gromitDir string,
	runner exploreRunner,
	input pipeline.ExploreInput,
) (*pipeline.ExploreResult, error) {
	if runner == nil {
		return nil, fmt.Errorf("runner is nil")
	}

	var result *pipeline.ExploreResult
	fallback := func() error {
		var err error
		result, err = runner.Explore(ctx, input)
		return err
	}

	if err := launchInSessionIfEnabled(cfg, gromitDir, exploreSessionCommand, exploreSessionLauncherFn, func(sessionDir string) error {
		return exploreRunInDirFn(sessionDir, func() error {
			var runErr error
			result, runErr = runner.Explore(ctx, input)
			return runErr
		})
	}, fallback); err != nil {
		return nil, err
	}

	return result, nil
}

// buildExploreRunner creates an exploreRunner by building a pipeline
func buildExploreRunner(cmd *cobra.Command, cfg *config.Config) (exploreRunner, error) {
	return buildExplorePipeline(cmd, cfg)
}

// buildExplorePipeline constructs a Pipeline configured for the explore workflow
func buildExplorePipeline(cmd *cobra.Command, cfg *config.Config) (*pipeline.Pipeline, error) {
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

	// Construct pipeline dependencies using dependency injection
	deps, err := NewPipelineDeps(cfg, gromitDir)
	if err != nil {
		return nil, fmt.Errorf("constructing pipeline deps: %w", err)
	}

	// Create renderer for explore-specific rendering
	renderer, err := prompt.NewRenderer(
		templatesDir,
		specsDir,
		projectClaudeMDPath,
		gromitDir,
	)
	if err != nil {
		return nil, fmt.Errorf("creating renderer: %w", err)
	}

	// Override explore-specific renderer
	promptRenderer := &explorePromptRenderer{renderer: renderer}
	deps.ExploreRenderer = promptRenderer

	// Override backlog client with explore-specific implementation
	backlogFile, err := backlog.NewFile(gromitDir)
	if err != nil {
		return nil, fmt.Errorf("creating backlog file: %w", err)
	}
	backlogClient := &exploreBacklogClient{file: backlogFile}
	deps.BacklogClient = backlogClient

	paths := &pipeline.Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		EpicsDir:  epicsDir,
	}

	deps.ModelForwarder = exploreInteractiveModelForwarder(cmd, cfg, exploreModelFlagName)

	return pipeline.New(deps, paths), nil
}

func exploreInteractiveModelForwarder(cmd *cobra.Command, cfg *config.Config, flagName string) func(pipeline.Agent, string) (pipeline.Agent, string) {
	return func(pAgent pipeline.Agent, model string) (pipeline.Agent, string) {
		if model == "" {
			return pAgent, ""
		}

		resolvedAgent, ok := pAgent.(agent.Agent)
		if !ok {
			return pAgent, "model forwarding not supported for agent " + pAgent.Name()
		}

		flagChanged := cmd != nil && flagName != "" && cmd.Flags().Lookup(flagName) != nil && cmd.Flags().Changed(flagName)

		if resolvedAgent.Name() == "claude" {
			overridden := TryOverrideModel(cmd, resolvedAgent, model, cfg, flagName, true)
			if overridden != nil && overridden != resolvedAgent {
				return overridden, ""
			}
			return resolvedAgent, ""
		}

		forwarded, warning := agent.ForwardModelToAgent(resolvedAgent, model)
		if forwarded == nil {
			forwarded = resolvedAgent
		}
		if flagChanged && (warning != "" || forwarded == resolvedAgent) {
			warning = fmt.Sprintf("--model flag ignored for non-Claude agent %q", resolvedAgent.Name())
		}
		return forwarded, warning
	}
}

// Adapter types

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
