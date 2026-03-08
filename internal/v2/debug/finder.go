package debug

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var debugFinderSpecNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// FindPreservedWorktreeBranch finds the preserved worktree path for gromit/spec/<specName>.
func FindPreservedWorktreeBranch(gromitDir, specName string) (string, error) {
	trimmedSpecName := strings.TrimSpace(specName)
	if !debugFinderSpecNamePattern.MatchString(trimmedSpecName) {
		return "", fmt.Errorf("invalid spec name %q", specName)
	}

	repoRoot := filepath.Dir(gromitDir)
	targetBranch := "gromit/spec/" + trimmedSpecName

	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("listing worktrees: %w", err)
	}

	worktreePath, found := findPreservedWorktreePath(string(out), targetBranch)
	if !found {
		return "", fmt.Errorf("no preserved worktree found for branch %q", targetBranch)
	}
	return worktreePath, nil
}

func findPreservedWorktreePath(porcelainOutput, targetBranch string) (string, bool) {
	var currentWorktree string

	for _, raw := range strings.Split(porcelainOutput, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "worktree ") {
			currentWorktree = strings.TrimPrefix(line, "worktree ")
			continue
		}
		if strings.HasPrefix(line, "branch refs/heads/") {
			branch := strings.TrimPrefix(line, "branch refs/heads/")
			if branch == targetBranch && currentWorktree != "" {
				return normalizeFinderWorktreePath(currentWorktree), true
			}
		}
	}

	return "", false
}

func normalizeFinderWorktreePath(path string) string {
	normalized := filepath.Clean(path)
	if runtime.GOOS == "darwin" && strings.HasPrefix(normalized, "/private/var/") {
		return strings.TrimPrefix(normalized, "/private")
	}
	return normalized
}
