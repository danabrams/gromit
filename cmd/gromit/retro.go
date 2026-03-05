package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/retro"
	"github.com/danabrams/gromit/internal/scope"
	"github.com/danabrams/gromit/internal/state"
	"github.com/spf13/cobra"
)

const retroSessionCommand = "retro"

var (
	nonInteractive bool
	retroSpecFlag  string
	retroEpicFlag  string
)

var (
	retroResolveAgentFn         = agent.Resolve
	retroSessionLauncherFn      = runWithSessionWorktreeWithConflictSettings
	retroRecordStateFn          = recordRetroState
	retroClaudeFallbackRunnerFn = func(cfg *config.Config) (retro.ProviderRunner, error) {
		opusTimeout, _, _, _ := cfg.Claude.TimeoutsForModel("opus")
		claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, opusTimeout)
		if err != nil {
			return nil, err
		}
		return provider.NewClaudeProvider(claudeClient, provider.DefaultTierToModelMap), nil
	}
)

var retroCmd = &cobra.Command{
	Use:   "retro",
	Short: "Run retrospective analysis",
	Long: `Analyze accumulated learnings to identify patterns and recommend rule updates.

The retro command:
1. Reads LEARNINGS.md
2. Uses opus to analyze patterns and consolidate learnings
3. Identifies duplicate or related learnings
4. Proposes promoting patterns to RULES.md
5. Suggests archiving stale learnings
6. Launches an interactive agent session for review and application

Use --non-interactive to skip the interactive session and write analysis to .gromit/RETRO_PROPOSED_CHANGES.md instead.

Scoping:
  --spec <name>      Filter retro to a spec's beads only
  --epic <id>        Filter retro to an epic's beads only

The --spec and --epic flags are mutually exclusive. When either is used, the retro
analysis only includes iteration logs, stuck beads, and efficiency metrics for beads
within that scope. Without either flag, all beads are included (default behavior).`,
	RunE: runRetro,
}

func registerRetroCommand(root *cobra.Command) {
	if retroCmd.Flags().Lookup("non-interactive") == nil {
		retroCmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Skip interactive session and write analysis to .gromit/RETRO_PROPOSED_CHANGES.md")
		retroCmd.Flags().StringVar(&retroSpecFlag, "spec", "", "Scope retro to a specific spec")
		retroCmd.Flags().StringVar(&retroEpicFlag, "epic", "", "Scope retro to a specific epic")
		retroCmd.Flags().String("agent", "", "Override the default agent for this retro session")
		retroCmd.Flags().Bool("choose-agent", false, "Show interactive picker to choose agent")
	}
	if retroCmd.Parent() == nil {
		root.AddCommand(retroCmd)
	}
}

