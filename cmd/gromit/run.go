package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner"
	"github.com/danabrams/gromit/internal/scope"
	"github.com/spf13/cobra"
)

const (
	runSignalBufferSize = 2
	gracefulStopMessage = "\nReceived interrupt, stopping after current iteration (Ctrl+C again to force stop)..."
)

var (
	maxIterations              int
	timeBudgetMinutes          int
	timeBudgetHours            int
	readinessEmergencyOverride bool
	runSpecFlag                string
	runEpicFlag                string
)

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

var (
	newRunnerWithStageContextFn = runner.NewRunnerWithStageContext
	newBuildSpecStageContextFn  = runner.BuildSpecStageContext
	newSpecflowStoreFn          = runner.SpecflowStoreFactory
	newSpecBranchCreatorFn      = runner.SpecBranchCreatorFactory
	runInDedicatedWorktreeFn    = runInDedicatedWorktree
	runHasOpenBeadsForLabelFn   = hasOpenBeadsForLabel
)

func registerRunCommand(root *cobra.Command) {
	if runCmd.Flags().Lookup("max-iterations") == nil {
		runCmd.Flags().IntVarP(&maxIterations, "max-iterations", "n", 0, "Maximum iterations (0 = unlimited)")
		runCmd.Flags().IntVarP(&timeBudgetMinutes, "time-budget", "t", 0, "Time budget in minutes (0 = unlimited)")
		runCmd.Flags().IntVarP(&timeBudgetHours, "time-budget-hours", "H", 0, "Time budget in hours (0 = unlimited)")
		runCmd.Flags().StringVar(&runSpecFlag, "spec", "", "Filter to beads for a specific spec")
		runCmd.Flags().StringVar(&runEpicFlag, "epic", "", "Filter to beads for a specific epic")
		runCmd.Flags().BoolVar(&readinessEmergencyOverride, "readiness-emergency-override", false, "Allow bypass of the readiness gate in emergencies")
	}
	if runCmd.Parent() == nil {
		root.AddCommand(runCmd)
	}
}

func runLoop(cmd *cobra.Command, args []string) error {
	if err := scope.ValidateFlags(runEpicFlag, runSpecFlag); err != nil {
		return err
	}

	mainDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determining main dir: %w", err)
	}

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

		if maxIterations > 0 {
			cfg.Loop.MaxIterations = maxIterations
		}

		var deadline time.Time
		if timeBudgetMinutes > 0 || timeBudgetHours > 0 {
			totalMinutes := timeBudgetMinutes + timeBudgetHours*60
			deadline = time.Now().Add(time.Duration(totalMinutes) * time.Minute)
		}

		gromitDir := resolveGromitDir(cfg)
		var stageCtx *runner.StageContext
		if runSpecFlag != "" {
			stageCtx, err = buildRunSpecStageContext(ctx, cfg, runSpecFlag, gromitDir)
			if err != nil {
				return fmt.Errorf("initializing specflow stage: %w", err)
			}
			if stageCtx != nil && stageCtx.SpecName != "" {
				repoDir, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("determining repo dir: %w", err)
				}
				if err := runner.EnsureSpecBranch(ctx, cfg, stageCtx, repoDir, newSpecBranchCreatorFn); err != nil {
					return fmt.Errorf("preparing spec branch: %w", err)
				}
			}
		}

		r, err := newRunnerWithStageContextFn(cfg, os.Stdout, stageCtx, labels...)
		if err != nil {
			return fmt.Errorf("failed to create runner: %w", err)
		}
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

func buildRunSpecStageContext(ctx context.Context, cfg *config.Config, specName, gromitDir string) (*runner.StageContext, error) {
	if specName == "" {
		return nil, nil
	}
	return newBuildSpecStageContextFn(ctx, cfg, specName, gromitDir, newSpecflowStoreFn)
}

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
