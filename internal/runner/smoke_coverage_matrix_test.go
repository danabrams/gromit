package runner

import (
	"bufio"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type runnerSmokeMatrixEntry struct {
	Decision    string
	Rationale   string
	Destination string
}

var (
	runnerSmokeAnnotationRe = regexp.MustCompile(`^//\s*smoke-matrix:\s*(keep|move)\s*\|\s*rationale:\s*(.+?)\s*\|\s*destination:\s*(.+)\s*$`)
	runnerSmokeFuncRe       = regexp.MustCompile(`^func (Test[[:alnum:]_]+)\(`)
)

func parseRunnerSmokeMatrixForFile(t *testing.T, projectRoot, rel string) (map[string]runnerSmokeMatrixEntry, []string) {
	t.Helper()

	path := filepath.Join(projectRoot, rel)
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", rel, err)
	}
	t.Cleanup(func() {
		if closeErr := f.Close(); closeErr != nil {
			t.Fatalf("close %s: %v", rel, closeErr)
		}
	})

	entries := make(map[string]runnerSmokeMatrixEntry)
	tests := make([]string, 0)
	var pending *runnerSmokeMatrixEntry

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if m := runnerSmokeAnnotationRe.FindStringSubmatch(line); m != nil {
			pending = &runnerSmokeMatrixEntry{
				Decision:    strings.TrimSpace(m[1]),
				Rationale:   strings.TrimSpace(m[2]),
				Destination: strings.TrimSpace(m[3]),
			}
			continue
		}

		if m := runnerSmokeFuncRe.FindStringSubmatch(line); m != nil {
			testName := strings.TrimSpace(m[1])
			tests = append(tests, testName)
			if pending != nil {
				entries[testName] = *pending
			}
			pending = nil
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", rel, err)
	}

	return entries, tests
}

func listRunnerUnitTests(t *testing.T, projectRoot string) string {
	t.Helper()

	cmd := exec.Command("go", "test", "./internal/runner/...", "-list", "Test")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test -list ./internal/runner failed: %v\n%s", err, string(out))
	}

	return string(out)
}

func listRunnerAcceptanceFiles(t *testing.T, projectRoot string) []string {
	t.Helper()

	root := filepath.Join(projectRoot, "internal", "runner")
	files := make([]string, 0)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "_acceptance_test.go") {
			return nil
		}
		rel, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk runner acceptance files: %v", err)
	}

	sort.Strings(files)
	return files
}

func TestRunnerSmokeCoverageMatrix_CaseDecisionsIncludeRationale(t *testing.T) {
	projectRoot := runnerSmokeSuiteRepoRoot(t)

	files := listRunnerAcceptanceFiles(t, projectRoot)

	for _, rel := range files {
		entries, tests := parseRunnerSmokeMatrixForFile(t, projectRoot, rel)
		for _, testName := range tests {
			entry, ok := entries[testName]
			if !ok {
				t.Fatalf("%s missing smoke-matrix mapping for %s", rel, testName)
			}
			if entry.Decision != "keep" && entry.Decision != "move" {
				t.Fatalf("%s has invalid decision %q for %s", rel, entry.Decision, testName)
			}
			if entry.Rationale == "" || entry.Rationale == "-" {
				t.Fatalf("%s has empty rationale for %s", rel, testName)
			}
		}
	}
}

func TestRunnerSmokeCoverageMatrix_KeepSetIsHighValueE2E(t *testing.T) {
	projectRoot := runnerSmokeSuiteRepoRoot(t)

	files := listRunnerAcceptanceFiles(t, projectRoot)

	expectedKeep := map[string]bool{
		"TestRunnerSmoke_RunSingleBeadHappyPath":         true,
		"TestRunnerSmoke_ValidationFailureEscalatesTier": true,
		"TestRunnerSmoke_WorktreeMergeModesEndToEnd":     true,
	}

	actualKeep := make(map[string]bool)
	for _, rel := range files {
		entries, tests := parseRunnerSmokeMatrixForFile(t, projectRoot, rel)
		for _, testName := range tests {
			entry, ok := entries[testName]
			if !ok {
				t.Fatalf("%s missing smoke-matrix mapping for %s", rel, testName)
			}
			if entry.Decision == "keep" {
				actualKeep[testName] = true
			}
		}
	}

	if len(actualKeep) != len(expectedKeep) {
		t.Fatalf("runner keep set size=%d, want=%d (actual=%v)", len(actualKeep), len(expectedKeep), actualKeep)
	}
	for testName := range expectedKeep {
		if !actualKeep[testName] {
			t.Fatalf("runner keep set missing high-value E2E case %s", testName)
		}
	}
}

func TestRunnerSmokeCoverageMatrix_MovedCasesPointToConcreteUnitSuites(t *testing.T) {
	projectRoot := runnerSmokeSuiteRepoRoot(t)

	files := listRunnerAcceptanceFiles(t, projectRoot)
	unitTests := listRunnerUnitTests(t, projectRoot)

	for _, rel := range files {
		entries, tests := parseRunnerSmokeMatrixForFile(t, projectRoot, rel)
		for _, testName := range tests {
			entry, ok := entries[testName]
			if !ok {
				t.Fatalf("%s missing smoke-matrix mapping for %s", rel, testName)
			}
			if entry.Decision != "move" {
				continue
			}

			parts := strings.Split(entry.Destination, ":")
			if len(parts) != 2 {
				t.Fatalf("%s destination %q must be file:suite", testName, entry.Destination)
			}
			if !strings.HasSuffix(parts[0], "_test.go") {
				t.Fatalf("%s destination file must be *_test.go, got %q", testName, parts[0])
			}
			if !strings.Contains(unitTests, parts[1]) {
				t.Fatalf("runner unit suite missing moved destination test %s", parts[1])
			}
		}
	}
}
