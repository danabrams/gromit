package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/danabrams/ralph-runner/internal/config"
	"github.com/danabrams/ralph-runner/internal/retro"
	"github.com/danabrams/ralph-runner/internal/runner"
	"github.com/danabrams/ralph-runner/internal/state"
	"github.com/spf13/cobra"
)

var (
	configPath      string
	maxIterations   int
	dryRun          bool
	nonInteractive  bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "ralph",
	Short: "Ralph Runner - Execute the Ralph Wiggum loop correctly",
	Long: `Ralph Runner executes AI coding tasks with fresh context on each iteration.

It integrates with bd (beads) for task management and uses model escalation
for handling failures efficiently.`,
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the Ralph loop",
	Long: `Execute the Ralph loop, processing beads one at a time with fresh context.

Each iteration:
1. Gets the next unblocked bead from bd
2. Selects the appropriate model based on priority/labels
3. Invokes Claude with a fresh context
4. Runs validation (optional)
5. Closes the bead on success
6. Escalates to a stronger model on failure`,
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
6. Launches Claude Code for interactive review and application

Use --non-interactive to skip Claude Code and write analysis to .ralph/RETRO_PROPOSED_CHANGES.md instead.`,
	RunE: runRetro,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "ralph.yaml", "Path to config file")

	runCmd.Flags().IntVarP(&maxIterations, "max-iterations", "n", 0, "Maximum iterations (0 = unlimited)")
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would run without executing")

	retroCmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Skip Claude Code and write analysis to .ralph/RETRO_PROPOSED_CHANGES.md")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(retroCmd)
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		// If ralph.yaml doesn't exist, provide helpful error
		if os.IsNotExist(err) && configPath == "ralph.yaml" {
			return nil, fmt.Errorf("ralph.yaml not found - run 'ralph init' to set up this project")
		}
		return nil, err
	}
	return cfg, nil
}

func runLoop(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nReceived interrupt, stopping after current iteration...")
		cancel()
	}()

	// Override max iterations from flag if set
	if maxIterations > 0 {
		cfg.Loop.MaxIterations = maxIterations
	}

	r := runner.NewRunner(cfg, os.Stdout)
	if r == nil {
		return fmt.Errorf("failed to create runner")
	}
	return r.Run(ctx, cfg.Loop.MaxIterations, dryRun)
}

func showStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	r := runner.NewRunner(cfg, os.Stdout)
	if r == nil {
		return fmt.Errorf("failed to create runner")
	}
	return r.Status()
}

func runRetro(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Determine .ralph directory from config
	ralphDir := cfg.Paths.RalphDir
	if ralphDir == "" {
		// Default to .ralph in current directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
		ralphDir = filepath.Join(cwd, ".ralph")
	}

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

	// Run retrospective
	fmt.Println("Running retrospective analysis...")
	fmt.Println("This may take a few minutes as it uses opus for quality analysis.")

	r := retro.NewRetro(cfg, ralphDir)
	if r == nil {
		return fmt.Errorf("failed to create retro analyzer")
	}

	// Don't apply changes automatically - we'll handle that after review
	result, err := r.Run(ctx, false)
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
		analysisPath := filepath.Join(ralphDir, "RETRO_PROPOSED_CHANGES.md")
		if writeErr := os.WriteFile(analysisPath, []byte(result.Analysis), 0644); writeErr != nil {
			return fmt.Errorf("writing analysis file: %w", writeErr)
		}

		fmt.Printf("\nAnalysis written to %s\n", analysisPath)
		fmt.Println("Review and apply manually.")
		return nil
	}

	// Default: Launch Claude Code for interactive review and application
	fmt.Println("\nLaunching Claude Code for interactive review...")
	if err := retro.LaunchClaudeCode(result.Analysis); err != nil {
		return fmt.Errorf("launching Claude Code: %w", err)
	}

	// Record retro time in state
	sf := state.NewFile(ralphDir)
	if err := sf.RecordRetro(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not record retro time: %v\n", err)
	}

	return nil
}
