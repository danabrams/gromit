package main

import "github.com/spf13/cobra"

type benchmarkRunOptions struct {
	ManifestPath    string
	OutputTimestamp string
}

var benchmarkRunPipelineFn = runBenchmarkPipeline

var benchmarkManifestPath string
var benchmarkOutputTS string

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run methodology benchmark workflows",
}

var benchmarkRunCmd = &cobra.Command{
	Use:          "run",
	Short:        "Run benchmark pipeline",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return benchmarkRunPipelineFn(benchmarkRunOptions{
			ManifestPath:    benchmarkManifestPath,
			OutputTimestamp: benchmarkOutputTS,
		})
	},
}

func init() {
	benchmarkRunCmd.Flags().StringVar(&benchmarkManifestPath, "manifest", "", "Path to benchmark manifest")
	benchmarkRunCmd.Flags().StringVar(&benchmarkOutputTS, "output-ts", "", "Timestamp override for deterministic artifact names")
	_ = benchmarkRunCmd.MarkFlagRequired("manifest")

	benchmarkCmd.AddCommand(benchmarkRunCmd)
	rootCmd.AddCommand(benchmarkCmd)
}

func runBenchmarkPipeline(opts benchmarkRunOptions) error {
	return nil
}
