package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type scannedTestFile struct {
	path    string
	relPath string
	content string
}

// TestFinalVerification verifies the acceptance criteria for the
// "Final verification and line count audit" task (gromit-xeub).
func TestFinalVerification(t *testing.T) {
	t.Parallel()
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}
	testFiles, err := scanProjectTestFiles(projectRoot)
	if err != nil {
		t.Fatalf("scanning test files: %v", err)
	}

	t.Run("no untagged acceptance test files exist", func(t *testing.T) {
		t.Parallel(
		// Walk the entire project tree and find all *_acceptance_test.go files.
		// Each one must have //go:build acceptance or //go:build e2e_live.
		)

		var untagged []string

		for _, file := range testFiles {
			if !strings.HasSuffix(file.relPath, "_acceptance_test.go") {
				continue
			}
			if !hasAcceptedBuildTag(file.content) {
				untagged = append(untagged, file.relPath)
			}
		}

		if len(untagged) > 0 {
			t.Errorf("found %d *_acceptance_test.go files without //go:build acceptance/contract/e2e/e2e_live:\n  %s",
				len(untagged), strings.Join(untagged, "\n  "))
		}
	})

	t.Run("live-external tests must be e2e_live-tagged", func(t *testing.T) {
		t.Parallel()
		var violations []string
		markers := []string{
			"GROMIT_RUN_INTERACTIVE_ACCEPTANCE",
			"command -v claude",
			"command -v bd",
		}

		for _, file := range testFiles {
			if file.relPath == filepath.Join("cmd", "gromit", "final_verification_test.go") {
				continue
			}
			for _, marker := range markers {
				if !strings.Contains(file.content, marker) {
					continue
				}
				if !hasBuildTagContent(file.content, "e2e_live") {
					violations = append(violations, file.relPath)
				}
				break
			}
		}

		if len(violations) > 0 {
			t.Errorf("found live-external tests without //go:build e2e_live:\n  %s", strings.Join(violations, "\n  "))
		}
	})

	t.Run("key test behaviors still have coverage", func(t *testing.T) {
		t.Parallel(
		// Spot-check that the consolidated test files still exist and cover
		// the key behaviors that were in the original acceptance test files.
		// These files should contain the merged test content.
		)

		behaviorChecks := []struct {
			name     string
			file     string // relative to project root
			contains []string
		}{
			{
				name: "explore tests consolidated in explore_test.go",
				file: "cmd/gromit/explore_test.go",
				contains: []string{
					"setupExploreTest",
					"explorePromptRenderer",
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
				t.Parallel()
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
		t.Parallel(
		// The per-package cleanup acceptance test files were scaffolding for
		// the cleanup effort. After the final verification task completes,
		// these should be removed or renamed (they are themselves untagged
		// *_acceptance_test.go files).
		)

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

func TestFinalVerification_NoBuildExplorePromptReference(t *testing.T) {
	t.Parallel()
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(projectRoot, "cmd", "gromit", "final_verification_test.go"))
	if err != nil {
		t.Fatalf("failed to read final_verification_test.go: %v", err)
	}

	needle := strings.Join([]string{"build", "ExplorePrompt"}, "")
	if strings.Contains(string(content), needle) {
		t.Fatalf("final_verification_test.go must not reference %q", needle)
	}
}

func scanProjectTestFiles(projectRoot string) ([]scannedTestFile, error) {
	files := make([]scannedTestFile, 0, 512)
	err := filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "vendor" || d.Name() == ".git" || d.Name() == ".gromit" || d.Name() == ".claude" || d.Name() == ".worktrees" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".-gromit-")) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		relPath, relErr := filepath.Rel(projectRoot, path)
		if relErr != nil {
			return relErr
		}
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		files = append(files, scannedTestFile{
			path:    path,
			relPath: relPath,
			content: string(src),
		})
		return nil
	})
	return files, err
}

// hasAcceptedBuildTag checks the first few lines for //go:build acceptance/e2e_live/contract/e2e or old-style tags.
func hasAcceptedBuildTag(content string) bool {
	return hasBuildTagContent(content, "acceptance") || hasBuildTagContent(content, "e2e_live") ||
		hasBuildTagContent(content, "contract") || hasBuildTagContent(content, "e2e")
}

func hasBuildTagContent(content string, tag string) bool {
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines) && i < 10; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "//go:build "+tag || strings.HasPrefix(line, "//go:build "+tag+" ") ||
			(strings.Contains(line, "//go:build") && strings.Contains(line, tag)) {
			return true
		}
		if line == "// +build "+tag || strings.HasPrefix(line, "// +build "+tag+" ") {
			return true
		}
		if strings.HasPrefix(line, "package ") {
			return false
		}
	}
	return false
}

func hasBuildTag(t *testing.T, path string, tag string) bool {
	t.Helper()

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	return hasBuildTagContent(string(src), tag)
}
