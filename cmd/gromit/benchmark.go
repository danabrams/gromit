package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	benchpkg "github.com/danabrams/gromit/internal/benchmark"
	"github.com/spf13/cobra"
)

type benchmarkRunOptions struct {
	ManifestPath    string
	OutputTimestamp string
	BaseCommit      string
	SingleBead      string
	Beads           []string
	BeadCount       int
}

var benchmarkRunPipelineFn = runBenchmarkPipeline
var benchmarkLoadManifestFn = loadBenchmarkManifest
var benchmarkSelectCohortFn = selectBenchmarkCohort
var benchmarkValidateCohortFn = validateBenchmarkCohort
var benchmarkRunHarnessFn = runBenchmarkHarness
var benchmarkComputeMetricsFn = computeBenchmarkMetrics
var benchmarkWritePhase3MeasurementReportFn = benchpkg.WritePhase3MeasurementReport
var benchmarkInternalLoadManifestFn = benchpkg.LoadManifest
var benchmarkInternalResolveSelectedBeadsFn = benchpkg.ResolveSelectedBeads
var benchmarkInternalValidateSelectedCohortFn = benchpkg.ValidateSelectedCohort
var benchmarkInternalRunModesInIsolatedWorktreesFn = benchpkg.RunModesInIsolatedWorktrees
var benchmarkInternalAggregateModeMetricsFn = benchpkg.AggregateModeMetrics
var benchmarkInternalWriteReportFn = benchpkg.WriteReport
var benchmarkBuildReportInputFn = benchpkg.BuildReportInput
var benchmarkNewBeadLookupFn = func() (benchpkg.BeadLookup, error) { return bead.NewClient() }
var benchmarkNewBaseCommitResolverFn = func() benchpkg.BaseCommitResolver {
	return benchpkg.NewGitBaseCommitResolver(nil)
}
var benchmarkNewModeWorktreeRunnerFn = func() benchpkg.ModeWorktreeRunner {
	cwd, _ := os.Getwd()
	return benchpkg.NewSessionModeWorktreeRunner(benchpkg.SessionModeWorktreeRunnerOptions{
		MainDir: cwd,
	})
}
var benchmarkNowFn = time.Now

var benchmarkManifestPath string
var benchmarkOutputTS string
var benchmarkBaseCommit string
var benchmarkSingleBead string
var benchmarkBeads string
var benchmarkBeadCount int
var benchmarkPhase3BaselineLog string
var benchmarkPhase3OptimizedLog string
var benchmarkPhase3OutputTS string

type benchmarkManifest struct {
	ID              string   `yaml:"id"`
	BaseCommit      string   `yaml:"base_commit"`
	Beads           []string `yaml:"beads"`
	Modes           []string `yaml:"modes"`
	Provider        string   `yaml:"provider"`
	ModelFamily     string   `yaml:"model_family"`
	LowTierModel    string   `yaml:"low_tier_model"`
	MediumTierModel string   `yaml:"medium_tier_model"`
	HighTierModel   string   `yaml:"high_tier_model"`
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
	Modes         []benchmarkModeResult
	ModeLogs      []benchpkg.ModeLogInput
}

type benchmarkModeResult struct {
	Mode          string   `json:"mode"`
	BaseCommit    string   `json:"base_commit"`
	SelectedBeads []string `json:"selected_beads"`
}

type benchmarkMetricsResult struct {
	ModeScores    []benchmarkModeScore `json:"mode_scores"`
	ModeSummaries []benchpkg.ModeSummary
}

type benchmarkModeScore struct {
	Mode  string `json:"mode"`
	Score int    `json:"score"`
}

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run methodology benchmark workflows",
}

var benchmarkRunCmd = &cobra.Command{
	Use:          "run",
	Short:        "Run benchmark pipeline",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		manifestPath := strings.TrimSpace(benchmarkManifestPath)
		if manifestPath == "" {
			return fmt.Errorf("--manifest must be a non-empty path")
		}
		if benchmarkBaseCommit != "" && strings.TrimSpace(benchmarkBaseCommit) == "" {
			return fmt.Errorf("--base-commit must be a non-empty commit reference")
		}
		if strings.ContainsAny(benchmarkBaseCommit, " \t\r\n") {
			return fmt.Errorf("--base-commit must not contain whitespace")
		}
		if benchmarkBeadCount < 0 {
			return fmt.Errorf("--bead-count must be zero or greater")
		}
		singleBead := strings.TrimSpace(benchmarkSingleBead)
		if benchmarkSingleBead != "" && singleBead == "" {
			return fmt.Errorf("--single-bead must be a non-empty bead id")
		}
		if strings.ContainsAny(singleBead, " \t\r\n") {
			return fmt.Errorf("--single-bead must not contain whitespace")
		}
		if benchmarkOutputTS != "" {
			if _, err := time.Parse("20060102T150405Z", benchmarkOutputTS); err != nil {
				return fmt.Errorf("--output-ts must be in UTC format YYYYMMDDTHHMMSSZ")
			}
		}
		beads := parseCSV(benchmarkBeads)
		if singleBead != "" {
			if strings.TrimSpace(benchmarkBeads) != "" {
				return fmt.Errorf("--single-bead cannot be combined with --beads")
			}
			if benchmarkBeadCount != 0 {
				return fmt.Errorf("--single-bead cannot be combined with --bead-count")
			}
		}
		if strings.TrimSpace(benchmarkBeads) != "" && len(beads) == 0 {
			return fmt.Errorf("--beads must include at least one bead id when provided")
		}
		return benchmarkRunPipelineFn(benchmarkRunOptions{
			ManifestPath:    manifestPath,
			OutputTimestamp: benchmarkOutputTS,
			BaseCommit:      benchmarkBaseCommit,
			SingleBead:      singleBead,
			Beads:           beads,
			BeadCount:       benchmarkBeadCount,
		})
	},
}

