package main

import (
	"fmt"

	v2spec "github.com/danabrams/gromit/internal/v2/spec"
	"github.com/spf13/cobra"
)

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "List specs whose dependencies are satisfied",
	Long:  `List specs that have all dependencies accepted and are not yet accepted themselves.`,
	Args:  cobra.NoArgs,
	RunE:  runReady,
}

func runReady(cmd *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	specsDir := resolveSpecsDir(cfg)
	readySpecs, err := v2spec.ListReady(specsDir)
	if err != nil {
		return fmt.Errorf("listing ready specs: %w", err)
	}

	out := cmd.OutOrStdout()
	for _, readySpec := range readySpecs {
		fmt.Fprintf(out, "%s\t%s\n", readySpec.ID, readySpec.Path)
	}

	return nil
}
