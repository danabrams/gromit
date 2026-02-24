package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	benchmarkValidateCohortFn = func(selection benchmarkSelection) (benchmarkValidatedCohort, error) {
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
		SelectedBeads []string `json:"selected_beads"`
		Modes         []struct {
			Mode          string   `json:"mode"`
			BaseCommit    string   `json:"base_commit"`
			SelectedBeads []string `json:"selected_beads"`
		} `json:"modes"`
	}
	if err := json.Unmarshal(jsonFirst, &payload); err != nil {
		t.Fatalf("unmarshal json artifact: %v", err)
	}

	if strings.Join(payload.SelectedBeads, ",") != "gromit-1,gromit-2,gromit-3" {
		t.Fatalf("selected_beads = %v", payload.SelectedBeads)
	}
	if len(payload.Modes) != 3 {
		t.Fatalf("mode count = %d, want 3", len(payload.Modes))
	}
	for _, mode := range payload.Modes {
		if mode.BaseCommit != "abc123" {
			t.Fatalf("mode %s base_commit = %q, want %q", mode.Mode, mode.BaseCommit, "abc123")
		}
		if strings.Join(mode.SelectedBeads, ",") != "gromit-1,gromit-2,gromit-3" {
			t.Fatalf("mode %s selected_beads = %v", mode.Mode, mode.SelectedBeads)
		}
	}
}

func TestBenchmarkRunCommand_BeadOverridesDriveSelection(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	manifestPath := filepath.Join("/home/dabrams/gromit", "cmd", "gromit", "testdata", "fixtures", "benchmark", "basic.yaml")
	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "run",
		"--manifest", manifestPath,
		"--beads", "gromit-9,gromit-8,gromit-7",
		"--bead-count", "2",
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
		SelectedBeads []string `json:"selected_beads"`
	}
	if err := json.Unmarshal(reportBytes, &payload); err != nil {
		t.Fatalf("unmarshal report json: %v", err)
	}

	if strings.Join(payload.SelectedBeads, ",") != "gromit-9,gromit-8" {
		t.Fatalf("selected_beads = %v, want [%s]", payload.SelectedBeads, "gromit-9 gromit-8")
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
