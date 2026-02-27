package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/spf13/cobra"
)

var boardCmd = &cobra.Command{
	Use:   "board",
	Short: "Show all beads grouped by status",
	Long: `Display all beads organized by status (open vs closed) with priority and title.

Open beads are sorted by priority (P0 first), while closed beads are listed without priority.`,
	RunE: runBoard,
}

func init() {
	rootCmd.AddCommand(boardCmd)
}

func runBoard(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	gromitDir := resolveGromitDir(cfg)
	executor, err := boardPipelineFactory(cfg, gromitDir)
	if err != nil {
		return fmt.Errorf("creating pipeline: %w", err)
	}

	data, err := executor.Board(cmd.Context())
	if err != nil {
		return err
	}
	if data == nil {
		data = &pipeline.BoardData{}
	}

	sort.Slice(data.Open, func(i, j int) bool {
		return data.Open[i].Priority < data.Open[j].Priority
	})

	fmt.Printf("Open (%d)\n", len(data.Open))
	if len(data.Open) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, b := range data.Open {
			priorityLabel := fmt.Sprintf("P%d", b.Priority)
			fmt.Printf("  %-3s %-20s %s\n", priorityLabel, b.ID, b.Title)
		}
	}

	fmt.Println()

	fmt.Printf("Closed (%d)\n", len(data.Closed))
	if len(data.Closed) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, b := range data.Closed {
			fmt.Printf("  %-20s %s\n", b.ID, b.Title)
		}
	}

	return nil
}

var boardPipelineFactory = createBoardPipeline

func createBoardPipeline(cfg *config.Config, gromitDir string) (boardExecutor, error) {
	p, err := newPipeline(cfg, gromitDir)
	if err != nil {
		return nil, err
	}
	return p, nil
}

type boardExecutor interface {
	Board(context.Context) (*pipeline.BoardData, error)
}
