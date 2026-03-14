package specloop

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// GitFilesChanged returns a FilesChangedFunc that detects changed files using git.
// It combines `git diff --name-only HEAD` (modified tracked files) with
// `git ls-files --others --exclude-standard` (new untracked files).
// If the directory is not a git repository, it returns an empty list with no error.
func GitFilesChanged() FilesChangedFunc {
	return func(workDir string) ([]string, error) {
		absDir, err := filepath.Abs(workDir)
		if err != nil {
			return []string{}, nil
		}

		// Check if this is a git repo. If not, return empty gracefully.
		checkCmd := exec.Command("git", "-C", absDir, "rev-parse", "--git-dir")
		if err := checkCmd.Run(); err != nil {
			return []string{}, nil
		}

		seen := make(map[string]bool)
		var files []string

		// Tracked files that differ from HEAD (staged + unstaged).
		diffCmd := exec.Command("git", "-C", absDir, "diff", "--name-only", "HEAD")
		if out, err := diffCmd.Output(); err == nil {
			for _, f := range splitLines(string(out)) {
				if f != "" && !seen[f] {
					seen[f] = true
					files = append(files, f)
				}
			}
		}

		// Untracked files (new files not yet added).
		untrackedCmd := exec.Command("git", "-C", absDir, "ls-files", "--others", "--exclude-standard")
		if out, err := untrackedCmd.Output(); err == nil {
			for _, f := range splitLines(string(out)) {
				if f != "" && !seen[f] {
					seen[f] = true
					files = append(files, f)
				}
			}
		}

		if files == nil {
			files = []string{}
		}
		return files, nil
	}
}

// splitLines splits a string by newlines, trimming whitespace.
func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}
