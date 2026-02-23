package main

import (
	"errors"
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
