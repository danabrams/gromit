package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	benchpkg "github.com/danabrams/gromit/internal/benchmark"
)

func TestBenchmarkRunCommand_DispatchesPipeline(t *testing.T) {
	called := false
	orig := benchmarkRunPipelineFn
	t.Cleanup(func() { benchmarkRunPipelineFn = orig })

	benchmarkRunPipelineFn = func(opts benchmarkRunOptions) error {
		called = true
		if opts.ManifestPath != "testdata/fixtures/benchmark/basic.yaml" {
			t.Fatalf("manifest path = %q, want %q", opts.ManifestPath, "testdata/fixtures/benchmark/basic.yaml")
		}
		if opts.OutputTimestamp != "20260223T120000Z" {
			t.Fatalf("output ts = %q, want %q", opts.OutputTimestamp, "20260223T120000Z")
		}
		return errors.New("sentinel")
	}

	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "run",
		"--manifest", "testdata/fixtures/benchmark/basic.yaml",
		"--output-ts", "20260223T120000Z",
	)
	if !called {
		t.Fatal("expected benchmarkRunPipelineFn to be called")
	}
	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "sentinel") {
		t.Fatalf("stderr = %q, want to contain %q", stderr, "sentinel")
	}
}

func TestRunBenchmarkPipeline_ExecutesStagesInOrder(t *testing.T) {
	origLoad := benchmarkLoadManifestFn
	origSelect := benchmarkSelectCohortFn
	origValidate := benchmarkValidateCohortFn
	origHarness := benchmarkRunHarnessFn
	origMetrics := benchmarkComputeMetricsFn
	origReport := benchmarkWriteReportFn
	t.Cleanup(func() {
		benchmarkLoadManifestFn = origLoad
		benchmarkSelectCohortFn = origSelect
		benchmarkValidateCohortFn = origValidate
		benchmarkRunHarnessFn = origHarness
		benchmarkComputeMetricsFn = origMetrics
		benchmarkWriteReportFn = origReport
	})

	opts := benchmarkRunOptions{ManifestPath: "manifest.yaml", BaseCommit: "abc123"}
	order := make([]string, 0, 6)

	benchmarkLoadManifestFn = func(path string) (benchmarkManifest, error) {
		order = append(order, "manifest")
		if path != opts.ManifestPath {
			return benchmarkManifest{}, fmt.Errorf("path = %q, want %q", path, opts.ManifestPath)
		}
		return benchmarkManifest{ID: "bm-1"}, nil
	}
	benchmarkSelectCohortFn = func(manifest benchmarkManifest, opts benchmarkRunOptions) (benchmarkSelection, error) {
		order = append(order, "selection")
		return benchmarkSelection{SelectedBeads: []string{"gromit-1", "gromit-2"}}, nil
	}
	benchmarkValidateCohortFn = func(selection benchmarkSelection, opts benchmarkRunOptions) (benchmarkValidatedCohort, error) {
		order = append(order, "validation")
		return benchmarkValidatedCohort{SelectedBeads: selection.SelectedBeads}, nil
	}
	benchmarkRunHarnessFn = func(manifest benchmarkManifest, cohort benchmarkValidatedCohort, opts benchmarkRunOptions) (benchmarkHarnessResult, error) {
		order = append(order, "harness")
		if opts.BaseCommit != "abc123" {
			return benchmarkHarnessResult{}, fmt.Errorf("base_commit = %q, want %q", opts.BaseCommit, "abc123")
		}
		return benchmarkHarnessResult{
			BaseCommit:    "abc123",
			SelectedBeads: cohort.SelectedBeads,
		}, nil
	}
	benchmarkComputeMetricsFn = func(result benchmarkHarnessResult) (benchmarkMetricsResult, error) {
		order = append(order, "metrics")
		if result.BaseCommit != "abc123" {
			return benchmarkMetricsResult{}, fmt.Errorf("metrics base_commit = %q, want %q", result.BaseCommit, "abc123")
		}
		if strings.Join(result.SelectedBeads, ",") != "gromit-1,gromit-2" {
			return benchmarkMetricsResult{}, fmt.Errorf("metrics selected_beads = %v", result.SelectedBeads)
		}
		return benchmarkMetricsResult{}, nil
	}
	benchmarkWriteReportFn = func(manifest benchmarkManifest, result benchmarkHarnessResult, metrics benchmarkMetricsResult, opts benchmarkRunOptions) error {
		order = append(order, "report")
		if result.BaseCommit != "abc123" {
			return fmt.Errorf("report base_commit = %q, want %q", result.BaseCommit, "abc123")
		}
		if strings.Join(result.SelectedBeads, ",") != "gromit-1,gromit-2" {
			return fmt.Errorf("report selected_beads = %v", result.SelectedBeads)
		}
		return nil
	}

	if err := runBenchmarkPipeline(opts); err != nil {
		t.Fatalf("runBenchmarkPipeline() error = %v", err)
	}

	got := strings.Join(order, "->")
	want := "manifest->selection->validation->harness->metrics->report"
	if got != want {
		t.Fatalf("pipeline order = %q, want %q", got, want)
	}
}

