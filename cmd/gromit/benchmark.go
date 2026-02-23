package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type benchmarkRunOptions struct {
	ManifestPath    string
	OutputTimestamp string
	BaseCommit      string
	Beads           []string
	BeadCount       int
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
var benchmarkBeads string
var benchmarkBeadCount int

type benchmarkManifest struct {
	ID         string   `yaml:"id"`
	BaseCommit string   `yaml:"base_commit"`
	Beads      []string `yaml:"beads"`
	Modes      []string `yaml:"modes"`
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
}

type benchmarkModeResult struct {
	Mode          string   `json:"mode"`
	BaseCommit    string   `json:"base_commit"`
	SelectedBeads []string `json:"selected_beads"`
}

type benchmarkMetricsResult struct {
	ModeScores []benchmarkModeScore `json:"mode_scores"`
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
		beads := parseCSV(benchmarkBeads)
		return benchmarkRunPipelineFn(benchmarkRunOptions{
			ManifestPath:    benchmarkManifestPath,
			OutputTimestamp: benchmarkOutputTS,
			BaseCommit:      benchmarkBaseCommit,
			Beads:           beads,
			BeadCount:       benchmarkBeadCount,
		})
	},
}

func init() {
	benchmarkRunCmd.Flags().StringVar(&benchmarkManifestPath, "manifest", "", "Path to benchmark manifest")
	benchmarkRunCmd.Flags().StringVar(&benchmarkOutputTS, "output-ts", "", "Timestamp override for deterministic artifact names")
	benchmarkRunCmd.Flags().StringVar(&benchmarkBaseCommit, "base-commit", "", "Base commit override for benchmark runs")
	benchmarkRunCmd.Flags().StringVar(&benchmarkBeads, "beads", "", "Comma-separated ordered bead IDs to benchmark")
	benchmarkRunCmd.Flags().IntVar(&benchmarkBeadCount, "bead-count", 0, "Optional deterministic truncation count for selected beads")
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
	content, err := os.ReadFile(path)
	if err != nil {
		return benchmarkManifest{}, fmt.Errorf("read manifest: %w", err)
	}

	var manifest benchmarkManifest
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return benchmarkManifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if manifest.ID == "" {
		return benchmarkManifest{}, fmt.Errorf("manifest id is required")
	}
	if len(manifest.Beads) == 0 {
		return benchmarkManifest{}, fmt.Errorf("manifest beads is required")
	}
	if len(manifest.Modes) == 0 {
		return benchmarkManifest{}, fmt.Errorf("manifest modes is required")
	}

	return manifest, nil
}

func selectBenchmarkCohort(manifest benchmarkManifest, opts benchmarkRunOptions) (benchmarkSelection, error) {
	selected := manifest.Beads
	if len(opts.Beads) > 0 {
		selected = opts.Beads
	}
	if opts.BeadCount > 0 {
		if opts.BeadCount > len(selected) {
			return benchmarkSelection{}, fmt.Errorf("--bead-count %d exceeds selected cohort size %d", opts.BeadCount, len(selected))
		}
		selected = selected[:opts.BeadCount]
	}

	return benchmarkSelection{SelectedBeads: append([]string(nil), selected...)}, nil
}

func validateBenchmarkCohort(selection benchmarkSelection) (benchmarkValidatedCohort, error) {
	if len(selection.SelectedBeads) == 0 {
		return benchmarkValidatedCohort{}, fmt.Errorf("selected cohort cannot be empty")
	}
	return benchmarkValidatedCohort{SelectedBeads: selection.SelectedBeads}, nil
}

func runBenchmarkHarness(manifest benchmarkManifest, cohort benchmarkValidatedCohort, opts benchmarkRunOptions) (benchmarkHarnessResult, error) {
	baseCommit := manifest.BaseCommit
	if opts.BaseCommit != "" {
		baseCommit = opts.BaseCommit
	}

	result := benchmarkHarnessResult{
		BaseCommit:    baseCommit,
		SelectedBeads: append([]string(nil), cohort.SelectedBeads...),
		Modes:         make([]benchmarkModeResult, 0, len(manifest.Modes)),
	}
	for _, mode := range manifest.Modes {
		result.Modes = append(result.Modes, benchmarkModeResult{
			Mode:          mode,
			BaseCommit:    baseCommit,
			SelectedBeads: append([]string(nil), cohort.SelectedBeads...),
		})
	}

	return result, nil
}

func computeBenchmarkMetrics(result benchmarkHarnessResult) (benchmarkMetricsResult, error) {
	metrics := benchmarkMetricsResult{ModeScores: make([]benchmarkModeScore, 0, len(result.Modes))}
	for idx, mode := range result.Modes {
		metrics.ModeScores = append(metrics.ModeScores, benchmarkModeScore{
			Mode:  mode.Mode,
			Score: idx + 1,
		})
	}
	return metrics, nil
}

func writeBenchmarkReport(manifest benchmarkManifest, result benchmarkHarnessResult, metrics benchmarkMetricsResult, opts benchmarkRunOptions) error {
	ts := opts.OutputTimestamp
	if ts == "" {
		ts = time.Now().UTC().Format("20060102T150405Z")
	}

	resultDir := filepath.Join(".gromit", "benchmarks", "results", manifest.ID)
	if err := os.MkdirAll(resultDir, 0755); err != nil {
		return fmt.Errorf("create results directory: %w", err)
	}

	payload := struct {
		ManifestID    string                 `json:"manifest_id"`
		SelectedBeads []string               `json:"selected_beads"`
		Modes         []benchmarkModeResult  `json:"modes"`
		Metrics       benchmarkMetricsResult `json:"metrics"`
	}{
		ManifestID:    manifest.ID,
		SelectedBeads: append([]string(nil), result.SelectedBeads...),
		Modes:         result.Modes,
		Metrics:       metrics,
	}

	jsonBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report json: %w", err)
	}
	jsonBytes = append(jsonBytes, '\n')
	if err := os.WriteFile(filepath.Join(resultDir, ts+".json"), jsonBytes, 0644); err != nil {
		return fmt.Errorf("write report json: %w", err)
	}

	var md strings.Builder
	md.WriteString("# Benchmark Report\n\n")
	md.WriteString("Manifest: " + manifest.ID + "\n")
	md.WriteString("Base commit: " + result.BaseCommit + "\n")
	md.WriteString("Selected beads: " + strings.Join(result.SelectedBeads, ", ") + "\n\n")
	md.WriteString("## Modes\n")
	for _, mode := range result.Modes {
		md.WriteString("- " + mode.Mode + " (" + mode.BaseCommit + ")\n")
	}
	md.WriteString("\n## Metrics\n")
	for _, score := range metrics.ModeScores {
		md.WriteString("- " + score.Mode + ": " + fmt.Sprintf("%d", score.Score) + "\n")
	}

	if err := os.WriteFile(filepath.Join(resultDir, ts+".md"), []byte(md.String()), 0644); err != nil {
		return fmt.Errorf("write report markdown: %w", err)
	}

	return nil
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
