package main

import (
	"errors"
	"strings"
	"testing"
)

func TestBenchmarkDecomposeCompare_CommandExists(t *testing.T) {
	// Test that the command wiring exists
	if benchmarkDecomposeCompareCmd == nil {
		t.Fatal("benchmarkDecomposeCompareCmd must be defined")
	}
}

func TestBenchmarkDecomposeCompare_RunEndToEnd(t *testing.T) {
	// Test end-to-end flow with fake dependencies
	called := false
	origFn := benchmarkRunDecomposeCompareFn
	t.Cleanup(func() { benchmarkRunDecomposeCompareFn = origFn })

	benchmarkRunDecomposeCompareFn = func(opts benchmarkDecomposeCompareOptions) error {
		called = true
		if opts.ManifestPath != "testdata/fixtures/benchmark/decompose.yaml" {
			t.Fatalf("manifest path = %q, want %q", opts.ManifestPath, "testdata/fixtures/benchmark/decompose.yaml")
		}
		return errors.New("sentinel")
	}

	_, _, exitCode := runGromitCobra(t,
		"benchmark", "decompose-compare",
		"--manifest", "testdata/fixtures/benchmark/decompose.yaml",
	)
	if !called {
		t.Fatal("expected benchmarkRunDecomposeCompareFn to be called")
	}
	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
}

