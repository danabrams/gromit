package main

import (
	"errors"
	"reflect"
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
	// Test error handling when cohort selector returns too few specs
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

func TestBenchmarkDecomposeCompare_FailsWithWriteError(t *testing.T) {
	// RED: Test error handling when report writer fails
	origWriter := benchmarkDecomposeCompareReportWriterFn
	t.Cleanup(func() { benchmarkDecomposeCompareReportWriterFn = origWriter })

	benchmarkDecomposeCompareReportWriterFn = func(opts benchmarkDecomposeCompareReportWriterOptions) error {
		return errors.New("write permission denied")
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

	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "decompose-compare",
		"--manifest", "testdata/fixtures/benchmark/decompose.yaml",
	)
	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "write report") && !strings.Contains(stderr, "write permission") {
		t.Fatalf("stderr = %q, want to contain write error", stderr)
	}
}

func TestBenchmarkDecomposeCompare_FailsWithCohortSelectorError(t *testing.T) {
	// Test error handling when cohort selector fails
	origCohort := benchmarkDecomposeCompareCohortSelectorFn
	t.Cleanup(func() { benchmarkDecomposeCompareCohortSelectorFn = origCohort })

	benchmarkDecomposeCompareCohortSelectorFn = func(opts benchmarkDecomposeCompareCohortSelectorOptions) ([]string, error) {
		return nil, errors.New("tracker unavailable")
	}

	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "decompose-compare",
		"--manifest", "testdata/fixtures/benchmark/decompose.yaml",
	)
	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "select cohort") && !strings.Contains(stderr, "tracker unavailable") {
		t.Fatalf("stderr = %q, want to contain cohort selector error", stderr)
	}
}

func TestBenchmarkDecomposeCompare_ValidatesOutputTimestampFormat(t *testing.T) {
	// RED: Test that invalid timestamp is rejected
	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "decompose-compare",
		"--manifest", "testdata/fixtures/benchmark/decompose.yaml",
		"--output-ts", "invalid-timestamp",
	)
	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "must be in UTC format") {
		t.Fatalf("stderr = %q, want to contain timestamp format error", stderr)
	}
}

func TestBenchmarkDecomposeCompare_AcceptsValidOutputTimestamp(t *testing.T) {
	// Test that valid timestamp is accepted and used
	origCohort := benchmarkDecomposeCompareCohortSelectorFn
	t.Cleanup(func() { benchmarkDecomposeCompareCohortSelectorFn = origCohort })

	benchmarkDecomposeCompareCohortSelectorFn = func(opts benchmarkDecomposeCompareCohortSelectorOptions) ([]string, error) {
		return []string{"spec1", "spec2", "spec3", "spec4", "spec5"}, nil
	}

	origRunner := benchmarkDecomposeCompareRunnerFn
	t.Cleanup(func() { benchmarkDecomposeCompareRunnerFn = origRunner })

	benchmarkDecomposeCompareRunnerFn = func(opts benchmarkDecomposeCompareRunnerOptions) (interface{}, error) {
		return nil, nil
	}

	origWriter := benchmarkDecomposeCompareReportWriterFn
	t.Cleanup(func() { benchmarkDecomposeCompareReportWriterFn = origWriter })

	writerCalled := false
	writtenTS := ""
	benchmarkDecomposeCompareReportWriterFn = func(opts benchmarkDecomposeCompareReportWriterOptions) error {
		writerCalled = true
		writtenTS = opts.OutputTS
		return nil
	}

	stdout, _, exitCode := runGromitCobra(t,
		"benchmark", "decompose-compare",
		"--manifest", "testdata/fixtures/benchmark/decompose.yaml",
		"--output-ts", "20260225T123456Z",
	)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if !writerCalled {
		t.Fatal("report writer must be called")
	}
	if writtenTS != "20260225T123456Z" {
		t.Fatalf("writer called with ts = %q, want %q", writtenTS, "20260225T123456Z")
	}
	if !strings.Contains(stdout, "20260225T123456Z") {
		t.Fatalf("stdout = %q, want to contain provided timestamp", stdout)
	}
}

func TestBenchmarkDecomposeCompare_FailsWithInvalidSpecOverrides(t *testing.T) {
	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "decompose-compare",
		"--manifest", "testdata/fixtures/benchmark/decompose.yaml",
		"--specs", "spec1,spec2",
	)
	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "--specs must include exactly 5 spec ids") {
		t.Fatalf("stderr = %q, want spec override error", stderr)
	}
}

func TestBenchmarkDecomposeCompare_FailsWithInvalidThreshold(t *testing.T) {
	_, stderr, exitCode := runGromitCobra(t,
		"benchmark", "decompose-compare",
		"--manifest", "testdata/fixtures/benchmark/decompose.yaml",
		"--threshold", "-0.1",
	)
	if exitCode == 0 {
		t.Fatalf("exitCode = %d, want non-zero", exitCode)
	}
	if !strings.Contains(stderr, "--threshold must be between 0 and 1") {
		t.Fatalf("stderr = %q, want threshold error", stderr)
	}
}

func TestBenchmarkDecomposeCompare_PassesOverridesToRunner(t *testing.T) {
	specs := []string{"spec-A", "spec-B", "spec-C", "spec-D", "spec-E"}
	var runnerOpts benchmarkDecomposeCompareRunnerOptions
	runnerCalled := false

	origSelector := benchmarkDecomposeCompareCohortSelectorFn
	t.Cleanup(func() { benchmarkDecomposeCompareCohortSelectorFn = origSelector })
	benchmarkDecomposeCompareCohortSelectorFn = func(opts benchmarkDecomposeCompareCohortSelectorOptions) ([]string, error) {
		if !reflect.DeepEqual(opts.SpecOverrides, specs) {
			t.Fatalf("SpecOverrides = %v, want %v", opts.SpecOverrides, specs)
		}
		return append([]string(nil), specs...), nil
	}

	origRunner := benchmarkDecomposeCompareRunnerFn
	t.Cleanup(func() { benchmarkDecomposeCompareRunnerFn = origRunner })
	benchmarkDecomposeCompareRunnerFn = func(opts benchmarkDecomposeCompareRunnerOptions) (interface{}, error) {
		runnerCalled = true
		runnerOpts = opts
		return nil, nil
	}

	origWriter := benchmarkDecomposeCompareReportWriterFn
	t.Cleanup(func() { benchmarkDecomposeCompareReportWriterFn = origWriter })
	benchmarkDecomposeCompareReportWriterFn = func(opts benchmarkDecomposeCompareReportWriterOptions) error {
		return nil
	}

	stdout, _, exitCode := runGromitCobra(t,
		"benchmark", "decompose-compare",
		"--manifest", "testdata/fixtures/benchmark/decompose.yaml",
		"--specs", strings.Join(specs, ","),
		"--threshold", "0.42",
	)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if !runnerCalled {
		t.Fatal("runner was not called")
	}
	if !reflect.DeepEqual(runnerOpts.Specs, specs) {
		t.Fatalf("runner specs = %v, want %v", runnerOpts.Specs, specs)
	}
	if !runnerOpts.FailureThresholdSet {
		t.Fatal("runner should see threshold flag")
	}
	if runnerOpts.FailureThreshold != 0.42 {
		t.Fatalf("runner threshold = %f, want 0.42", runnerOpts.FailureThreshold)
	}
	if !strings.Contains(stdout, ".gromit/benchmarks/results/decompose-haiku-vs-sonnet/") {
		t.Fatalf("stdout = %q, want results mention", stdout)
	}
}
