package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
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

	queueInput := pipeline.QueueInput{
		LogsDir:        cfg.Paths.Logs,
		StuckThreshold: cfg.Loop.StuckBeadThreshold,
	}

	// If a bead ID was provided, unstick that specific bead
	if len(args) > 0 {
		beadID := args[0]
		if err := executor.Unstick(cmd.Context(), beadID, gromitDir); err != nil {
			return err
		}
		fmt.Printf("✓ Unsticked %s\n", beadID)
		return nil
	}

	// No arguments - list stuck beads
	stuckBeads, err := executor.ListStuck(cmd.Context(), queueInput)
	if err != nil {
		return err
	}

	if len(stuckBeads) == 0 {
		fmt.Println("No stuck beads")
		return nil
	}

	// Display numbered list of stuck beads
	fmt.Println("Stuck beads:")
	for i, b := range stuckBeads {
		fmt.Printf("%d. %s - %s\n", i+1, b.ID, b.Title)
	}

	// Read user selection
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("\nSelect bead to unstick [1-" + strconv.Itoa(len(stuckBeads)) + "]: ")
	if !scanner.Scan() {
		return nil
	}

	choice := strings.TrimSpace(scanner.Text())
	index, err := strconv.Atoi(choice)
	if err != nil || index < 1 || index > len(stuckBeads) {
		fmt.Println("Invalid selection")
		return nil
	}

	// Unstick the selected bead
	selectedBead := stuckBeads[index-1]
	if err := executor.Unstick(cmd.Context(), selectedBead.ID, gromitDir); err != nil {
		return err
	}

	fmt.Printf("✓ Unsticked %s\n", selectedBead.ID)
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
	Unstick(context.Context, string, string) error
	ListStuck(context.Context, pipeline.QueueInput) ([]*bead.Bead, error)
}