var benchmarkPhase3ReportCmd = &cobra.Command{
	Use:          "phase3-report",
	Short:        "Compute and publish phase-3 measurement report artifacts",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(benchmarkPhase3BaselineLog) == "" {
			return fmt.Errorf("--baseline-log must be a non-empty path")
		}
		if strings.TrimSpace(benchmarkPhase3OptimizedLog) == "" {
			return fmt.Errorf("--optimized-log must be a non-empty path")
		}
		if _, err := time.Parse("20060102T150405Z", benchmarkPhase3OutputTS); err != nil {
			return fmt.Errorf("--output-ts must be in UTC format YYYYMMDDTHHMMSSZ")
		}
		_, err := benchmarkWritePhase3MeasurementReportFn(benchpkg.Phase3MeasurementInput{
			Timestamp:        benchmarkPhase3OutputTS,
			BaselineLogPath:  benchmarkPhase3BaselineLog,
			OptimizedLogPath: benchmarkPhase3OptimizedLog,
		})
		return err
	},
}

func init() {
	benchmarkRunCmd.Flags().StringVar(&benchmarkManifestPath, "manifest", "", "Path to benchmark manifest")
	benchmarkRunCmd.Flags().StringVar(&benchmarkOutputTS, "output-ts", "", "Timestamp override for deterministic artifact names")
	benchmarkRunCmd.Flags().StringVar(&benchmarkBaseCommit, "base-commit", "", "Base commit override for benchmark runs")
	benchmarkRunCmd.Flags().StringVar(&benchmarkSingleBead, "single-bead", "", "Pilot mode: run benchmark modes on one bead id")
	benchmarkRunCmd.Flags().StringVar(&benchmarkBeads, "beads", "", "Comma-separated ordered bead IDs to benchmark")
	benchmarkRunCmd.Flags().IntVar(&benchmarkBeadCount, "bead-count", 0, "Optional deterministic truncation count for selected beads")
	_ = benchmarkRunCmd.MarkFlagRequired("manifest")

	benchmarkPhase3ReportCmd.Flags().StringVar(&benchmarkPhase3BaselineLog, "baseline-log", "", "Path to baseline JSONL run log")
	benchmarkPhase3ReportCmd.Flags().StringVar(&benchmarkPhase3OptimizedLog, "optimized-log", "", "Path to optimized JSONL run log")
	benchmarkPhase3ReportCmd.Flags().StringVar(&benchmarkPhase3OutputTS, "output-ts", "", "UTC timestamp for report artifact names")
	_ = benchmarkPhase3ReportCmd.MarkFlagRequired("baseline-log")
	_ = benchmarkPhase3ReportCmd.MarkFlagRequired("optimized-log")
	_ = benchmarkPhase3ReportCmd.MarkFlagRequired("output-ts")
}

func registerBenchmarkCommands(root *cobra.Command) {
	benchmarkCmd.AddCommand(benchmarkRunCmd)
	benchmarkCmd.AddCommand(benchmarkPhase3ReportCmd)
	root.AddCommand(benchmarkCmd)
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
	cohort, err := benchmarkValidateCohortFn(selection, opts)
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
	ts := opts.OutputTimestamp
	if ts == "" {
		ts = benchmarkNowFn().UTC().Format("20060102T150405Z")
	}
	modeSummaries := metrics.ModeSummaries
	if len(modeSummaries) == 0 {
		modeSummaries = make([]benchpkg.ModeSummary, 0, len(metrics.ModeScores))
		for _, score := range metrics.ModeScores {
			modeSummaries = append(modeSummaries, benchpkg.ModeSummary{Mode: score.Mode})
		}
	}
	input := benchmarkBuildReportInputFn(
		manifest.ID,
		harnessResult.BaseCommit,
		harnessResult.SelectedBeads,
		manifest.Provider,
		manifest.ModelFamily,
		manifest.LowTierModel,
		manifest.MediumTierModel,
		manifest.HighTierModel,
		modeSummaries,
		ts,
	)
	if _, err := benchmarkInternalWriteReportFn(input); err != nil {
		return err
	}
	return nil
}

