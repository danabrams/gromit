package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// projectRoot returns the project root directory, failing the test on error.
func projectRoot(t *testing.T) string {
	t.Helper()
	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}
	return root
}

// requireNotTracked asserts that none of the given paths are tracked by git.
func requireNotTracked(t *testing.T, root string, paths ...string) {
	t.Helper()
	args := append([]string{"ls-files"}, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files failed: %v", err)
	}
	if tracked := strings.TrimSpace(string(out)); tracked != "" {
		t.Errorf("files still tracked by git:\n%s", tracked)
	}
}

// requireGitignoreContains asserts that .gitignore contains every listed pattern.
func requireGitignoreContains(t *testing.T, root string, patterns ...string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("could not read .gitignore: %v", err)
	}
	lines := strings.Split(string(data), "\n")
	for _, pattern := range patterns {
		found := false
		for _, line := range lines {
			if strings.TrimSpace(line) == pattern {
				found = true
				break
			}
		}
		if !found {
			t.Errorf(".gitignore missing pattern %q", pattern)
		}
	}
}

func TestRepoHygiene_TestBinaryRemoved(t *testing.T) {
	root := projectRoot(t)

	t.Run("gromit.test binary is not tracked by git", func(t *testing.T) {
		requireNotTracked(t, root, "cmd/gromit/gromit.test")
	})

	t.Run("gromit.test binary does not exist on disk", func(t *testing.T) {
		binaryPath := filepath.Join(root, "cmd", "gromit", "gromit.test")
		if _, err := os.Stat(binaryPath); err == nil {
			t.Error("cmd/gromit/gromit.test still exists on disk; should be deleted")
		} else if !os.IsNotExist(err) {
			t.Fatalf("unexpected error checking for gromit.test: %v", err)
		}
	})

	t.Run("gitignore contains test binary pattern", func(t *testing.T) {
		requireGitignoreContains(t, root, "*.test")
	})
}

func TestRepoHygiene_CodexHomeIgnored(t *testing.T) {
	root := projectRoot(t)
	requireGitignoreContains(t, root, ".codex-home/")
}

func TestRepoHygiene_CodexHomeNotTracked(t *testing.T) {
	root := projectRoot(t)
	requireNotTracked(t, root, ".codex-home")
}

func TestRepoHygiene_ScratchFilesRemovedAndIgnored(t *testing.T) {
	root := projectRoot(t)

	scratchFiles := []string{
		"debug.md", "devug.md", "fixed.md", "fixes.md",
		"progress.md", "testfailure.md", "testfix.md",
		"runner.test",
	}

	t.Run("scratch files are not tracked by git", func(t *testing.T) {
		requireNotTracked(t, root, scratchFiles...)
	})

	t.Run("gitignore contains scratch markdown patterns", func(t *testing.T) {
		requireGitignoreContains(t, root,
			"debug.md", "devug.md", "fixed.md", "fixes.md",
			"progress.md", "testfailure.md", "testfix.md",
		)
	})
}
