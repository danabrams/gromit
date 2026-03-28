package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/retro"
	"github.com/danabrams/gromit/internal/runner"
	"github.com/danabrams/gromit/internal/scope"
	"github.com/danabrams/gromit/internal/state"
	"github.com/spf13/cobra"
)

var (
	configPath                 string
	maxIterations              int
	readinessEmergencyOverride bool
	nonInteractive             bool
	timeBudgetMinutes          int
	timeBudgetHours            int
	retroSpecFlag              string
	retroEpicFlag              string
	runSpecFlag                string
	runEpicFlag                string
	statusSPC                  bool
	statusJSON                 bool
)

var (
	newRunnerWithStageContextFn = runner.NewRunnerWithStageContext
	newBuildSpecStageContextFn  = runner.BuildSpecStageContext
	runInDedicatedWorktreeFn    = runInDedicatedWorktree
)

var (
	newSpecflowStoreFn     = runner.SpecflowStoreFactory
	newSpecBranchCreatorFn = runner.SpecBranchCreatorFactory
)

var runHasOpenBeadsForLabelFn = hasOpenBeadsForLabel

var retroResolveAgentFn = agent.Resolve
var retroSessionLauncherFn = runWithSessionWorktreeWithConflictSettings
var retroRecordStateFn = recordRetroState
var retroClaudeFallbackRunnerFn = func(cfg *config.Config) (retro.ProviderRunner, error) {
	opusTimeout, _, _, _ := cfg.Claude.TimeoutsForModel("opus")
	claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, opusTimeout)
	if err != nil {
		return nil, err
	}
	return provider.NewClaudeProvider(claudeClient, provider.DefaultTierToModelMap), nil
}
var _ retro.ProviderRunner = (*provider.ClaudeProvider)(nil)

const retroSessionCommand = "retro"

const (
	runSignalBufferSize = 2
	gracefulStopMessage = "\nReceived interrupt, stopping after current iteration (Ctrl+C again to force stop)..."
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:           "gromit",
	Short:         "Gromit - Execute the Gromit loop correctly",
	SilenceErrors: true,
	Long: `Gromit executes AI coding tasks with fresh context on each iteration.

It integrates with bd (beads) for task management and uses model escalation
for handling failures efficiently.`,
}

var runCmd = &cobra.Command{
	Use:          "run",
	Short:        "Run the Gromit loop",
	SilenceUsage: true,
	Long: `Execute the Gromit loop, processing beads one at a time with fresh context.

Each iteration:
1. Gets the next unblocked bead from bd
2. Selects the appropriate model based on priority/labels
3. Invokes the selected provider with a fresh context
4. Runs validation (optional)
5. Closes the bead on success
6. Escalates to a stronger model on failure

Press Ctrl+C once to stop after the current iteration (press again to force stop).
`,
	RunE: runLoop,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current queue status",
	Long:  `Display information about the next bead in the queue and what model would be used.`,
	RunE:  showStatus,
}

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

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "gromit.yaml", "Path to config file")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if !commandRequiresRepoRoot(cmd) {
			return nil
		}
		return ensureRepoRoot()
	}

	runCmd.Flags().IntVarP(&maxIterations, "max-iterations", "n", 0, "Maximum iterations (0 = unlimited)")
	runCmd.Flags().IntVarP(&timeBudgetMinutes, "time-budget", "t", 0, "Time budget in minutes (0 = unlimited)")
	runCmd.Flags().IntVarP(&timeBudgetHours, "time-budget-hours", "H", 0, "Time budget in hours (0 = unlimited)")
	runCmd.Flags().StringVar(&runSpecFlag, "spec", "", "Filter to beads for a specific spec")
	runCmd.Flags().StringVar(&runEpicFlag, "epic", "", "Filter to beads for a specific epic")
	runCmd.Flags().BoolVar(&readinessEmergencyOverride, "readiness-emergency-override", false, "Allow bypass of the readiness gate in emergencies")

	statusCmd.Flags().BoolVar(&statusSPC, "spc", false, "Show SPC dashboard status only")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Show status output as JSON")

	retroCmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Skip interactive session and write analysis to .gromit/RETRO_PROPOSED_CHANGES.md")
	retroCmd.Flags().StringVar(&retroSpecFlag, "spec", "", "Scope retro to a specific spec")
	retroCmd.Flags().StringVar(&retroEpicFlag, "epic", "", "Scope retro to a specific epic")
	retroCmd.Flags().String("agent", "", "Override the default agent for this retro session")
	retroCmd.Flags().Bool("choose-agent", false, "Show interactive picker to choose agent")

	registerRootCommands(rootCmd)
}

func registerRootCommands(root *cobra.Command) {
	root.AddCommand(runCmd)
	root.AddCommand(run2Cmd)
	root.AddCommand(debugCmd)
	root.AddCommand(statusCmd)
	root.AddCommand(readyCmd)
	root.AddCommand(retroCmd)
	root.AddCommand(validatePRMetadataCmd)
	registerBenchmarkCommands(root)
}