func loadBenchmarkManifest(path string) (benchmarkManifest, error) {
	manifest, err := benchmarkInternalLoadManifestFn(path)
	if err != nil {
		return benchmarkManifest{}, err
	}
	return benchmarkManifest{
		ID:              manifest.ID,
		BaseCommit:      manifest.BaseCommit,
		Beads:           append([]string(nil), manifest.Beads...),
		Modes:           append([]string(nil), manifest.Modes...),
		Provider:        manifest.Provider,
		ModelFamily:     manifest.ModelFamily,
		LowTierModel:    manifest.LowTierModel,
		MediumTierModel: manifest.MediumTierModel,
		HighTierModel:   manifest.HighTierModel,
	}, nil
}

func selectBenchmarkCohort(manifest benchmarkManifest, opts benchmarkRunOptions) (benchmarkSelection, error) {
	if opts.SingleBead != "" {
		return benchmarkSelection{SelectedBeads: []string{opts.SingleBead}}, nil
	}
	selected, err := benchmarkInternalResolveSelectedBeadsFn(manifest.Beads, opts.Beads, opts.BeadCount)
	if err != nil {
		return benchmarkSelection{}, err
	}
	return benchmarkSelection{SelectedBeads: selected}, nil
}

func validateBenchmarkCohort(selection benchmarkSelection, opts benchmarkRunOptions) (benchmarkValidatedCohort, error) {
	lookup, err := benchmarkNewBeadLookupFn()
	if err != nil {
		return benchmarkValidatedCohort{}, fmt.Errorf("create bead lookup: %w", err)
	}
	requiredSize := 5
	requireTierCoverage := true
	if opts.SingleBead != "" {
		requiredSize = 1
		requireTierCoverage = false
	}
	selected, err := benchmarkInternalValidateSelectedCohortFn(lookup, selection.SelectedBeads, requiredSize, requireTierCoverage)
	if err != nil {
		return benchmarkValidatedCohort{}, err
	}
	return benchmarkValidatedCohort{SelectedBeads: selected}, nil
}

func runBenchmarkHarness(manifest benchmarkManifest, cohort benchmarkValidatedCohort, opts benchmarkRunOptions) (benchmarkHarnessResult, error) {
	runs, baseCommit, err := benchmarkInternalRunModesInIsolatedWorktreesFn(context.Background(), benchpkg.RunModesInput{
		Manifest: benchpkg.HarnessManifest{
			Provider:        manifest.Provider,
			ModelFamily:     manifest.ModelFamily,
			LowTierModel:    manifest.LowTierModel,
			MediumTierModel: manifest.MediumTierModel,
			HighTierModel:   manifest.HighTierModel,
		},
		Modes:          append([]string(nil), manifest.Modes...),
		SelectedBeads:  append([]string(nil), cohort.SelectedBeads...),
		BaseCommitHint: firstNonEmpty(opts.BaseCommit, manifest.BaseCommit),
		Resolver:       benchmarkNewBaseCommitResolverFn(),
		Runner:         benchmarkNewModeWorktreeRunnerFn(),
	})
	if err != nil {
		return benchmarkHarnessResult{}, err
	}
	result := benchmarkHarnessResult{
		BaseCommit:    baseCommit,
		SelectedBeads: append([]string(nil), cohort.SelectedBeads...),
		Modes:         make([]benchmarkModeResult, 0, len(runs)),
		ModeLogs:      make([]benchpkg.ModeLogInput, 0, len(runs)),
	}
	for _, run := range runs {
		result.Modes = append(result.Modes, benchmarkModeResult{
			Mode:          run.Mode,
			BaseCommit:    baseCommit,
			SelectedBeads: append([]string(nil), cohort.SelectedBeads...),
		})
		result.ModeLogs = append(result.ModeLogs, benchpkg.ModeLogInput{
			Mode:          run.Mode,
			RunStartedAt:  run.RunStartedAt,
			RunFinishedAt: run.RunFinishedAt,
			LogPath:       run.LogPath,
		})
	}
	return result, nil
}

func computeBenchmarkMetrics(result benchmarkHarnessResult) (benchmarkMetricsResult, error) {
	summaries, err := benchmarkInternalAggregateModeMetricsFn(result.ModeLogs)
	if err != nil {
		return benchmarkMetricsResult{}, err
	}
	metrics := benchmarkMetricsResult{
		ModeScores:    make([]benchmarkModeScore, 0, len(summaries)),
		ModeSummaries: summaries,
	}
	for idx, mode := range summaries {
		metrics.ModeScores = append(metrics.ModeScores, benchmarkModeScore{
			Mode:  mode.Mode,
			Score: idx + 1,
		})
	}
	return metrics, nil
}

func parseCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	raw := strings.Split(value, ",")
	out := make([]string, 0, len(raw))
	for _, token := range raw {
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
