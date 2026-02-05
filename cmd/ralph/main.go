package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/danabrams/ralph-runner/internal/config"
	"github.com/danabrams/ralph-runner/internal/runner"
	"github.com/spf13/cobra"
)

var (
	configPath    string
	maxIterations int
	dryRun        bool
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

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "config.yaml", "Path to config file")

	runCmd.Flags().IntVarP(&maxIterations, "max-iterations", "n", 0, "Maximum iterations (0 = unlimited)")
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would run without executing")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(statusCmd)
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		// Try to load from current directory if path doesn't exist
		if os.IsNotExist(err) && configPath == "config.yaml" {
			// Create default config
			return &config.Config{
				Models: config.ModelsConfig{
					P0:         "opus",
					P1:         "sonnet",
					P2:         "haiku",
					Validation: "haiku",
					Labels:     map[string]string{},
				},
				Escalation: config.EscalationConfig{
					Enabled:            true,
					Chain:              []string{"haiku", "sonnet", "opus"},
					MaxRetriesPerModel: 1,
				},
				Loop: config.LoopConfig{
					MaxIterations: 0,
					StopOnFailure: false,
				},
				Validation: config.ValidationConfig{
					Enabled:  true,
					Commands: []string{"pnpm run test", "pnpm run lint:check", "pnpm run build"},
				},
				Claude: config.ClaudeConfig{
					Binary:  "claude",
					Timeout: 600,
					Flags:   []string{"--dangerously-skip-permissions"},
				},
				Paths: config.PathsConfig{
					Templates:       "./templates",
					Specs:           "./specs",
					ProjectClaudeMD: "./CLAUDE.md",
				},
			}, nil
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
	return r.Run(ctx, cfg.Loop.MaxIterations, dryRun)
}

func showStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	r := runner.NewRunner(cfg, os.Stdout)
	return r.Status()
}