func TestBenchmarkDecomposeCompare_WiresCohortSelector(t *testing.T) {
	// Test that cohort selection is called during end-to-end flow
	cohortSelectorCalled := false
	origCohort := benchmarkDecomposeCompareCohortSelectorFn
	t.Cleanup(func() { benchmarkDecomposeCompareCohortSelectorFn = origCohort })

	benchmarkDecomposeCompareCohortSelectorFn = func(opts benchmarkDecomposeCompareCohortSelectorOptions) ([]string, error) {
		cohortSelectorCalled = true
		return []string{"spec1", "spec2", "spec3", "spec4", "spec5"}, nil
	}

	origRunner := benchmarkDecomposeCompareRunnerFn
	t.Cleanup(func() { benchmarkDecomposeCompareRunnerFn = origRunner })
	benchmarkDecomposeCompareRunnerFn = func(opts benchmarkDecomposeCompareRunnerOptions) (interface{}, error) {
		return nil, nil
	}

	origWriter := benchmarkDecomposeCompareReportWriterFn
	t.Cleanup(func() { benchmarkDecomposeCompareReportWriterFn = origWriter })
	benchmarkDecomposeCompareReportWriterFn = func(opts benchmarkDecomposeCompareReportWriterOptions) error {
		return nil
	}

	_, _, exitCode := runGromitCobra(t,
		"benchmark", "decompose-compare",
		"--manifest", "testdata/fixtures/benchmark/decompose.yaml",
	)
	if !cohortSelectorCalled {
		t.Fatal("expected cohort selector to be called")
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestBenchmarkDecomposeCompare_WiresCompareRunner(t *testing.T) {
	// Test that compare runner is called during end-to-end flow
	compareRunnerCalled := false
	origRunner := benchmarkDecomposeCompareRunnerFn
	t.Cleanup(func() { benchmarkDecomposeCompareRunnerFn = origRunner })

	benchmarkDecomposeCompareRunnerFn = func(opts benchmarkDecomposeCompareRunnerOptions) (interface{}, error) {
		compareRunnerCalled = true
		return nil, nil
	}

	origCohort := benchmarkDecomposeCompareCohortSelectorFn
	t.Cleanup(func() { benchmarkDecomposeCompareCohortSelectorFn = origCohort })

	benchmarkDecomposeCompareCohortSelectorFn = func(opts benchmarkDecomposeCompareCohortSelectorOptions) ([]string, error) {
		return []string{"spec1", "spec2", "spec3", "spec4", "spec5"}, nil
	}

	origWriter := benchmarkDecomposeCompareReportWriterFn
	t.Cleanup(func() { benchmarkDecomposeCompareReportWriterFn = origWriter })
	benchmarkDecomposeCompareReportWriterFn = func(opts benchmarkDecomposeCompareReportWriterOptions) error {
		return nil
	}

	_, _, exitCode := runGromitCobra(t,
		"benchmark", "decompose-compare",
		"--manifest", "testdata/fixtures/benchmark/decompose.yaml",
	)
	if !compareRunnerCalled {
		t.Fatal("expected compare runner to be called")
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestBenchmarkDecomposeCompare_WiresReportWriter(t *testing.T) {
	// Test that report writer is called during end-to-end flow
	reportWriterCalled := false
	origWriter := benchmarkDecomposeCompareReportWriterFn
	t.Cleanup(func() { benchmarkDecomposeCompareReportWriterFn = origWriter })

	benchmarkDecomposeCompareReportWriterFn = func(opts benchmarkDecomposeCompareReportWriterOptions) error {
		reportWriterCalled = true
		return nil
	}

	origRunner := benchmarkDecomposeCompareRunnerFn
	t.Cleanup(func() { benchmarkDecomposeCompareRunnerFn = origRunner })

	benchmarkDecomposeCompareRunnerFn = func(opts benchmarkDecomposeCompareRunnerOptions) (interface{}, error) {
		return nil, nil
	}

	origCohort := benchmarkDecomposeCompareCohortSelectorFn
	t.Cleanup(func() { benchmarkDecomposeCompareCohortSelectorFn = origCohort })

	benchmarkDecomposeCompareCohortSelectorFn = func(opts benchmarkDecomposeCompareCohortSelectorOptions) ([]string, error) {
		return []string{"spec1", "spec2", "spec3", "spec4", "spec5"}, nil
	}

	_, _, exitCode := runGromitCobra(t,
		"benchmark", "decompose-compare",
		"--manifest", "testdata/fixtures/benchmark/decompose.yaml",
	)
	if !reportWriterCalled {
		t.Fatal("expected report writer to be called")
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestBenchmarkDecomposeCompare_PrintsArtifactPathsOnStdout(t *testing.T) {
	// RED: Test that artifact paths are printed on stdout
	origWriter := benchmarkDecomposeCompareReportWriterFn
	t.Cleanup(func() { benchmarkDecomposeCompareReportWriterFn = origWriter })

	benchmarkDecomposeCompareReportWriterFn = func(opts benchmarkDecomposeCompareReportWriterOptions) error {
		return nil
	}

	origRunner := benchmarkDecomposeCompareRunnerFn
	t.Cleanup(func() { benchmarkDecomposeCompareRunnerFn = origRunner })

	benchmarkDecomposeCompareRunnerFn = func(opts benchmarkDecomposeCompareRunnerOptions) (interface{}, error) {
		return nil, nil
	}

	origCohort := benchmarkDecomposeCompareCohortSelectorFn
	t.Cleanup(func() { benchmarkDecomposeCompareCohortSelectorFn = origCohort })

	benchmarkDecomposeCompareCohortSelectorFn = func(opts benchmarkDecomposeCompareCohortSelectorOptions) ([]string, error) {
		return []string{"spec1", "spec2", "spec3", "spec4", "spec5"}, nil
	}

	stdout, _, exitCode := runGromitCobra(t,
		"benchmark", "decompose-compare",
		"--manifest", "testdata/fixtures/benchmark/decompose.yaml",
	)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if !strings.Contains(stdout, ".gromit/benchmarks/results") {
		t.Fatalf("stdout = %q, want to contain benchmark results path", stdout)
	}
}

func TestBenchmarkDecomposeCompare_FailsWithInsufficientCohort(t *testing.T) {
	// RED: Test error handling when cohort selector returns too few specs
	origCohort := benchmarkDecomposeCompareCohortSelectorFn
	t.Cleanup(func() { benchmarkDecomposeCompareCohortSelectorFn = origCohort })

	benchmarkDecomposeCompareCohortSelectorFn = func(opts benchmarkDecomposeCompareCohortSelectorOptions) ([]string, error) {
		return []string{"spec1", "spec2", "spec3"}, nil // Only 3 instead of 5
	}

	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "decompose-compare",
		"--manifest", "testdata/fixtures/benchmark/decompose.yaml",
	)
	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "insufficient") && !strings.Contains(stderr, "exactly 5") {
		t.Fatalf("stderr = %q, want to contain error about cohort size", stderr)
	}
}