func TestRunBenchmarkPipeline_WritesDeterministicArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	origValidate := benchmarkValidateCohortFn
	origHarness := benchmarkRunHarnessFn
	origMetrics := benchmarkComputeMetricsFn
	t.Cleanup(func() {
		benchmarkValidateCohortFn = origValidate
		benchmarkRunHarnessFn = origHarness
		benchmarkComputeMetricsFn = origMetrics
	})
	benchmarkValidateCohortFn = func(selection benchmarkSelection, opts benchmarkRunOptions) (benchmarkValidatedCohort, error) {
		return benchmarkValidatedCohort{SelectedBeads: selection.SelectedBeads}, nil
	}
	benchmarkRunHarnessFn = func(manifest benchmarkManifest, cohort benchmarkValidatedCohort, opts benchmarkRunOptions) (benchmarkHarnessResult, error) {
		baseCommit := manifest.BaseCommit
		if opts.BaseCommit != "" {
			baseCommit = opts.BaseCommit
		}
		modes := make([]benchmarkModeResult, 0, len(manifest.Modes))
		for _, mode := range manifest.Modes {
			modes = append(modes, benchmarkModeResult{
				Mode:          mode,
				BaseCommit:    baseCommit,
				SelectedBeads: append([]string(nil), cohort.SelectedBeads...),
			})
		}
		return benchmarkHarnessResult{
			BaseCommit:    baseCommit,
			SelectedBeads: append([]string(nil), cohort.SelectedBeads...),
			Modes:         modes,
		}, nil
	}
	benchmarkComputeMetricsFn = func(result benchmarkHarnessResult) (benchmarkMetricsResult, error) {
		summaries := make([]benchpkg.ModeSummary, 0, len(result.Modes))
		for _, mode := range result.Modes {
			summaries = append(summaries, benchpkg.ModeSummary{Mode: mode.Mode})
		}
		return benchmarkMetricsResult{ModeSummaries: summaries}, nil
	}

	opts := benchmarkRunOptions{
		ManifestPath:    filepath.Join("/home/dabrams/gromit", "cmd", "gromit", "testdata", "fixtures", "benchmark", "basic.yaml"),
		OutputTimestamp: "20260223T120000Z",
	}

	if err := runBenchmarkPipeline(opts); err != nil {
		t.Fatalf("first runBenchmarkPipeline() error = %v", err)
	}

	jsonPath := filepath.Join(".gromit", "benchmarks", "results", "tdd-vs-single-pass", "20260223T120000Z.json")
	mdPath := filepath.Join(".gromit", "benchmarks", "results", "tdd-vs-single-pass", "20260223T120000Z.md")
	jsonFirst, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read first json artifact: %v", err)
	}
	mdFirst, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read first markdown artifact: %v", err)
	}

	if err := runBenchmarkPipeline(opts); err != nil {
		t.Fatalf("second runBenchmarkPipeline() error = %v", err)
	}
	jsonSecond, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read second json artifact: %v", err)
	}
	mdSecond, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read second markdown artifact: %v", err)
	}

	if string(jsonFirst) != string(jsonSecond) {
		t.Fatalf("json artifact changed across repeated runs\nfirst:\n%s\nsecond:\n%s", string(jsonFirst), string(jsonSecond))
	}
	if string(mdFirst) != string(mdSecond) {
		t.Fatalf("markdown artifact changed across repeated runs\nfirst:\n%s\nsecond:\n%s", string(mdFirst), string(mdSecond))
	}

	var payload struct {
		Manifest struct {
			Beads []string `json:"beads"`
		} `json:"manifest"`
		Modes []struct {
			Mode string `json:"mode"`
		} `json:"modes"`
	}
	if err := json.Unmarshal(jsonFirst, &payload); err != nil {
		t.Fatalf("unmarshal json artifact: %v", err)
	}

	if strings.Join(payload.Manifest.Beads, ",") != "gromit-1,gromit-2,gromit-3,gromit-4,gromit-5" {
		t.Fatalf("manifest.beads = %v", payload.Manifest.Beads)
	}
	if len(payload.Modes) != 3 {
		t.Fatalf("mode count = %d, want 3", len(payload.Modes))
	}
}

