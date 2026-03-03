package main

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/spf13/cobra"
)

var unstickCmd = &cobra.Command{
	Use:   "unstick [bead-id]",
	Short: "Unstick a bead or list stuck beads",
	Long: `Unstick a bead to mark it as no longer blocked.

With a bead ID argument, unsticks that specific bead.
Without arguments, lists all stuck beads and prompts for selection.`,
	RunE: runUnstick,
}

func init() {
	rootCmd.AddCommand(unstickCmd)
}

func runUnstick(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	gromitDir := resolveGromitDir(cfg)
	executor, err := createUnstickPipelineFn(cfg, gromitDir)
	if err != nil {
		return fmt.Errorf("creating pipeline: %w", err)
	}

	// If a bead ID was provided, unstick that specific bead
	if len(args) > 0 {
		beadID := args[0]
		result, err := executor.Unstick(cmd.Context(), beadID)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Unsticked %s (%s)\n", beadID, result.Status)
		return nil
	}

	// No arguments - list stuck beads
	result, err := executor.ListStuck(cmd.Context())
	if err != nil {
		return err
	}

	if len(result.StuckBeads) == 0 {
		fmt.Println("No stuck beads")
		return nil
	}

	// TODO: implement interactive selection for stuck beads
	return nil
}

var createUnstickPipelineFn = createUnstickPipeline

func createUnstickPipeline(cfg *config.Config, gromitDir string) (unstickExecutor, error) {
	p, err := newPipeline(cfg, gromitDir)
	if err != nil {
		return nil, err
	}
	return p, nil
}

type unstickExecutor interface {
	Unstick(context.Context, string) (*pipeline.UnstickResult, error)
	ListStuck(context.Context) (*pipeline.ListStuckResult, error)
}
