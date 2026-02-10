package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFinalVerification verifies the acceptance criteria for the
// "Final verification and line count audit" task (gromit-xeub).
func TestFinalVerification(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}

	t.Run("no untagged acceptance test files exist", func(t *testing.T) {
		// Walk the entire project tree and find all *_acceptance_test.go files.
		// Each one must either have //go:build acceptance or not exist at all.
		var untagged []string

		err := filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			// Skip vendor, .git, and other non-source directories
			if d.IsDir() && (d.Name() == "vendor" || d.Name() == ".git" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), "_acceptance_test.go") {
				return nil
			}

			if !hasAcceptanceBuildTag(t, path) {
				rel, _ := filepath.Rel(projectRoot, path)
				untagged = append(untagged, rel)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking project tree: %v", err)
		}

		if len(untagged) > 0 {
			t.Errorf("found %d *_acceptance_test.go files without //go:build acceptance:\n  %s",
				len(untagged), strings.Join(untagged, "\n  "))
		}
	})

	t.Run("total acceptance test lines reduced by at least 30 percent", func(t *testing.T) {
		// The original acceptance test line count was 8,370 lines across 24 files.
		// After cleanup, the total must be 5,859 or fewer (30% reduction).
		const originalLines = 8370
		const maxAllowedLines = 5859

		totalLines := 0
		var files []string

		err := filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "vendor" || d.Name() == ".git" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), "_acceptance_test.go") {
				return nil
			}

			lines, countErr := countFileLines(path)
			if countErr != nil {
				t.Errorf("counting lines in %s: %v", path, countErr)
				return nil
			}
			totalLines += lines
			rel, _ := filepath.Rel(projectRoot, path)
			files = append(files, rel)
			return nil
		})
		if err != nil {
			t.Fatalf("walking project tree: %v", err)
		}

		if totalLines > maxAllowedLines {
			t.Errorf("total acceptance test lines = %d (across %d files), want <= %d (30%% reduction from %d)\nfiles: %s",
				totalLines, len(files), maxAllowedLines, originalLines, strings.Join(files, ", "))
		}
	})

	t.Run("key test behaviors still have coverage", func(t *testing.T) {
		// Spot-check that the consolidated test files still exist and cover
		// the key behaviors that were in the original acceptance test files.
		// These files should contain the merged test content.
		behaviorChecks := []struct {
			name     string
			file     string // relative to project root
			contains []string
		}{
			{
				name: "explore tests consolidated in explore_test.go",
				file: "cmd/gromit/explore_test.go",
				contains: []string{
					"buildExplorePrompt",
					"setupExploreTest",
				},
			},
			{
				name: "label filtering tests consolidated with table-driven pattern",
				file: "internal/runner/label_filter_test.go",
				contains: []string{
					"setupLabelFilterTest",
					"t.Run(",
				},
			},
			{
				name: "scope tests include ValidateFlags with three-way exclusivity",
				file: "internal/scope/scope_test.go",
				contains: []string{
					"ValidateFlags",
					"since",
				},
			},
			{
				name: "bead label method tests preserved",
				file: "internal/bead/bead_test.go",
				contains: []string{
					"ReadyWithLabel",
				},
			},
			{
				name: "retro filtered hash eviction tests preserved with build tag",
				file: "internal/retro/filtered_hash_eviction_acceptance_test.go",
				contains: []string{
					"//go:build acceptance",
					"setupHashEviction",
				},
			},
		}

		for _, bc := range behaviorChecks {
			t.Run(bc.name, func(t *testing.T) {
				fullPath := filepath.Join(projectRoot, bc.file)
				src, err := os.ReadFile(fullPath)
				if err != nil {
					t.Fatalf("cannot read %s: %v", bc.file, err)
				}
				content := string(src)
				for _, want := range bc.contains {
					if !strings.Contains(content, want) {
						t.Errorf("%s should contain %q", bc.file, want)
					}
				}
			})
		}
	})

	t.Run("no cleanup acceptance test files remain", func(t *testing.T) {
		// The per-package cleanup acceptance test files were scaffolding for
		// the cleanup effort. After the final verification task completes,
		// these should be removed or renamed (they are themselves untagged
		// *_acceptance_test.go files).
		cleanupFiles := []string{
			"cmd/gromit/explore_cleanup_acceptance_test.go",
			"cmd/gromit/review_cleanup_acceptance_test.go",
			"cmd/gromit/retro_cleanup_acceptance_test.go",
			"internal/runner/runner_cleanup_acceptance_test.go",
			"internal/bead/bead_cleanup_acceptance_test.go",
			"internal/scope/scope_cleanup_acceptance_test.go",
			"internal/retro/retro_cleanup_acceptance_test.go",
		}

		for _, relPath := range cleanupFiles {
			fullPath := filepath.Join(projectRoot, relPath)
			if _, err := os.Stat(fullPath); err == nil {
				t.Errorf("cleanup scaffolding file %s should be removed after final verification", relPath)
			} else if !os.IsNotExist(err) {
				t.Errorf("unexpected error checking %s: %v", relPath, err)
			}
		}
	})
}

// hasAcceptanceBuildTag reads the first few lines of a file and checks for
// //go:build acceptance (or the older // +build acceptance).
func hasAcceptanceBuildTag(t *testing.T, path string) bool {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Build tags must appear before the package declaration, so check
	// the first 10 lines (covers blank lines + comments).
	for i := 0; i < 10 && scanner.Scan(); i++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "//go:build acceptance" || strings.HasPrefix(line, "//go:build acceptance ") ||
			strings.Contains(line, "//go:build") && strings.Contains(line, "acceptance") {
			return true
		}
		// Also handle old-style build tags
		if line == "// +build acceptance" || strings.HasPrefix(line, "// +build acceptance ") {
			return true
		}
		// Stop at package declaration — if we haven't found a build tag by then, there isn't one
		if strings.HasPrefix(line, "package ") {
			return false
		}
	}
	return false
}

// countFileLines counts the number of lines in a file.
func countFileLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}
