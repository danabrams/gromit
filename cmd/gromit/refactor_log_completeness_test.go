package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const refactorBaselineLog = "test-logs/refactor-baseline-2026-02-18-044945.log"

func refactorBaselineLogPath(t *testing.T) string {
	t.Helper()
	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}
	return filepath.Join(root, refactorBaselineLog)
}

func readLogLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("could not open log file %q: %v", path, err)
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("reading log file %q: %v", path, err)
	}
	return lines
}

// TestRefactorLog_ContainsPerPackageOkLines verifies the log has per-package
// summary lines starting with "ok" (as emitted by go test for passing packages).
func TestRefactorLog_ContainsPerPackageOkLines(t *testing.T) {
	path := refactorBaselineLogPath(t)
	lines := readLogLines(t, path)

	var okLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "ok\t") {
			okLines = append(okLines, line)
		}
	}

	if len(okLines) == 0 {
		t.Errorf("log %q contains no per-package 'ok' result lines", refactorBaselineLog)
	}
}

// TestRefactorLog_ContainsStatusMarkers verifies the log contains PASS/FAIL/SKIP
// status indicators as emitted by go test.
func TestRefactorLog_ContainsStatusMarkers(t *testing.T) {
	path := refactorBaselineLogPath(t)
	lines := readLogLines(t, path)

	markers := map[string]bool{
		"PASS": false,
		"SKIP": false,
	}
	for _, line := range lines {
		for marker := range markers {
			if strings.Contains(line, marker) {
				markers[marker] = true
			}
		}
	}

	for marker, found := range markers {
		if !found {
			t.Errorf("log %q contains no %s status marker", refactorBaselineLog, marker)
		}
	}
}

// TestRefactorLog_ArtifactPathIsUnderTestLogs verifies the saved log file path
// is under the test-logs/ directory and the file exists.
func TestRefactorLog_ArtifactPathIsUnderTestLogs(t *testing.T) {
	const wantPrefix = "test-logs/"

	if !strings.HasPrefix(refactorBaselineLog, wantPrefix) {
		t.Fatalf("log artifact path %q does not start with %q", refactorBaselineLog, wantPrefix)
	}

	path := refactorBaselineLogPath(t)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("log artifact %q does not exist: %v", refactorBaselineLog, err)
	}
	if info.Size() == 0 {
		t.Errorf("log artifact %q is empty", refactorBaselineLog)
	}

	t.Logf("refactor baseline log artifact: %s", refactorBaselineLog)
}