func commandRequiresRepoRoot(cmd *cobra.Command) bool {
	if isInitCommand(cmd) {
		return false
	}
	if isBenchmarkCommand(cmd) {
		return false
	}
	if isValidatePRMetadataCommand(cmd) {
		return false
	}
	return true
}

func isValidatePRMetadataCommand(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if cmd == validatePRMetadataCmd {
		return true
	}
	if cmd.Name() == validatePRMetadataCmd.Name() {
		return true
	}
	if cmd.Use == validatePRMetadataCmd.Use {
		return true
	}
	return false
}

func isInitCommand(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if cmd == initCmd {
		return true
	}
	if cmd.Name() == initCmd.Name() {
		return true
	}
	if cmd.Use == initCmd.Use {
		return true
	}
	return false
}

func isBenchmarkCommand(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		if current == benchmarkCmd {
			return true
		}
	}
	return false
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		// If gromit.yaml doesn't exist, provide helpful error
		if os.IsNotExist(err) && configPath == "gromit.yaml" {
			return nil, fmt.Errorf("gromit.yaml not found - run 'gromit init' to set up this project")
		}
		return nil, err
	}
	return cfg, nil
}

func runLoop(cmd *cobra.Command, args []string) error {
	if err := scope.ValidateFlags(runEpicFlag, runSpecFlag); err != nil {
		return err
	}

	// Capture main worktree dir before entering the dedicated run worktree.
	// ensureRepoRoot() has already set cwd to project root via PersistentPreRunE.
	mainDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determining main dir: %w", err)
	}

	// Set up context with signal handling outside the worktree
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, runSignalBufferSize)
	stopCh := make(chan struct{})
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go handleRunSignals(sigCh, stopCh, cancel, os.Stderr)

	return runInDedicatedWorktreeFn(ctx, mainDir, func() error {
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		cfg.SetDefaults()
		cfg.RunWorktreeMode = true

		applyReadinessEmergencyOverrideFlag(cfg)

		specsDir := resolveSpecsDir(cfg)
		labels, err := resolveScopeLabels(specsDir, runEpicFlag, runSpecFlag)
		if err != nil {
			return err
		}
		if runSpecFlag != "" {
			fmt.Fprintf(os.Stderr, "Spec: %s\n", runSpecFlag)
		}

		// Override max iterations from flag if set
		if maxIterations > 0 {
			cfg.Loop.MaxIterations = maxIterations
		}

		// Compute deadline from time budget flags (additive: total = minutes + hours*60)
		var deadline time.Time
		if timeBudgetMinutes > 0 || timeBudgetHours > 0 {
			totalMinutes := timeBudgetMinutes + timeBudgetHours*60
			deadline = time.Now().Add(time.Duration(totalMinutes) * time.Minute)
		}

		gromitDir := resolveGromitDir(cfg)
		var stageCtx *runner.StageContext
		if runSpecFlag != "" {
			var stageCtxErr error
			origStoreFactory := runner.SpecflowStoreFactory
			runner.SpecflowStoreFactory = newSpecflowStoreFn
			stageCtx, stageCtxErr = newBuildSpecStageContextFn(ctx, cfg, runSpecFlag, gromitDir)
			runner.SpecflowStoreFactory = origStoreFactory
			if stageCtxErr != nil {
				return fmt.Errorf("initializing specflow stage: %w", stageCtxErr)
			}
			if stageCtx != nil && stageCtx.SpecName != "" {
				repoDir, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("determining repo dir: %w", err)
				}
				origBranchFactory := runner.SpecBranchCreatorFactory
				runner.SpecBranchCreatorFactory = newSpecBranchCreatorFn
				if err := runner.EnsureSpecBranch(ctx, cfg, stageCtx, repoDir); err != nil {
					runner.SpecBranchCreatorFactory = origBranchFactory
					return fmt.Errorf("preparing spec branch: %w", err)
				}
				runner.SpecBranchCreatorFactory = origBranchFactory
				fmt.Fprintf(os.Stderr, "Spec branch ready\n")
			}
		}

		r, err := newRunnerWithStageContextFn(cfg, os.Stdout, stageCtx, labels...)
		if err != nil {
			return fmt.Errorf("failed to create runner: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Starting run loop...\n")
		return r.Run(ctx, cfg.Loop.MaxIterations, deadline, stopCh)
	})
}

func applyReadinessEmergencyOverrideFlag(cfg *config.Config) {
	if !readinessEmergencyOverride || cfg == nil {
		return
	}
	cfg.ReadinessEmergencyOverride = true
}

func handleRunSignals(sigCh <-chan os.Signal, stopCh chan<- struct{}, cancel context.CancelFunc, stderr io.Writer) {
	gracefulStopTriggered := false
	for sig := range sigCh {
		switch sig {
		case syscall.SIGINT:
			if !gracefulStopTriggered {
				gracefulStopTriggered = true
				fmt.Fprintln(stderr, gracefulStopMessage)
				close(stopCh)
				continue
			}
			cancel()
			return
		default:
			cancel()
			return
		}
	}
}

