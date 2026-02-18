package main

import (
	"bufio"
	"io"
	"strings"
)

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
	failingTests := map[string]bool{}
	var currentFailPkg string

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()

		// "--- FAIL: TestFoo (0.01s)" — record the test name
		if strings.HasPrefix(line, "--- FAIL: ") {
			rest := strings.TrimPrefix(line, "--- FAIL: ")
			// rest = "TestFoo (0.01s)"
			name := strings.Fields(rest)[0]
			failingTests[name] = true
		}

		// "--- SKIP: TestBar (0.00s)" — record skipped test
		if strings.HasPrefix(line, "--- SKIP: ") {
			rest := strings.TrimPrefix(line, "--- SKIP: ")
			name := strings.Fields(rest)[0]
			result.SkippedTests = append(result.SkippedTests, name)
		}

		// "FAIL\tgithub.com/..." — package-level failure line
		if strings.HasPrefix(line, "FAIL\t") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				currentFailPkg = parts[1]
			}
		}

		// "[build failed]" in package result line
		if strings.Contains(line, "[build failed]") {
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
