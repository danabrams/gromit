package main

import (
	"errors"
	"fmt"
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