func showStatus(cmd *cobra.Command, args []string) error {
	if statusJSON && statusSPC {
		return fmt.Errorf("--json and --spc flags are mutually exclusive")
	}
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	gromitDir := resolveGromitDir(cfg)

	if statusJSON {
		statusJSONOutput, err := runner.BuildStatusJSON(gromitDir, cfg)
		if err != nil {
			return fmt.Errorf("building status JSON: %w", err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(statusJSONOutput); err != nil {
			return fmt.Errorf("encoding status JSON: %w", err)
		}
		return nil
	}

	return runner.PrintStatus(gromitDir, cfg, os.Stdout, nil, statusSPC)
}

func runRetro(cmd *cobra.Command, args []string) error {
	// Validate flags first
	if err := scope.ValidateFlags(retroEpicFlag, retroSpecFlag); err != nil {
		return err
	}

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Determine .gromit directory from config
	gromitDir := resolveGromitDir(cfg)

	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nReceived interrupt, stopping...")
		cancel()
	}()

	// Build bead filter from flags
	var beadFilter map[string]bool
	specsDir := resolveSpecsDir(cfg)
	labels, err := resolveScopeLabels(specsDir, retroEpicFlag, retroSpecFlag)
	if err != nil {
		return err
	}

	if len(labels) > 0 {
		var err error
		beadFilter, err = buildBeadFilter(ctx, labels)
		if err != nil {
			return fmt.Errorf("building bead filter: %w", err)
		}
	}

	// Run retrospective
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

	// Resolve the logs directory, falling back to the main repo's logs when
	// running from a session worktree where .gromit/logs/ doesn't exist.
	r.SetLogsDir(resolveMainRepoLogsDirFn(gromitDir))

	result, err := r.Run(ctx, beadFilter)
	if err != nil {
		return fmt.Errorf("running retro: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("retro analysis failed")
	}

	// Display results
	fmt.Println("=" + "=" + strings.Repeat("=", 78))
	fmt.Println("RETROSPECTIVE ANALYSIS")
	fmt.Println("=" + "=" + strings.Repeat("=", 78))
	fmt.Println()
	fmt.Println(result.Analysis)
	fmt.Println()
	fmt.Println("=" + "=" + strings.Repeat("=", 78))

	// If --non-interactive flag is set, write to file and exit
	if nonInteractive {
		analysisPath := filepath.Join(gromitDir, "RETRO_PROPOSED_CHANGES.md")
		if writeErr := os.WriteFile(analysisPath, []byte(result.Analysis), 0644); writeErr != nil {
			return fmt.Errorf("writing analysis file: %w", writeErr)
		}

		fmt.Printf("\nAnalysis written to %s\n", analysisPath)
		fmt.Println("Review and apply manually.")
		return nil
	}

	// Default: launch interactive review and application session
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

// buildBeadFilterWithClient is a testable version of buildBeadFilter that accepts a bead client.
// It threads the provided context through to all ListWithLabel calls.
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

// buildBeadFilter resolves labels to bead IDs and returns a filter map.
// If labels is empty or nil, returns nil (no filtering).
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

// resolveScopeLabels validates and resolves spec/epic scope flags into bead labels.
// It assumes mutual exclusivity has already been enforced by scope.ValidateFlags.
func resolveScopeLabels(specsDir, epicFlag, specFlag string) ([]string, error) {
	if specFlag != "" {
		if err := scope.ValidateSpec(specsDir, specFlag); err != nil {
			return nil, fmt.Errorf("validating spec: %w", err)
		}
		return scope.ResolveSpec(specFlag), nil
	}

	if epicFlag != "" {
		labels, err := scope.ResolveEpic(epicFlag, specsDir)
		if err != nil {
			return nil, fmt.Errorf("resolving epic scope: %w", err)
		}
		return labels, nil
	}

	return nil, nil
}

func resolveLegacyRunSpecScope(cfg *config.Config, specsDir, specFlag string, originalErr error) ([]string, error) {
	if originalErr == nil || specFlag == "" || cfg == nil || cfg.Compatibility.StrictLegacyFallback {
		return nil, originalErr
	}

	label := fmt.Sprintf("spec:%s", specFlag)
	hasOpen, err := runHasOpenBeadsForLabelFn(label)
	if err != nil || !hasOpen {
		return nil, originalErr
	}

	fmt.Fprintf(
		os.Stderr,
		"Warning: spec %q not found in %s; using legacy label scope %q because matching open beads exist\n",
		specFlag,
		specsDir,
		label,
	)
	return []string{label}, nil
}

// hasOpenBeadsForLabelWithClient is a testable version that accepts a bead client.
// It threads the provided context through to ListWithLabel.
func hasOpenBeadsForLabelWithClient(ctx context.Context, label string, client interface {
	ListWithLabel(context.Context, string) ([]*bead.Bead, error)
}) (bool, error) {
	beads, err := client.ListWithLabel(ctx, label)
	if err != nil {
		return false, err
	}
	return len(beads) > 0, nil
}

func hasOpenBeadsForLabel(label string) (bool, error) {
	client, err := bead.NewClient()
	if err != nil {
		return false, err
	}
	beads, err := client.ListWithLabel(context.Background(), label)
	if err != nil {
		return false, err
	}
	return len(beads) > 0, nil
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
