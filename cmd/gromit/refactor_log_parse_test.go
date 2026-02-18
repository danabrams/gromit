package main

import (
	"os"
	"strings"
	"testing"
)

func TestParseTestLog_ZeroFailuresWhenAllPass(t *testing.T) {
	input := `=== RUN   TestBar
--- PASS: TestBar (0.00s)
PASS
ok  	github.com/danabrams/gromit/internal/bar	(cached)
`
	result := parseTestLog(strings.NewReader(input))

	if len(result.Failures) != 0 {
		t.Errorf("want 0 failures, got %d: %v", len(result.Failures), result.Failures)
	}
}

func TestParseTestLog_DetectsSkippedTest(t *testing.T) {
	input := `=== RUN   TestBaz
    baz_test.go:10: skipping
--- SKIP: TestBaz (0.00s)
PASS
ok  	github.com/danabrams/gromit/internal/baz	(cached)
`
	result := parseTestLog(strings.NewReader(input))

	if len(result.SkippedTests) != 1 {
		t.Fatalf("want 1 skipped test, got %d", len(result.SkippedTests))
	}
	if result.SkippedTests[0] != "TestBaz" {
		t.Errorf("skipped = %q, want %q", result.SkippedTests[0], "TestBaz")
	}
}

func TestParseTestLog_DetectsBuildError(t *testing.T) {
	input := `# github.com/danabrams/gromit/internal/broken [github.com/danabrams/gromit/internal/broken.test]
FAIL	github.com/danabrams/gromit/internal/broken [build failed]
`
	result := parseTestLog(strings.NewReader(input))

	if len(result.BuildErrors) != 1 {
		t.Fatalf("want 1 build error, got %d", len(result.BuildErrors))
	}
	if result.BuildErrors[0] != "github.com/danabrams/gromit/internal/broken" {
		t.Errorf("build error pkg = %q, want %q", result.BuildErrors[0], "github.com/danabrams/gromit/internal/broken")
	}
}

func TestFormatTestLogSummary_ZeroFailures(t *testing.T) {
	result := TestLogResult{
		Failures:     []TestFailure{},
		BuildErrors:  []string{},
		SkippedTests: []string{},
	}
	summary := formatTestLogSummary("test-logs/foo.log", result)

	if !strings.Contains(summary, "ZERO FAILURES") {
		t.Errorf("summary missing ZERO FAILURES marker:\n%s", summary)
	}
}

func TestRefactorBaselineSummary_FileExistsAndIsClean(t *testing.T) {
	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}
	summaryPath := root + "/test-logs/refactor-baseline-failures.txt"

	data, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("summary file %q not found: %v", summaryPath, err)
	}

	content := string(data)
	if !strings.Contains(content, "ZERO FAILURES") {
		t.Errorf("summary missing ZERO FAILURES marker:\n%s", content)
	}
}

func TestParseTestLog_DetectsFailingTest(t *testing.T) {
	input := `=== RUN   TestFoo
--- FAIL: TestFoo (0.01s)
    foo_test.go:12: got 1, want 2
FAIL
FAIL	github.com/danabrams/gromit/internal/foo	0.01s
`
	result := parseTestLog(strings.NewReader(input))

	if len(result.Failures) != 1 {
		t.Fatalf("want 1 failure, got %d", len(result.Failures))
	}
	got := result.Failures[0]
	if got.Package != "github.com/danabrams/gromit/internal/foo" {
		t.Errorf("package = %q, want %q", got.Package, "github.com/danabrams/gromit/internal/foo")
	}
	if got.Test != "TestFoo" {
		t.Errorf("test = %q, want %q", got.Test, "TestFoo")
	}
}
