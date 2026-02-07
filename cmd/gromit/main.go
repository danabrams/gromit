package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/retro"
	"github.com/danabrams/gromit/internal/runner"
	"github.com/danabrams/gromit/internal/state"
	"github.com/spf13/cobra"
)

var (
	configPath        string
	maxIterations     int
	dryRun            bool
	nonInteractive    bool
	timeBudgetMinutes int
	timeBudgetHours   int
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "gromit",
	Short: "Gromit - Execute the Gromit loop correctly",
	Long: `Gromit executes AI coding tasks with fresh context on each iteration.

It integrates with bd (beads) for task management and uses model escalation
for handling failures efficiently.`,
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the Gromit loop",
	Long: `Execute the Gromit loop, processing beads one at a time with fresh context.

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

Use --non-interactive to skip Claude Code and write analysis to .gromit/RETRO_PROPOSED_CHANGES.md instead.`,
	RunE: runRetro,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "gromit.yaml", "Path to config file")

	runCmd.Flags().IntVarP(&maxIterations, "max-iterations", "n", 0, "Maximum iterations (0 = unlimited)")
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would run without executing")
	runCmd.Flags().IntVarP(&timeBudgetMinutes, "time-budget", "t", 0, "Time budget in minutes (0 = unlimited)")
	runCmd.Flags().IntVarP(&timeBudgetHours, "time-budget-hours", "H", 0, "Time budget in hours (0 = unlimited)")

	retroCmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Skip Claude Code and write analysis to .gromit/RETRO_PROPOSED_CHANGES.md")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(retroCmd)
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

	// Compute deadline from time budget flags (additive: total = minutes + hours*60)
	var deadline time.Time
	if timeBudgetMinutes > 0 || timeBudgetHours > 0 {
		totalMinutes := timeBudgetMinutes + timeBudgetHours*60
		deadline = time.Now().Add(time.Duration(totalMinutes) * time.Minute)
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	r, err := runner.NewRunner(cfg, os.Stdout)
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}
	return r.Run(ctx, cfg.Loop.MaxIterations, deadline, dryRun)
}

func showStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	r, err := runner.NewRunner(cfg, os.Stdout)
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}
	return r.Status()
}

func runRetro(cmd *cobra.Command, args []string) error {
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

	// Run retrospective
	fmt.Println("Running retrospective analysis...")
	fmt.Println("This may take a few minutes as it uses opus for quality analysis.")

	r, err := retro.NewRetro(cfg, gromitDir)
	if err != nil {
		return fmt.Errorf("failed to create retro analyzer: %w", err)
	}

	result, err := r.Run(ctx)
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

	// Default: Launch Claude Code for interactive review and application
	fmt.Println("\nLaunching Claude Code for interactive review...")
	if err := retro.LaunchClaudeCode(result.Analysis); err != nil {
		return fmt.Errorf("launching Claude Code: %w", err)
	}

	// Record retro time in state
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
