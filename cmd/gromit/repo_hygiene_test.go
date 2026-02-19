package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepoHygiene_TestBinaryRemoved verifies that the accidentally committed
// test binary cmd/gromit/gromit.test has been deleted from the repository and
// that .gitignore prevents recurrence.
func TestRepoHygiene_TestBinaryRemoved(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}

	t.Run("gromit.test binary is not tracked by git", func(t *testing.T) {
		cmd := exec.Command("git", "ls-files", "cmd/gromit/gromit.test")
		cmd.Dir = projectRoot
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git ls-files failed: %v", err)
		}
		tracked := strings.TrimSpace(string(out))
		if tracked != "" {
			t.Errorf("cmd/gromit/gromit.test is still tracked by git; should be removed with git rm")
		}
	})

	t.Run("gromit.test binary does not exist on disk", func(t *testing.T) {
		binaryPath := filepath.Join(projectRoot, "cmd", "gromit", "gromit.test")
		if _, err := os.Stat(binaryPath); err == nil {
			t.Error("cmd/gromit/gromit.test still exists on disk; should be deleted")
		} else if !os.IsNotExist(err) {
			t.Fatalf("unexpected error checking for gromit.test: %v", err)
		}
	})

	t.Run("gitignore contains test binary pattern", func(t *testing.T) {
		gitignorePath := filepath.Join(projectRoot, ".gitignore")
		data, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("could not read .gitignore: %v", err)
		}
		lines := strings.Split(string(data), "\n")
		found := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "*.test" {
				found = true
				break
			}
		}
		if !found {
			t.Error(".gitignore does not contain '*.test' pattern to prevent test binary recurrence")
		}
	})
}

func TestRepoHygiene_CodexHomeIgnored(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}

	gitignorePath := filepath.Join(projectRoot, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("could not read .gitignore: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	found := false
	for _, line := range lines {
		if strings.TrimSpace(line) == ".codex-home/" {
			found = true
			break
		}
	}
	if !found {
		t.Error(".gitignore does not contain '.codex-home/' to prevent local Codex runtime artifacts from being tracked")
	}
}

func TestRepoHygiene_CodexHomeNotTracked(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}

	cmd := exec.Command("git", "ls-files", ".codex-home")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files failed: %v", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Error(".codex-home/ is still tracked by git; remove with 'git rm -r --cached .codex-home/'")
	}
}