func TestRunBenchmarkPipeline_ReportArtifactsMatchInternalWriter(t *testing.T) {
	tmpDir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origWD) })
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir tmp dir: %v", err)
	}

	origLoad := benchmarkInternalLoadManifestFn
	origSelect := benchmarkSelectCohortFn
	origValidate := benchmarkValidateCohortFn
	origHarness := benchmarkRunHarnessFn
	origAggregate := benchmarkInternalAggregateModeMetricsFn
	origWrite := benchmarkInternalWriteReportFn
	origNow := benchmarkNowFn
	t.Cleanup(func() {
		benchmarkInternalLoadManifestFn = origLoad
		benchmarkSelectCohortFn = origSelect
		benchmarkValidateCohortFn = origValidate
		benchmarkRunHarnessFn = origHarness
		benchmarkInternalAggregateModeMetricsFn = origAggregate
		benchmarkInternalWriteReportFn = origWrite
		benchmarkNowFn = origNow
	})

	benchmarkNowFn = func() time.Time {
		return time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)
	}

	manifest := benchpkg.Manifest{
		ID:         "tdd-vs-single-pass",
		BaseCommit: "abc123",
		Beads:      []string{"gromit-1", "gromit-2"},
		ModeConfig: benchpkg.ModeConfig{Modes: []string{"single_pass", "tdd_shared_context"}},
		ModelPinning: benchpkg.ModelPinning{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
		},
	}

	benchmarkSelectCohortFn = func(manifest benchmarkManifest, opts benchmarkRunOptions) (benchmarkSelection, error) {
		return benchmarkSelection{SelectedBeads: []string{"gromit-1", "gromit-2", "gromit-3", "gromit-4", "gromit-5"}}, nil
	}

	benchmarkInternalLoadManifestFn = func(path string) (benchpkg.Manifest, error) {
		return manifest, nil
	}

	benchmarkValidateCohortFn = func(selection benchmarkSelection, opts benchmarkRunOptions) (benchmarkValidatedCohort, error) {
		return benchmarkValidatedCohort{SelectedBeads: selection.SelectedBeads}, nil
	}

	benchmarkRunHarnessFn = func(manifest benchmarkManifest, cohort benchmarkValidatedCohort, opts benchmarkRunOptions) (benchmarkHarnessResult, error) {
		return benchmarkHarnessResult{
			BaseCommit:    manifest.BaseCommit,
			SelectedBeads: append([]string(nil), cohort.SelectedBeads...),
			Modes: []benchmarkModeResult{
				{Mode: "single_pass", BaseCommit: manifest.BaseCommit, SelectedBeads: append([]string(nil), cohort.SelectedBeads...)},
				{Mode: "tdd_shared_context", BaseCommit: manifest.BaseCommit, SelectedBeads: append([]string(nil), cohort.SelectedBeads...)},
			},
		}, nil
	}

	summary := benchpkg.ModeSummary{
		Mode:           "single_pass",
		ElapsedSeconds: 60,
		TotalInput:     1200,
		TotalOutput:    600,
		TotalCostUSD:   1.5,
		TierTotals:     benchpkg.TierTotals{Low: benchpkg.TierTotalsRow{InputTokens: 800, OutputTokens: 400, CostUSD: 0.8}},
		Quality:        benchpkg.QualityMetrics{AverageScore: 0.9, FirstPassRate: 0.75, ReviewFindings: 1, ReviewFixesApplied: 0, FinalValidationPassed: true},
		CostQualityRatio: 1.67,
	}
	benchmarkInternalAggregateModeMetricsFn = func(inputs []benchpkg.ModeLogInput) ([]benchpkg.ModeSummary, error) {
		return []benchpkg.ModeSummary{summary}, nil
	}

	var capturedInput benchpkg.ReportInput
	benchmarkInternalWriteReportFn = func(input benchpkg.ReportInput) (benchpkg.ReportPaths, error) {
		capturedInput = input
		return benchpkg.WriteReport(input)
	}

	opts := benchmarkRunOptions{
		ManifestPath: "unused-manifest.yaml",
		Beads:        []string{"gromit-1", "gromit-2"},
	}

	if err := runBenchmarkPipeline(opts); err != nil {
		t.Fatalf("runBenchmarkPipeline() error = %v", err)
	}

	if capturedInput.Manifest.ID == "" || capturedInput.Timestamp == "" {
		t.Fatal("captured report input missing manifest or timestamp")
	}

	actualJSONPath := filepath.Join(tmpDir, ".gromit", "benchmarks", "results", capturedInput.Manifest.ID, capturedInput.Timestamp+".json")
	actualMDPath := filepath.Join(tmpDir, ".gromit", "benchmarks", "results", capturedInput.Manifest.ID, capturedInput.Timestamp+".md")
	actualJSON, err := os.ReadFile(actualJSONPath)
	if err != nil {
		t.Fatalf("read actual json artifact: %v", err)
	}
	actualMD, err := os.ReadFile(actualMDPath)
	if err != nil {
		t.Fatalf("read actual markdown artifact: %v", err)
	}

	expectedJSON, expectedMD := renderInternalBenchmarkReport(t, capturedInput)
	if !bytes.Equal(actualJSON, expectedJSON) {
		t.Fatalf("json artifact diverges from internal writer\nactual:\n%s\nexpected:\n%s", string(actualJSON), string(expectedJSON))
	}
	if !bytes.Equal(actualMD, expectedMD) {
		t.Fatalf("markdown artifact diverges from internal writer\nactual:\n%s\nexpected:\n%s", string(actualMD), string(expectedMD))
	}
}