func runRetro(cmd *cobra.Command, args []string) error {
	if err := scope.ValidateFlags(retroEpicFlag, retroSpecFlag); err != nil {
		return err
	}

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	gromitDir := resolveGromitDir(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nReceived interrupt, stopping...")
		cancel()
	}()

	var beadFilter map[string]bool
	specsDir := resolveSpecsDir(cfg)
	labels, err := resolveScopeLabels(specsDir, retroEpicFlag, retroSpecFlag)
	if err != nil {
		return err
	}

	if len(labels) > 0 {
		beadFilter, err = buildBeadFilter(ctx, labels)
		if err != nil {
			return fmt.Errorf("building bead filter: %w", err)
		}
	}

	fmt.Println("Running retrospective analysis...")
	fmt.Println("This may take a few minutes as it uses opus for quality analysis.")

	runner, err := buildRetroProviderRunner(cfg)
	if err != nil {
		return fmt.Errorf("failed to create retro provider runner: %w", err)
	}

	r, err := retro.NewRetroWithProviderAndBudget(runner, gromitDir, cfg.Prompt.Budget.MaxChars)
	if err != nil {
		return fmt.Errorf("failed to create retro analyzer: %w", err)
	}

	r.SetLogsDir(resolveMainRepoLogsDirFn(gromitDir))

	result, err := r.Run(ctx, beadFilter)
	if err != nil {
		return fmt.Errorf("running retro: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("retro analysis failed")
	}

	fmt.Println("=" + "=" + strings.Repeat("=", 78))
	fmt.Println("RETROSPECTIVE ANALYSIS")
	fmt.Println("=" + "=" + strings.Repeat("=", 78))
	fmt.Println()
	fmt.Println(result.Analysis)
	fmt.Println()
	fmt.Println("=" + "=" + strings.Repeat("=", 78))

	if nonInteractive {
		analysisPath := filepath.Join(gromitDir, "RETRO_PROPOSED_CHANGES.md")
		if err := os.WriteFile(analysisPath, []byte(result.Analysis), 0644); err != nil {
			return fmt.Errorf("writing analysis file: %w", err)
		}

		fmt.Printf("\nAnalysis written to %s\n", analysisPath)
		fmt.Println("Review and apply manually.")
		return nil
	}

	promptText := retro.BuildClaudeCodePrompt(result.Analysis, result.Efficiency, result.Experiment)
	tmpDir := filepath.Join(gromitDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("creating tmp dir: %w", err)
	}
	promptFile, err := os.CreateTemp(tmpDir, "retro-prompt-*.md")
	if err != nil {
		return fmt.Errorf("creating temp prompt file: %w", err)
	}
	promptPath := promptFile.Name()
	defer os.Remove(promptPath)
	if _, err := promptFile.WriteString(promptText); err != nil {
		promptFile.Close()
		return fmt.Errorf("writing prompt file: %w", err)
	}
	promptFile.Close()

	return launchRetroInteractiveSession(cfg, cmd, gromitDir, promptPath)
}

func buildBeadFilterWithClient(ctx context.Context, labels []string, client interface {
	ListWithLabel(context.Context, string) ([]*bead.Bead, error)
}) (map[string]bool, error) {
	if len(labels) == 0 {
		return nil, nil
	}

	filter := make(map[string]bool)
	for _, label := range labels {
		beads, err := client.ListWithLabel(ctx, label)
		if err != nil {
			return nil, err
		}

		for _, b := range beads {
			filter[b.ID] = true
		}
	}

	return filter, nil
}

func buildBeadFilter(ctx context.Context, labels []string) (map[string]bool, error) {
	if len(labels) == 0 {
		return nil, nil
	}

	client, err := bead.NewClient()
	if err != nil {
		return nil, err
	}

	return buildBeadFilterWithClient(ctx, labels, client)
}

func buildRetroProviderRunner(cfg *config.Config) (retro.ProviderRunner, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	if cfg.HasProviders() {
		router, err := provider.BuildRouterFromConfig(cfg)
		if err != nil {
			return nil, err
		}
		return &retroRouterAdapter{Router: router, Phase: retroSessionCommand}, nil
	}

	runner, err := retroClaudeFallbackRunnerFn(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating retro claude fallback runner: %w", err)
	}
	return runner, nil
}

func launchRetroInteractiveSession(cfg *config.Config, cmd *cobra.Command, gromitDir, promptPath string) error {
	fmt.Println("\nLaunching interactive review session...")
	absPromptPath, err := filepath.Abs(promptPath)
	if err != nil {
		return fmt.Errorf("resolving prompt path: %w", err)
	}

	agentFlag, _ := cmd.Flags().GetString("agent")
	chooseAgent, _ := cmd.Flags().GetBool("choose-agent")
	selectedAgent, err := retroResolveAgentFn(cfg, retroSessionCommand, agentFlag, chooseAgent, os.Stdin, os.Stdout)
	if err != nil {
		return fmt.Errorf("resolving agent: %w", err)
	}

	err = launchInSessionIfEnabled(cfg, gromitDir, retroSessionCommand, retroSessionLauncherFn, func(sessionDir string) error {
		if err := selectedAgent.LaunchInDir(absPromptPath, sessionDir); err != nil {
			return fmt.Errorf("launching interactive review session: %w", err)
		}
		return nil
	}, func() error {
		return selectedAgent.LaunchInDir(absPromptPath, "")
	})
	if err != nil {
		return err
	}

	return retroRecordStateFn(gromitDir)
}

func recordRetroState(gromitDir string) error {
	sf, err := state.NewFile(gromitDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create state file: %v\n", err)
		return nil
	}
	if err := sf.RecordRetro(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not record retro time: %v\n", err)
	}
	return nil
}
