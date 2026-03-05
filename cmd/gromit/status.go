package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/danabrams/gromit/internal/runner"
	"github.com/spf13/cobra"
)

var (
	statusSPC  bool
	statusJSON bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current queue status",
	Long:  `Display information about the next bead in the queue and what model would be used.`,
	RunE:  showStatus,
}

func registerStatusCommand(root *cobra.Command) {
	if statusCmd.Flags().Lookup("spc") == nil {
		statusCmd.Flags().BoolVar(&statusSPC, "spc", false, "Show SPC dashboard status only")
		statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Show status output as JSON")
	}
	if statusCmd.Parent() == nil {
		root.AddCommand(statusCmd)
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