func TestBenchmarkRunCommand_ReportInputMatchesCanonicalBuilder(t *testing.T) {
	tmpDir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origWD) })
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir tmp dir: %v", err)
	}

	origLoad := benchmarkInternalLoadManifestFn
	origSelect := benchmarkSelectCohortFn
	origValidate := benchmarkValidateCohortFn
	origHarness := benchmarkRunHarnessFn
	origMetrics := benchmarkComputeMetricsFn
	origWrite := benchmarkInternalWriteReportFn
	origNow := benchmarkNowFn
	t.Cleanup(func() {
		benchmarkInternalLoadManifestFn = origLoad
		benchmarkSelectCohortFn = origSelect
		benchmarkValidateCohortFn = origValidate
		benchmarkRunHarnessFn = origHarness
		benchmarkComputeMetricsFn = origMetrics
		benchmarkInternalWriteReportFn = origWrite
		benchmarkNowFn = origNow
	})

	manifest := benchpkg.Manifest{
		ID:         "cli-benchmark",
		BaseCommit: "abc123",
		Beads:      []string{"gromit-1", "gromit-2"},
		ModeConfig: benchpkg.ModeConfig{
			Modes: []string{"single_pass"},
		},
		ModelPinning: benchpkg.ModelPinning{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
		},
	}
	selectedBeads := append([]string(nil), manifest.Beads...)

	benchmarkNowFn = func() time.Time {
		return time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	}

	benchmarkInternalLoadManifestFn = func(path string) (benchpkg.Manifest, error) {
		return manifest, nil
	}
	benchmarkSelectCohortFn = func(manifest benchmarkManifest, opts benchmarkRunOptions) (benchmarkSelection, error) {
		return benchmarkSelection{SelectedBeads: append([]string(nil), selectedBeads...)}, nil
	}
	benchmarkValidateCohortFn = func(selection benchmarkSelection, opts benchmarkRunOptions) (benchmarkValidatedCohort, error) {
		return benchmarkValidatedCohort{SelectedBeads: selection.SelectedBeads}, nil
	}
	harnessResult := benchmarkHarnessResult{
		BaseCommit:    manifest.BaseCommit,
		SelectedBeads: append([]string(nil), selectedBeads...),
		Modes: []benchmarkModeResult{
			{
				Mode:          "single_pass",
				BaseCommit:    manifest.BaseCommit,
				SelectedBeads: append([]string(nil), selectedBeads...),
			},
		},
	}
	benchmarkRunHarnessFn = func(manifest benchmarkManifest, cohort benchmarkValidatedCohort, opts benchmarkRunOptions) (benchmarkHarnessResult, error) {
		return harnessResult, nil
	}
	metrics := benchmarkMetricsResult{
		ModeSummaries: []benchpkg.ModeSummary{
			{
				Mode:           "single_pass",
				ElapsedSeconds: 90,
				TotalInput:     1500,
				TotalOutput:    700,
				TotalCostUSD:   1.75,
				Quality:        benchpkg.QualityMetrics{AverageScore: 0.95, FirstPassRate: 0.8},
			},
		},
		ModeScores: []benchmarkModeScore{{Mode: "single_pass", Score: 1}},
	}
	benchmarkComputeMetricsFn = func(result benchmarkHarnessResult) (benchmarkMetricsResult, error) {
		return metrics, nil
	}

	var capturedInput benchpkg.ReportInput
	benchmarkInternalWriteReportFn = func(input benchpkg.ReportInput) (benchpkg.ReportPaths, error) {
		capturedInput = input
		return benchpkg.WriteReport(input)
	}

	manifestPath := filepath.Join(tmpDir, "manifest.yaml")
	if err := os.WriteFile(manifestPath, []byte("test manifest"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	opts := benchmarkRunOptions{
		ManifestPath:    manifestPath,
		OutputTimestamp: "20260225T010000Z",
	}

	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "run",
		"--manifest", manifestPath,
		"--output-ts", opts.OutputTimestamp,
	)
	if exitCode != 0 {
		t.Fatalf("benchmark run exitCode = %d, stderr = %q", exitCode, stderr)
	}

	loadedManifest, err := benchmarkLoadManifestFn(opts.ManifestPath)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	expected := buildBenchmarkReportInput(loadedManifest, harnessResult, metrics, opts)
	if !reflect.DeepEqual(expected, capturedInput) {
		t.Fatalf("report input mismatch\nexpected: %+v\nactual: %+v", expected, capturedInput)
	}
}

func renderInternalBenchmarkReport(t *testing.T, input benchpkg.ReportInput) ([]byte, []byte) {
	t.Helper()

	tmpDir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir expected dir: %v", err)
	}
	defer func() {
		if err := os.Chdir(origWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	if _, err := benchpkg.WriteReport(input); err != nil {
		t.Fatalf("render internal report: %v", err)
	}

	jsonPath := filepath.Join(tmpDir, ".gromit", "benchmarks", "results", input.Manifest.ID, input.Timestamp+".json")
	mdPath := filepath.Join(tmpDir, ".gromit", "benchmarks", "results", input.Manifest.ID, input.Timestamp+".md")

	jsonBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read internal json: %v", err)
	}
	mdBytes, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read internal markdown: %v", err)
	}
	return jsonBytes, mdBytes
}

func TestWriteBenchmarkReport_PreservesInternalReportMarkdownSections(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	err := writeBenchmarkReport(
		benchmarkManifest{
			ID:              "tdd-vs-single-pass",
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
		},
		benchmarkHarnessResult{
			BaseCommit:    "abc123",
			SelectedBeads: []string{"gromit-1", "gromit-2", "gromit-3", "gromit-4", "gromit-5"},
		},
		benchmarkMetricsResult{
			ModeSummaries: []benchpkg.ModeSummary{
				{
					Mode:             "single_pass",
					ElapsedSeconds:   120,
					TotalInput:       1000,
					TotalOutput:      500,
					TotalCostUSD:     1.25,
					CostQualityRatio: 1.42,
					Quality:          benchpkg.QualityMetrics{AverageScore: 0.88, FirstPassRate: 0.67, ReviewFindings: 3, ReviewFixesApplied: 2, FinalValidationPassed: true},
				},
			},
		},
		benchmarkRunOptions{OutputTimestamp: "20260223T120000Z"},
	)
	if err != nil {
		t.Fatalf("writeBenchmarkReport() error = %v", err)
	}

	mdPath := filepath.Join(".gromit", "benchmarks", "results", "tdd-vs-single-pass", "20260223T120000Z.md")
	content, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read report markdown: %v", err)
	}

	for _, section := range []string{
		"## Per-Mode Summary",
		"## By-Tier Totals",
		"## By-Model Totals",
		"## Quality Metrics",
		"## Winner Hints",
	} {
		if !strings.Contains(string(content), section) {
			t.Fatalf("markdown missing %q section\n%s", section, string(content))
		}
	}
}

