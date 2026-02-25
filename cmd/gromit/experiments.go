package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/experiment"
	"github.com/spf13/cobra"
)

var experimentsJSON bool

var experimentsCmd = &cobra.Command{
	Use:   "experiments",
	Short: "Report on configured experiments",
	Long: `Load experiment definitions and present the
Thompson sampling summary from the saved bandit state.`,
	RunE: runExperiments,
}

func init() {
	experimentsCmd.Flags().BoolVar(&experimentsJSON, "json", false, "Output experiments report as JSON")
	rootCmd.AddCommand(experimentsCmd)
}

func runExperiments(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("loading config: %w", err)
		}
		cfg = nil
	}

	experimentsDir := resolveExperimentsDir(cfg)
	experiments, err := experiment.LoadExperiments(experimentsDir)
	if err != nil {
		return fmt.Errorf("loading experiments: %w", err)
	}

	if len(experiments) == 0 {
		fmt.Printf("No experiments defined; add YAML files to %s\n", experimentsDir)
		return nil
	}

	report, err := experiment.GenerateReport(experiments, experimentsDir)
	if err != nil {
		return fmt.Errorf("generating experiment report: %w", err)
	}

	if experimentsJSON {
		fmt.Println(report.FormatReportJSON())
		return nil
	}

	fmt.Print(report.FormatReport())
	return nil
}

func resolveExperimentsDir(cfg *config.Config) string {
	if cfg != nil && cfg.Experiment.ExperimentsDir != "" {
		return cfg.Experiment.ExperimentsDir
	}
	return filepath.Join(resolveGromitDir(cfg), "experiments")
}
