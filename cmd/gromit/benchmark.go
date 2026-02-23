package main

import "github.com/spf13/cobra"

type benchmarkRunOptions struct {
	ManifestPath    string
	OutputTimestamp string
	BaseCommit      string
}

var benchmarkRunPipelineFn = runBenchmarkPipeline
var benchmarkLoadManifestFn = loadBenchmarkManifest
var benchmarkSelectCohortFn = selectBenchmarkCohort
var benchmarkValidateCohortFn = validateBenchmarkCohort
var benchmarkRunHarnessFn = runBenchmarkHarness
var benchmarkComputeMetricsFn = computeBenchmarkMetrics
var benchmarkWriteReportFn = writeBenchmarkReport

var benchmarkManifestPath string
var benchmarkOutputTS string
var benchmarkBaseCommit string

type benchmarkManifest struct {
	ID string
}

type benchmarkSelection struct {
	SelectedBeads []string
}

type benchmarkValidatedCohort struct {
	SelectedBeads []string
}

type benchmarkHarnessResult struct {
	BaseCommit    string
	SelectedBeads []string
}

type benchmarkMetricsResult struct{}

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
			BaseCommit:      benchmarkBaseCommit,
		})
	},
}

func init() {
	benchmarkRunCmd.Flags().StringVar(&benchmarkManifestPath, "manifest", "", "Path to benchmark manifest")
	benchmarkRunCmd.Flags().StringVar(&benchmarkOutputTS, "output-ts", "", "Timestamp override for deterministic artifact names")
	benchmarkRunCmd.Flags().StringVar(&benchmarkBaseCommit, "base-commit", "", "Base commit override for benchmark runs")
	_ = benchmarkRunCmd.MarkFlagRequired("manifest")

	benchmarkCmd.AddCommand(benchmarkRunCmd)
	rootCmd.AddCommand(benchmarkCmd)
}

func runBenchmarkPipeline(opts benchmarkRunOptions) error {
	manifest, err := benchmarkLoadManifestFn(opts.ManifestPath)
	if err != nil {
		return err
	}
	selection, err := benchmarkSelectCohortFn(manifest, opts)
	if err != nil {
		return err
	}
	cohort, err := benchmarkValidateCohortFn(selection)
	if err != nil {
		return err
	}
	harnessResult, err := benchmarkRunHarnessFn(manifest, cohort, opts)
	if err != nil {
		return err
	}
	metrics, err := benchmarkComputeMetricsFn(harnessResult)
	if err != nil {
		return err
	}
	if err := benchmarkWriteReportFn(manifest, harnessResult, metrics, opts); err != nil {
		return err
	}
	return nil
}

func loadBenchmarkManifest(path string) (benchmarkManifest, error) {
	return benchmarkManifest{}, nil
}

func selectBenchmarkCohort(manifest benchmarkManifest, opts benchmarkRunOptions) (benchmarkSelection, error) {
	return benchmarkSelection{}, nil
}

func validateBenchmarkCohort(selection benchmarkSelection) (benchmarkValidatedCohort, error) {
	return benchmarkValidatedCohort{SelectedBeads: selection.SelectedBeads}, nil
}

func runBenchmarkHarness(manifest benchmarkManifest, cohort benchmarkValidatedCohort, opts benchmarkRunOptions) (benchmarkHarnessResult, error) {
	return benchmarkHarnessResult{}, nil
}

func computeBenchmarkMetrics(result benchmarkHarnessResult) (benchmarkMetricsResult, error) {
	return benchmarkMetricsResult{}, nil
}

func writeBenchmarkReport(manifest benchmarkManifest, result benchmarkHarnessResult, metrics benchmarkMetricsResult, opts benchmarkRunOptions) error {
	return nil
}