func TestRunBenchmarkPipeline_ReportJSONUsesInternalManifestMetadataShape(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	manifestPath := filepath.Join(t.TempDir(), "manifest.yaml")
	manifest := `id: tdd-vs-single-pass
base_commit: abc123
beads:
  - gromit-1
  - gromit-2
  - gromit-3
  - gromit-4
  - gromit-5
modes:
  - single_pass
provider: openai
model_family: gpt-5
low_tier_model: gpt-5.1-codex-mini
medium_tier_model: gpt-5.3-codex
high_tier_model: gpt-5.3-codex
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	origValidate := benchmarkValidateCohortFn
	origHarness := benchmarkRunHarnessFn
	origMetrics := benchmarkComputeMetricsFn
	t.Cleanup(func() {
		benchmarkValidateCohortFn = origValidate
		benchmarkRunHarnessFn = origHarness
		benchmarkComputeMetricsFn = origMetrics
	})

	benchmarkValidateCohortFn = func(selection benchmarkSelection, opts benchmarkRunOptions) (benchmarkValidatedCohort, error) {
		return benchmarkValidatedCohort{SelectedBeads: selection.SelectedBeads}, nil
	}
	benchmarkRunHarnessFn = func(manifest benchmarkManifest, cohort benchmarkValidatedCohort, opts benchmarkRunOptions) (benchmarkHarnessResult, error) {
		return benchmarkHarnessResult{
			BaseCommit:    "abc123",
			SelectedBeads: append([]string(nil), cohort.SelectedBeads...),
			Modes: []benchmarkModeResult{
				{Mode: "single_pass", BaseCommit: "abc123", SelectedBeads: append([]string(nil), cohort.SelectedBeads...)},
			},
		}, nil
	}
	benchmarkComputeMetricsFn = func(result benchmarkHarnessResult) (benchmarkMetricsResult, error) {
		return benchmarkMetricsResult{}, nil
	}

	opts := benchmarkRunOptions{
		ManifestPath:    manifestPath,
		OutputTimestamp: "20260223T140000Z",
	}
	if err := runBenchmarkPipeline(opts); err != nil {
		t.Fatalf("runBenchmarkPipeline() error = %v", err)
	}

	jsonPath := filepath.Join(".gromit", "benchmarks", "results", "tdd-vs-single-pass", "20260223T140000Z.json")
	reportBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read report json: %v", err)
	}

	var payload struct {
		Manifest struct {
			ID          string `json:"id"`
			Provider    string `json:"provider"`
			ModelFamily string `json:"model_family"`
		} `json:"manifest"`
	}
	if err := json.Unmarshal(reportBytes, &payload); err != nil {
		t.Fatalf("unmarshal report json: %v", err)
	}

	if payload.Manifest.ID != "tdd-vs-single-pass" {
		t.Fatalf("manifest.id = %q, want %q", payload.Manifest.ID, "tdd-vs-single-pass")
	}
	if payload.Manifest.Provider != "openai" {
		t.Fatalf("manifest.provider = %q, want %q", payload.Manifest.Provider, "openai")
	}
	if payload.Manifest.ModelFamily != "gpt-5" {
		t.Fatalf("manifest.model_family = %q, want %q", payload.Manifest.ModelFamily, "gpt-5")
	}
}

func TestRunBenchmarkPipeline_RejectsManifestMissingProviderPinning(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	manifestPath := filepath.Join(t.TempDir(), "manifest.yaml")
	manifest := `id: tdd-vs-single-pass
base_commit: abc123
beads:
  - gromit-1
  - gromit-2
  - gromit-3
  - gromit-4
  - gromit-5
modes:
  - single_pass
`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	err := runBenchmarkPipeline(benchmarkRunOptions{ManifestPath: manifestPath, OutputTimestamp: "20260223T140500Z"})
	if err == nil {
		t.Fatal("runBenchmarkPipeline() error = nil, want provider/model pinning validation error")
	}
	if !strings.Contains(err.Error(), "provider is required") {
		t.Fatalf("runBenchmarkPipeline() error = %q, want contains %q", err.Error(), "provider is required")
	}
}

func TestRunBenchmarkPipeline_UsesInternalBenchmarkStagesInOrder(t *testing.T) {
	origLoad := benchmarkInternalLoadManifestFn
	origResolve := benchmarkInternalResolveSelectedBeadsFn
	origValidate := benchmarkInternalValidateSelectedCohortFn
	origRunModes := benchmarkInternalRunModesInIsolatedWorktreesFn
	origAggregate := benchmarkInternalAggregateModeMetricsFn
	origWrite := benchmarkInternalWriteReportFn
	origNewLookup := benchmarkNewBeadLookupFn
	origNewResolver := benchmarkNewBaseCommitResolverFn
	origNewRunner := benchmarkNewModeWorktreeRunnerFn
	origNow := benchmarkNowFn
	t.Cleanup(func() {
		benchmarkInternalLoadManifestFn = origLoad
		benchmarkInternalResolveSelectedBeadsFn = origResolve
		benchmarkInternalValidateSelectedCohortFn = origValidate
		benchmarkInternalRunModesInIsolatedWorktreesFn = origRunModes
		benchmarkInternalAggregateModeMetricsFn = origAggregate
		benchmarkInternalWriteReportFn = origWrite
		benchmarkNewBeadLookupFn = origNewLookup
		benchmarkNewBaseCommitResolverFn = origNewResolver
		benchmarkNewModeWorktreeRunnerFn = origNewRunner
		benchmarkNowFn = origNow
	})

	order := make([]string, 0, 6)
	manifest := benchpkg.Manifest{
		ID:         "bench-1",
		BaseCommit: "abc123",
		Beads:      []string{"gromit-1", "gromit-2", "gromit-3", "gromit-4", "gromit-5"},
		ModeConfig: benchpkg.ModeConfig{Modes: []string{"single_pass"}},
		ModelPinning: benchpkg.ModelPinning{
			Provider:        "openai",
			ModelFamily:     "gpt-5",
			LowTierModel:    "gpt-5.1-codex-mini",
			MediumTierModel: "gpt-5.3-codex",
			HighTierModel:   "gpt-5.3-codex",
		},
	}

	benchmarkInternalLoadManifestFn = func(path string) (benchpkg.Manifest, error) {
		order = append(order, "load")
		return manifest, nil
	}
	benchmarkInternalResolveSelectedBeadsFn = func(manifestBeads, cliBeads []string, beadCount int) ([]string, error) {
		order = append(order, "resolve")
		return append([]string(nil), manifestBeads...), nil
	}
	benchmarkInternalValidateSelectedCohortFn = func(lookup benchpkg.BeadLookup, selected []string, minSize int, requireTierCoverage bool) ([]string, error) {
		order = append(order, "validate")
		return append([]string(nil), selected...), nil
	}
	benchmarkInternalRunModesInIsolatedWorktreesFn = func(ctx context.Context, input benchpkg.RunModesInput) ([]benchpkg.ModeWorktreeRun, string, error) {
		order = append(order, "run_modes")
		if strings.Join(input.Modes, ",") != "single_pass" {
			t.Fatalf("run modes input = %v, want [single_pass]", input.Modes)
		}
		return []benchpkg.ModeWorktreeRun{{Mode: "single_pass"}}, "abc123", nil
	}
	benchmarkInternalAggregateModeMetricsFn = func(inputs []benchpkg.ModeLogInput) ([]benchpkg.ModeSummary, error) {
		order = append(order, "aggregate")
		return []benchpkg.ModeSummary{{Mode: "single_pass"}}, nil
	}
	benchmarkInternalWriteReportFn = func(input benchpkg.ReportInput) (benchpkg.ReportPaths, error) {
		order = append(order, "write")
		return benchpkg.ReportPaths{}, nil
	}
	benchmarkNewBeadLookupFn = func() (benchpkg.BeadLookup, error) { return nil, nil }
	benchmarkNewBaseCommitResolverFn = func() benchpkg.BaseCommitResolver { return nil }
	benchmarkNewModeWorktreeRunnerFn = func() benchpkg.ModeWorktreeRunner { return nil }
	benchmarkNowFn = func() time.Time { return time.Date(2026, 2, 23, 15, 0, 0, 0, time.UTC) }

	if err := runBenchmarkPipeline(benchmarkRunOptions{ManifestPath: "manifest.yaml"}); err != nil {
		t.Fatalf("runBenchmarkPipeline() error = %v", err)
	}

	got := strings.Join(order, "->")
	want := "load->resolve->validate->run_modes->aggregate->write"
	if got != want {
		t.Fatalf("stage order = %q, want %q", got, want)
	}
}

func TestBenchmarkRunCommand_BeadOverridesDriveSelection(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	origValidate := benchmarkValidateCohortFn
	origHarness := benchmarkRunHarnessFn
	origMetrics := benchmarkComputeMetricsFn
	t.Cleanup(func() {
		benchmarkValidateCohortFn = origValidate
		benchmarkRunHarnessFn = origHarness
		benchmarkComputeMetricsFn = origMetrics
	})
	benchmarkValidateCohortFn = func(selection benchmarkSelection, opts benchmarkRunOptions) (benchmarkValidatedCohort, error) {
		return benchmarkValidatedCohort{SelectedBeads: selection.SelectedBeads}, nil
	}
	benchmarkRunHarnessFn = func(manifest benchmarkManifest, cohort benchmarkValidatedCohort, opts benchmarkRunOptions) (benchmarkHarnessResult, error) {
		baseCommit := manifest.BaseCommit
		if opts.BaseCommit != "" {
			baseCommit = opts.BaseCommit
		}
		return benchmarkHarnessResult{
			BaseCommit:    baseCommit,
			SelectedBeads: append([]string(nil), cohort.SelectedBeads...),
			Modes: []benchmarkModeResult{
				{Mode: "single_pass", BaseCommit: baseCommit, SelectedBeads: append([]string(nil), cohort.SelectedBeads...)},
			},
		}, nil
	}
	benchmarkComputeMetricsFn = func(result benchmarkHarnessResult) (benchmarkMetricsResult, error) {
		return benchmarkMetricsResult{}, nil
	}

	manifestPath := filepath.Join("/home/dabrams/gromit", "cmd", "gromit", "testdata", "fixtures", "benchmark", "basic.yaml")
	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "run",
		"--manifest", manifestPath,
		"--beads", "gromit-9,gromit-8,gromit-7,gromit-6,gromit-10",
		"--bead-count", "5",
		"--output-ts", "20260223T130000Z",
	)
	if exitCode != 0 {
		t.Fatalf("benchmark run exitCode = %d, stderr = %q", exitCode, stderr)
	}

	jsonPath := filepath.Join(".gromit", "benchmarks", "results", "tdd-vs-single-pass", "20260223T130000Z.json")
	reportBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read report json: %v", err)
	}

	var payload struct {
		Manifest struct {
			Beads []string `json:"beads"`
		} `json:"manifest"`
	}
	if err := json.Unmarshal(reportBytes, &payload); err != nil {
		t.Fatalf("unmarshal report json: %v", err)
	}

	if strings.Join(payload.Manifest.Beads, ",") != "gromit-9,gromit-8,gromit-7,gromit-6,gromit-10" {
		t.Fatalf("manifest.beads = %v, want [%s]", payload.Manifest.Beads, "gromit-9 gromit-8 gromit-7 gromit-6 gromit-10")
	}
}

func TestBenchmarkRunCommand_RejectsInvalidOutputTimestamp(t *testing.T) {
	manifestPath := filepath.Join("/home/dabrams/gromit", "cmd", "gromit", "testdata", "fixtures", "benchmark", "basic.yaml")

	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "run",
		"--manifest", manifestPath,
		"--output-ts", "not-a-ts",
	)

	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "--output-ts must be in UTC format YYYYMMDDTHHMMSSZ") {
		t.Fatalf("stderr = %q, want output-ts validation error", stderr)
	}
}

func TestBenchmarkRunCommand_RejectsNegativeBeadCount(t *testing.T) {
	manifestPath := filepath.Join("/home/dabrams/gromit", "cmd", "gromit", "testdata", "fixtures", "benchmark", "basic.yaml")

	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "run",
		"--manifest", manifestPath,
		"--bead-count", "-1",
	)

	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "--bead-count must be zero or greater") {
		t.Fatalf("stderr = %q, want bead-count validation error", stderr)
	}
}

func TestBenchmarkRunCommand_RejectsEmptyBeadsOverride(t *testing.T) {
	manifestPath := filepath.Join("/home/dabrams/gromit", "cmd", "gromit", "testdata", "fixtures", "benchmark", "basic.yaml")

	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "run",
		"--manifest", manifestPath,
		"--beads", ", ,",
	)

	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "--beads must include at least one bead id when provided") {
		t.Fatalf("stderr = %q, want beads validation error", stderr)
	}
}

func TestBenchmarkRunCommand_RejectsInvalidBaseCommit(t *testing.T) {
	manifestPath := filepath.Join("/home/dabrams/gromit", "cmd", "gromit", "testdata", "fixtures", "benchmark", "basic.yaml")

	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "run",
		"--manifest", manifestPath,
		"--base-commit", " ",
	)

	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "--base-commit must be a non-empty commit reference") {
		t.Fatalf("stderr = %q, want base-commit validation error", stderr)
	}
}

func TestBenchmarkRunCommand_RejectsBlankManifestPath(t *testing.T) {
	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "run",
		"--manifest", " ",
	)

	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "--manifest must be a non-empty path") {
		t.Fatalf("stderr = %q, want manifest validation error", stderr)
	}
}

func TestBenchmarkRunCommand_RejectsBaseCommitWithWhitespace(t *testing.T) {
	manifestPath := filepath.Join("/home/dabrams/gromit", "cmd", "gromit", "testdata", "fixtures", "benchmark", "basic.yaml")

	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "run",
		"--manifest", manifestPath,
		"--base-commit", "abc def",
	)

	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "--base-commit must not contain whitespace") {
		t.Fatalf("stderr = %q, want base-commit whitespace validation error", stderr)
	}
}

func TestValidateBenchmarkCohort_UsesRequiredSizeFive(t *testing.T) {
	origNewLookup := benchmarkNewBeadLookupFn
	origValidate := benchmarkInternalValidateSelectedCohortFn
	t.Cleanup(func() {
		benchmarkNewBeadLookupFn = origNewLookup
		benchmarkInternalValidateSelectedCohortFn = origValidate
	})

	benchmarkNewBeadLookupFn = func() (benchpkg.BeadLookup, error) { return nil, nil }
	benchmarkInternalValidateSelectedCohortFn = func(lookup benchpkg.BeadLookup, selected []string, minSize int, requireTierCoverage bool) ([]string, error) {
		if minSize != 5 {
			t.Fatalf("minSize = %d, want 5", minSize)
		}
		if !requireTierCoverage {
			t.Fatal("requireTierCoverage = false, want true")
		}
		return append([]string(nil), selected...), nil
	}

	_, err := validateBenchmarkCohort(benchmarkSelection{SelectedBeads: []string{"gromit-1", "gromit-2", "gromit-3", "gromit-4", "gromit-5"}}, benchmarkRunOptions{})
	if err != nil {
		t.Fatalf("validateBenchmarkCohort() error = %v", err)
	}
}

func TestValidateBenchmarkCohort_UsesSingleBeadPilotConstraints(t *testing.T) {
	origNewLookup := benchmarkNewBeadLookupFn
	origValidate := benchmarkInternalValidateSelectedCohortFn
	t.Cleanup(func() {
		benchmarkNewBeadLookupFn = origNewLookup
		benchmarkInternalValidateSelectedCohortFn = origValidate
	})

	benchmarkNewBeadLookupFn = func() (benchpkg.BeadLookup, error) { return nil, nil }
	benchmarkInternalValidateSelectedCohortFn = func(lookup benchpkg.BeadLookup, selected []string, minSize int, requireTierCoverage bool) ([]string, error) {
		if minSize != 1 {
			t.Fatalf("minSize = %d, want 1", minSize)
		}
		if requireTierCoverage {
			t.Fatal("requireTierCoverage = true, want false")
		}
		return append([]string(nil), selected...), nil
	}

	_, err := validateBenchmarkCohort(
		benchmarkSelection{SelectedBeads: []string{"gromit-1"}},
		benchmarkRunOptions{SingleBead: "gromit-1"},
	)
	if err != nil {
		t.Fatalf("validateBenchmarkCohort() error = %v", err)
	}
}

func TestBenchmarkRunCommand_SingleBeadPilotSelection(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	origValidate := benchmarkValidateCohortFn
	origHarness := benchmarkRunHarnessFn
	origMetrics := benchmarkComputeMetricsFn
	t.Cleanup(func() {
		benchmarkValidateCohortFn = origValidate
		benchmarkRunHarnessFn = origHarness
		benchmarkComputeMetricsFn = origMetrics
	})
	benchmarkValidateCohortFn = func(selection benchmarkSelection, opts benchmarkRunOptions) (benchmarkValidatedCohort, error) {
		return benchmarkValidatedCohort{SelectedBeads: selection.SelectedBeads}, nil
	}
	benchmarkRunHarnessFn = func(manifest benchmarkManifest, cohort benchmarkValidatedCohort, opts benchmarkRunOptions) (benchmarkHarnessResult, error) {
		return benchmarkHarnessResult{
			BaseCommit:    "abc123",
			SelectedBeads: append([]string(nil), cohort.SelectedBeads...),
			Modes: []benchmarkModeResult{
				{Mode: "single_pass", BaseCommit: "abc123", SelectedBeads: append([]string(nil), cohort.SelectedBeads...)},
				{Mode: "tdd_shared_context", BaseCommit: "abc123", SelectedBeads: append([]string(nil), cohort.SelectedBeads...)},
				{Mode: "tdd_fresh_context", BaseCommit: "abc123", SelectedBeads: append([]string(nil), cohort.SelectedBeads...)},
			},
		}, nil
	}
	benchmarkComputeMetricsFn = func(result benchmarkHarnessResult) (benchmarkMetricsResult, error) {
		return benchmarkMetricsResult{ModeSummaries: []benchpkg.ModeSummary{{Mode: "single_pass"}}}, nil
	}

	manifestPath := filepath.Join("/home/dabrams/gromit", "cmd", "gromit", "testdata", "fixtures", "benchmark", "basic.yaml")
	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "run",
		"--manifest", manifestPath,
		"--single-bead", "gromit-9",
		"--output-ts", "20260224T150000Z",
	)
	if exitCode != 0 {
		t.Fatalf("benchmark run exitCode = %d, stderr = %q", exitCode, stderr)
	}

	jsonPath := filepath.Join(".gromit", "benchmarks", "results", "tdd-vs-single-pass", "20260224T150000Z.json")
	reportBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read report json: %v", err)
	}

	var payload struct {
		Manifest struct {
			Beads []string `json:"beads"`
		} `json:"manifest"`
	}
	if err := json.Unmarshal(reportBytes, &payload); err != nil {
		t.Fatalf("unmarshal report json: %v", err)
	}
	if strings.Join(payload.Manifest.Beads, ",") != "gromit-9" {
		t.Fatalf("manifest.beads = %v, want [gromit-9]", payload.Manifest.Beads)
	}
}

func TestBenchmarkRunCommand_RejectsSingleBeadWithBeadsOverride(t *testing.T) {
	manifestPath := filepath.Join("/home/dabrams/gromit", "cmd", "gromit", "testdata", "fixtures", "benchmark", "basic.yaml")

	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "run",
		"--manifest", manifestPath,
		"--single-bead", "gromit-1",
		"--beads", "gromit-1,gromit-2,gromit-3,gromit-4,gromit-5",
	)
	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "--single-bead cannot be combined with --beads") {
		t.Fatalf("stderr = %q, want single-bead/beads validation error", stderr)
	}
}

func TestBenchmarkRunCommand_RejectsSingleBeadWithBeadCount(t *testing.T) {
	manifestPath := filepath.Join("/home/dabrams/gromit", "cmd", "gromit", "testdata", "fixtures", "benchmark", "basic.yaml")

	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "run",
		"--manifest", manifestPath,
		"--single-bead", "gromit-1",
		"--bead-count", "1",
	)
	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "--single-bead cannot be combined with --bead-count") {
		t.Fatalf("stderr = %q, want single-bead/bead-count validation error", stderr)
	}
}

func TestBenchmarkPhase3ReportCommand_DispatchesWriter(t *testing.T) {
	called := false
	orig := benchmarkWritePhase3MeasurementReportFn
	t.Cleanup(func() { benchmarkWritePhase3MeasurementReportFn = orig })

	benchmarkWritePhase3MeasurementReportFn = func(input benchpkg.Phase3MeasurementInput) (benchpkg.Phase3ReportPaths, error) {
		called = true
		if input.BaselineLogPath != "baseline.jsonl" {
			t.Fatalf("baseline log = %q, want %q", input.BaselineLogPath, "baseline.jsonl")
		}
		if input.OptimizedLogPath != "optimized.jsonl" {
			t.Fatalf("optimized log = %q, want %q", input.OptimizedLogPath, "optimized.jsonl")
		}
		if input.Timestamp != "20260224T124500Z" {
			t.Fatalf("timestamp = %q, want %q", input.Timestamp, "20260224T124500Z")
		}
		return benchpkg.Phase3ReportPaths{}, errors.New("sentinel")
	}

	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "phase3-report",
		"--baseline-log", "baseline.jsonl",
		"--optimized-log", "optimized.jsonl",
		"--output-ts", "20260224T124500Z",
	)
	if !called {
		t.Fatal("expected benchmarkWritePhase3MeasurementReportFn to be called")
	}
	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "sentinel") {
		t.Fatalf("stderr = %q, want to contain %q", stderr, "sentinel")
	}
}
