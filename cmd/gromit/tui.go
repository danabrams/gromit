package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner"
	"github.com/danabrams/gromit/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch Gromit TUI",
	RunE:  runTui,
}

var osExecutable = os.Executable

type teaProgram interface {
	Run() (tea.Model, error)
}

var newTeaProgram = func(model tea.Model) teaProgram {
	return tea.NewProgram(model)
}

var hydrateStoreFn = func(ctx context.Context, cfg *config.Config, gromitDir, specsDir, plansDir string, provider tui.HydrationProvider) *tui.Store {
	return tui.HydrateStore(ctx, cfg, gromitDir, specsDir, plansDir, provider)
}

var newHydrationProvider = func(cfg *config.Config) tui.HydrationProvider {
	return &defaultHydrationProvider{cfg: cfg}
}

type pendingActionProvider interface {
	PendingAction() *tui.PendingAction
}

var runTuiLoadConfig = loadConfig

var executePendingAction = func(action *tui.PendingAction) error {
	cmd, err := buildPendingActionCommand(action)
	if err != nil {
		return err
	}
	return cmd.Run()
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTui(_ *cobra.Command, _ []string) error {
	cfg, err := runTuiLoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if cfg == nil {
		cfg = &config.Config{}
	}
	cfg.SetDefaults()

	ctx := context.Background()
	gromitDir := resolveGromitDir(cfg)
	specsDir := resolveSpecsDir(cfg)
	plansDir := resolvePlansDir(cfg)
	provider := newHydrationProvider(cfg)

	for {
		store := hydrateStoreFn(ctx, cfg, gromitDir, specsDir, plansDir, provider)
		model := tui.NewModel(store)

		program := newTeaProgram(model)
		finalModel, err := program.Run()
		if err != nil {
			return err
		}

		pendingModel, ok := finalModel.(pendingActionProvider)
		if !ok {
			return fmt.Errorf("tui program returned unexpected model type %T", finalModel)
		}

		if pending := pendingModel.PendingAction(); pending == nil {
			break
		} else if err := executePendingAction(pending); err != nil {
			return err
		}
	}

	return nil
}

func buildPendingActionCommand(action *tui.PendingAction) (*exec.Cmd, error) {
	if action == nil || action.Command == "" {
		return nil, errors.New("pending action missing command")
	}

	executable, err := osExecutable()
	if err != nil {
		return nil, err
	}

	args := append([]string{action.Command}, action.Args...)
	cmd := exec.Command(executable, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, nil
}

type defaultHydrationProvider struct {
	cfg *config.Config
}

func (d *defaultHydrationProvider) RunnerStatus(ctx context.Context, gromitDir string) (*runner.Status, error) {
	return runner.ReadStatus(gromitDir)
}

func (d *defaultHydrationProvider) PipelineStatus(ctx context.Context, gromitDir, specsDir, plansDir string, startedAt *time.Time) (*pipeline.PipelineStatus, error) {
	return pipeline.ReadStatus(gromitDir, specsDir, plansDir, startedAt)
}

func (d *defaultHydrationProvider) PipelineItems(ctx context.Context, gromitDir, specsDir, plansDir string) (tui.PipelineItems, error) {
	items := tui.PipelineItems{}

	backlogFile, err := backlog.NewFile(gromitDir)
	if err != nil {
		return items, fmt.Errorf("creating backlog file: %w", err)
	}

	ideas, err := backlogFile.List()
	if err != nil {
		return items, fmt.Errorf("loading backlog: %w", err)
	}
	for _, idea := range ideas {
		if idea == nil {
			continue
		}
		items.BacklogIdeas = append(items.BacklogIdeas, *idea)
	}

	unplanned, err := pipeline.ListUnplannedSpecs(specsDir, plansDir)
	if err != nil {
		return items, fmt.Errorf("listing unplanned specs: %w", err)
	}
	items.UnplannedSpecs = unplanned

	undecomposed, err := pipeline.ListUndecomposedPlans(plansDir)
	if err != nil {
		return items, fmt.Errorf("listing undecomposed plans: %w", err)
	}
	items.UndecomposedPlans = undecomposed

	if ctx == nil {
		ctx = context.Background()
	}

	repoRoot := filepath.Dir(gromitDir)
	if repoRoot == "" {
		repoRoot = "."
	}

	if hasBeadsRepo(repoRoot) {
		client, err := bead.NewClient()
		if err != nil {
			return items, fmt.Errorf("creating bead client: %w", err)
		}
		client.Dir = repoRoot

		beads, err := pipeline.ListActiveBeads(ctx, client)
		if err != nil {
			return items, fmt.Errorf("listing active beads: %w", err)
		}
		for _, beadItem := range beads {
			if beadItem == nil {
				continue
			}
			items.Beads = append(items.Beads, *beadItem)
		}
	}

	return items, nil
}

func hasBeadsRepo(repoRoot string) bool {
	if repoRoot == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(repoRoot, ".beads"))
	return err == nil && info.IsDir()
}
