package main

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	failPrefix     = "--- FAIL: "
	skipPrefix     = "--- SKIP: "
	pkgFailPrefix  = "FAIL\t"
	buildFailToken = "[build failed]"
)

// testNameFromLine extracts the test name from a line with the given prefix.
func testNameFromLine(line, prefix string) string {
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(line, prefix)
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// TestFailure represents a single failing test with its package and test name.
type TestFailure struct {
	Package string
	Test    string
}

// TestLogResult holds the parsed results from a go test log.
type TestLogResult struct {
	Failures     []TestFailure
	BuildErrors  []string
	SkippedTests []string
}

// parseTestLog reads a go test output log and extracts failures, build errors,
// and skipped tests.
func parseTestLog(r io.Reader) TestLogResult {
	result := TestLogResult{
		Failures:     []TestFailure{},
		BuildErrors:  []string{},
		SkippedTests: []string{},
	}

	// Track current package from FAIL lines, and track failing test names from
	// "--- FAIL: TestName" lines.
	failingTests := map[string]struct{}{}
	var currentFailPkg string

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()

		// "--- FAIL: TestFoo (0.01s)" — record the test name
		if name := testNameFromLine(line, failPrefix); name != "" {
			failingTests[name] = struct{}{}
		}

		// "--- SKIP: TestBar (0.00s)" — record skipped test
		if name := testNameFromLine(line, skipPrefix); name != "" {
			result.SkippedTests = append(result.SkippedTests, name)
		}

		// "FAIL\tgithub.com/..." — package-level failure line
		if strings.HasPrefix(line, pkgFailPrefix) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				currentFailPkg = parts[1]
			}
		}

		// "[build failed]" in package result line
		if strings.Contains(line, buildFailToken) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				result.BuildErrors = append(result.BuildErrors, parts[1])
			}
		}
	}

	// Pair failing test names with the package they failed in.
	// We use currentFailPkg as the package for all recorded failing tests.
	for name := range failingTests {
		result.Failures = append(result.Failures, TestFailure{
			Package: currentFailPkg,
			Test:    name,
		})
	}

	return result
}

// formatTestLogSummary formats a human-readable summary of a parsed test log.
func formatTestLogSummary(logPath string, result TestLogResult) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Refactor Baseline Failure Summary\n")
	fmt.Fprintf(&sb, "Log: %s\n", logPath)
	fmt.Fprintf(&sb, "==================================\n\n")

	if len(result.BuildErrors) > 0 {
		fmt.Fprintf(&sb, "BUILD ERRORS (%d):\n", len(result.BuildErrors))
		sort.Strings(result.BuildErrors)
		for _, pkg := range result.BuildErrors {
			fmt.Fprintf(&sb, "  %s\n", pkg)
		}
		fmt.Fprintf(&sb, "\n")
	}

	if len(result.Failures) > 0 {
		fmt.Fprintf(&sb, "TEST FAILURES (%d):\n", len(result.Failures))
		sort.Slice(result.Failures, func(i, j int) bool {
			if result.Failures[i].Package != result.Failures[j].Package {
				return result.Failures[i].Package < result.Failures[j].Package
			}
			return result.Failures[i].Test < result.Failures[j].Test
		})
		for _, f := range result.Failures {
			fmt.Fprintf(&sb, "  %s.%s\n", f.Package, f.Test)
		}
		fmt.Fprintf(&sb, "\n")
	}

	if len(result.SkippedTests) > 0 {
		fmt.Fprintf(&sb, "SKIPPED TESTS (%d):\n", len(result.SkippedTests))
		sort.Strings(result.SkippedTests)
		for _, name := range result.SkippedTests {
			fmt.Fprintf(&sb, "  %s\n", name)
		}
		fmt.Fprintf(&sb, "\n")
	}

	if len(result.Failures) == 0 && len(result.BuildErrors) == 0 {
		fmt.Fprintf(&sb, "ZERO FAILURES — baseline is clean.\n")
	}

	return sb.String()
}
