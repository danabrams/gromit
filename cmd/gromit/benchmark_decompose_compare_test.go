package main

import (
	"errors"
	"testing"
)

func TestBenchmarkDecomposeCompare_CommandExists(t *testing.T) {
	// RED: Test that the command wiring exists
	if benchmarkDecomposeCompareCmd == nil {
		t.Fatal("benchmarkDecomposeCompareCmd must be defined")
	}
}

func TestBenchmarkDecomposeCompare_RunEndToEnd(t *testing.T) {
	// RED: Test end-to-end flow with fake dependencies
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
