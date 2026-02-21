//go:build acceptance

package acceptance_test

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
)

// smokeMatrixEntry holds the smoke-matrix annotation data for a test function.
type smokeMatrixEntry struct {
	Decision    string
	Rationale   string
	Destination string
}

var (
	smokeAnnotationRe = regexp.MustCompile(`^//\s*smoke-matrix:\s*(keep|move)\s*\|\s*rationale:\s*(.+?)\s*\|\s*destination:\s*(.+)\s*$`)
	smokeFuncRe       = regexp.MustCompile(`^func (Test[[:alnum:]_]+)\(`)
	testFuncNameRe    = regexp.MustCompile(`^func (Test[[:alnum:]_]+)\(`)

	testNameIndexMu    sync.Mutex
	testNameIndexCache = make(map[string]map[string]struct{})
)

// repoRoot returns the project root by navigating up from this file's location.
// This file lives at internal/runner/acceptance/, so root is three levels up.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
}

// listAcceptanceFiles returns all *_acceptance_test.go files under internal/runner/.
func listAcceptanceFiles(t *testing.T, projectRoot string) []string {
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

// listSmokeAcceptanceFiles returns only the smoke-suite acceptance files.
func listSmokeAcceptanceFiles(t *testing.T, projectRoot string) []string {
	t.Helper()

	smokeFiles := map[string]bool{
		"internal/runner/validation_extraction_acceptance_test.go":         true,
		"internal/runner/acceptance/invocation_timeout_acceptance_test.go": true,
		"internal/runner/acceptance/worktree_merge_acceptance_test.go":     true,
	}

	all := listAcceptanceFiles(t, projectRoot)
	files := make([]string, 0, len(smokeFiles))
	for _, rel := range all {
		if smokeFiles[filepath.ToSlash(rel)] {
			files = append(files, rel)
		}
	}
	sort.Strings(files)
	return files
}

// parseSmokeMatrixForFile parses smoke-matrix annotations and test function names from a file.
func parseSmokeMatrixForFile(t *testing.T, projectRoot, rel string) (map[string]smokeMatrixEntry, []string) {
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

	entries := make(map[string]smokeMatrixEntry)
	tests := make([]string, 0)
	var pending *smokeMatrixEntry

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if m := smokeAnnotationRe.FindStringSubmatch(line); m != nil {
			pending = &smokeMatrixEntry{
				Decision:    strings.TrimSpace(m[1]),
				Rationale:   strings.TrimSpace(m[2]),
				Destination: strings.TrimSpace(m[3]),
			}
			continue
		}

		if m := smokeFuncRe.FindStringSubmatch(line); m != nil {
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

// listUnitTests returns a map of unit test function names in a package directory.
func listUnitTests(t *testing.T, projectRoot, relDir string) map[string]struct{} {
	t.Helper()
	return packageTestNameIndex(t, projectRoot, relDir)
}

// packageTestNameIndex builds a map of all test function names in a directory tree.
func packageTestNameIndex(t *testing.T, projectRoot, relDir string) map[string]struct{} {
	t.Helper()

	absDir := filepath.Clean(filepath.Join(projectRoot, relDir))

	testNameIndexMu.Lock()
	cached := testNameIndexCache[absDir]
	testNameIndexMu.Unlock()
	if cached != nil {
		return cached
	}

	names := make(map[string]struct{})
	err := filepath.WalkDir(absDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			_ = f.Close()
		}()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			m := testFuncNameRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			names[m[1]] = struct{}{}
		}
		return scanner.Err()
	})
	if err != nil {
		t.Fatalf("index test names in %s: %v", relDir, err)
	}

	testNameIndexMu.Lock()
	testNameIndexCache[absDir] = names
	testNameIndexMu.Unlock()

	return names
}
