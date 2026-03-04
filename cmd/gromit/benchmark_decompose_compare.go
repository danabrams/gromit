package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	benchpkg "github.com/danabrams/gromit/internal/benchmark"
	"github.com/spf13/cobra"
)

type benchmarkDecomposeCompareOptions struct {
	ManifestPath        string
	SpecOverrides       []string
	FailureThreshold    float64
	FailureThresholdSet bool
	OutputTS            string
}

type benchmarkDecomposeCompareCohortSelectorOptions struct {
	ManifestPath  string
	SpecOverrides []string
}

type benchmarkDecomposeCompareRunnerOptions struct {
	Specs               []string
	FailureThreshold    float64
	FailureThresholdSet bool
}

type benchmarkDecomposeCompareReportWriterOptions struct {
	OutputTS string
}

var benchmarkRunDecomposeCompareFn = runBenchmarkDecomposeCompare
var benchmarkDecomposeCompareCohortSelectorFn = selectDecomposeCompareCohort
var benchmarkDecomposeCompareRunnerFn = runDecomposeCompare
var benchmarkDecomposeCompareReportWriterFn = writeDecomposeCompareReport
var benchmarkDecomposeCompareManifestPath string
var benchmarkDecomposeCompareOutputTS string
var benchmarkDecomposeCompareSpecs string
var benchmarkDecomposeCompareThreshold float64
var benchmarkDecomposeCompareExperimental bool

var benchmarkDecomposeCompareCmd = &cobra.Command{
	Use:          "decompose-compare",
	Short:        "Compare decomposition quality between model tiers",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !benchmarkDecomposeCompareExperimental {
			return fmt.Errorf("decompose-compare is experimental; run 'gromit benchmark --experimental decompose-compare' to enable it")
		}
		manifestPath := strings.TrimSpace(benchmarkDecomposeCompareManifestPath)
		if manifestPath == "" {
			return fmt.Errorf("--manifest must be a non-empty path")
		}
		specOverrides := parseCSV(benchmarkDecomposeCompareSpecs)
		if len(specOverrides) > 0 {
			if len(specOverrides) != 5 {
				return fmt.Errorf("--specs must include exactly 5 spec ids")
			}
			seen := make(map[string]struct{}, len(specOverrides))
			for _, spec := range specOverrides {
				if _, duplicate := seen[spec]; duplicate {
					return fmt.Errorf("--specs must not include duplicate spec ids")
				}
				seen[spec] = struct{}{}
			}
		}
		thresholdSet := cmd.Flags().Changed("threshold")
		if thresholdSet {
			if benchmarkDecomposeCompareThreshold < 0 || benchmarkDecomposeCompareThreshold > 1 {
				return fmt.Errorf("--threshold must be between 0 and 1")
			}
		}
		if benchmarkDecomposeCompareOutputTS != "" {
			if _, err := time.Parse("20060102T150405Z", benchmarkDecomposeCompareOutputTS); err != nil {
				return fmt.Errorf("--output-ts must be in UTC format YYYYMMDDTHHMMSSZ")
			}
		}
		return benchmarkRunDecomposeCompareFn(benchmarkDecomposeCompareOptions{
			ManifestPath:        manifestPath,
			SpecOverrides:       specOverrides,
			FailureThreshold:    benchmarkDecomposeCompareThreshold,
			FailureThresholdSet: thresholdSet,
			OutputTS:            benchmarkDecomposeCompareOutputTS,
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
	benchmarkDecomposeCompareCmd.Flags().StringVar(
		&benchmarkDecomposeCompareSpecs,
		"specs",
		"",
		"Comma-separated list of 5 spec ids to compare (no duplicates)",
	)
	benchmarkDecomposeCompareCmd.Flags().Float64Var(
		&benchmarkDecomposeCompareThreshold,
		"threshold",
		0,
		"Failure threshold tuning (0 ≤ threshold ≤ 1)",
	)
}

func runBenchmarkDecomposeCompare(opts benchmarkDecomposeCompareOptions) error {
	// Select cohort
	cohort, err := benchmarkDecomposeCompareCohortSelectorFn(benchmarkDecomposeCompareCohortSelectorOptions{
		ManifestPath:  opts.ManifestPath,
		SpecOverrides: opts.SpecOverrides,
	})
	if err != nil {
		return fmt.Errorf("select cohort: %w", err)
	}

	// Validate cohort size - must be exactly 5 specs
	if len(cohort) != 5 {
		return fmt.Errorf("insufficient cohort: got %d specs, require exactly 5", len(cohort))
	}

	// Run compare
	_, err = benchmarkDecomposeCompareRunnerFn(benchmarkDecomposeCompareRunnerOptions{
		Specs:               append([]string(nil), cohort...),
		FailureThreshold:    opts.FailureThreshold,
		FailureThresholdSet: opts.FailureThresholdSet,
	})
	if err != nil {
		return fmt.Errorf("run compare: %w", err)
	}

	// Write report
	ts := opts.OutputTS
	if ts == "" {
		ts = time.Now().UTC().Format("20060102T150405Z")
	}
	if err := benchmarkDecomposeCompareReportWriterFn(benchmarkDecomposeCompareReportWriterOptions{
		OutputTS: ts,
	}); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	// Print artifact paths
	fmt.Printf(".gromit/benchmarks/results/decompose-haiku-vs-sonnet/%s/raw.json\n", ts)
	fmt.Printf(".gromit/benchmarks/results/decompose-haiku-vs-sonnet/%s/summary.md\n", ts)

	return nil
}

func selectDecomposeCompareCohort(opts benchmarkDecomposeCompareCohortSelectorOptions) ([]string, error) {
	selector, err := buildDecomposeCohortSelector()
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if len(opts.SpecOverrides) > 0 {
		if err := selector.ValidateOverrides(ctx, opts.SpecOverrides); err != nil {
			return nil, err
		}
		return append([]string(nil), opts.SpecOverrides...), nil
	}

	manifest, err := benchpkg.LoadManifest(opts.ManifestPath)
	if err != nil {
		return nil, err
	}
	return selector.Select(ctx, manifest.Beads)
}

func runDecomposeCompare(opts benchmarkDecomposeCompareRunnerOptions) (interface{}, error) {
	return nil, fmt.Errorf("not implemented yet")
}

func writeDecomposeCompareReport(opts benchmarkDecomposeCompareReportWriterOptions) error {
	return fmt.Errorf("not implemented yet")
}

func buildDecomposeCohortSelector() (*benchpkg.DecomposeCohortSelector, error) {
	client, err := bead.NewClient()
	if err != nil {
		return nil, fmt.Errorf("create bead client: %w", err)
	}

	return benchpkg.NewDecomposeCohortSelector(client, resolvePlansDir(nil)), nil
}
