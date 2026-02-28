package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type benchmarkDecomposeCompareOptions struct {
	ManifestPath string
	OutputTS     string
}

var benchmarkRunDecomposeCompareFn = runBenchmarkDecomposeCompare
var benchmarkDecomposeCompareManifestPath string
var benchmarkDecomposeCompareOutputTS string

var benchmarkDecomposeCompareCmd = &cobra.Command{
	Use:          "decompose-compare",
	Short:        "Compare decomposition quality between model tiers",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		manifestPath := strings.TrimSpace(benchmarkDecomposeCompareManifestPath)
		if manifestPath == "" {
			return fmt.Errorf("--manifest must be a non-empty path")
		}
		if benchmarkDecomposeCompareOutputTS != "" {
			if _, err := time.Parse("20060102T150405Z", benchmarkDecomposeCompareOutputTS); err != nil {
				return fmt.Errorf("--output-ts must be in UTC format YYYYMMDDTHHMMSSZ")
			}
		}
		return benchmarkRunDecomposeCompareFn(benchmarkDecomposeCompareOptions{
			ManifestPath: manifestPath,
			OutputTS:     benchmarkDecomposeCompareOutputTS,
		})
	},
}

func init() {
	benchmarkDecomposeCompareCmd.Flags().StringVar(
		&benchmarkDecomposeCompareManifestPath,
		"manifest",
		"",
		"Path to benchmark manifest",
	)
	_ = benchmarkDecomposeCompareCmd.MarkFlagRequired("manifest")
	benchmarkDecomposeCompareCmd.Flags().StringVar(
		&benchmarkDecomposeCompareOutputTS,
		"output-ts",
		"",
		"Timestamp override for deterministic artifact names",
	)
}

func registerBenchmarkDecomposeCompareCommand(root *cobra.Command) {
	benchmarkCmd.AddCommand(benchmarkDecomposeCompareCmd)
}

func runBenchmarkDecomposeCompare(opts benchmarkDecomposeCompareOptions) error {
	return fmt.Errorf("not implemented yet")
}
