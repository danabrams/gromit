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
		// Each one must have //go:build acceptance or //go:build e2e_live.
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

			if !hasAcceptedBuildTag(t, path) {
				rel, _ := filepath.Rel(projectRoot, path)
				untagged = append(untagged, rel)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking project tree: %v", err)
		}

		if len(untagged) > 0 {
			t.Errorf("found %d *_acceptance_test.go files without //go:build acceptance or //go:build e2e_live:\n  %s",
				len(untagged), strings.Join(untagged, "\n  "))
		}
	})

	t.Run("live-external tests must be e2e_live-tagged", func(t *testing.T) {
		var violations []string
		markers := []string{
			"GROMIT_RUN_INTERACTIVE_ACCEPTANCE",
			"command -v claude",
			"command -v bd",
		}

		err := filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "vendor" || d.Name() == ".git" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(projectRoot, path)
			if relErr != nil {
				return relErr
			}
			if rel == filepath.Join("cmd", "gromit", "final_verification_test.go") {
				return nil
			}

			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content := string(src)

			for _, marker := range markers {
				if !strings.Contains(content, marker) {
					continue
				}
				if !hasBuildTag(t, path, "e2e_live") {
					violations = append(violations, rel)
				}
				break
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking project tree: %v", err)
		}

		if len(violations) > 0 {
			t.Errorf("found live-external tests without //go:build e2e_live:\n  %s", strings.Join(violations, "\n  "))
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

// hasAcceptedBuildTag reads the first few lines of a file and checks for
// //go:build acceptance, //go:build e2e_live, or matching old-style tags.
func hasAcceptedBuildTag(t *testing.T, path string) bool {
	t.Helper()
	return hasBuildTag(t, path, "acceptance") || hasBuildTag(t, path, "e2e_live")
}

func hasBuildTag(t *testing.T, path string, tag string) bool {
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
		if line == "//go:build "+tag || strings.HasPrefix(line, "//go:build "+tag+" ") ||
			(strings.Contains(line, "//go:build") && strings.Contains(line, tag)) {
			return true
		}
		// Also handle old-style build tags
		if line == "// +build "+tag || strings.HasPrefix(line, "// +build "+tag+" ") {
			return true
		}
		// Stop at package declaration — if we haven't found a build tag by then, there isn't one
		if strings.HasPrefix(line, "package ") {
			return false
		}
	}
	return false
}
