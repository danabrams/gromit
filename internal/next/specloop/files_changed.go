package specloop

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// GitFilesChanged returns a FilesChangedFunc that detects changed files using
// content-hash snapshots. The returned closure is stateful:
//
//   - First call (before the task): walks git-tracked + untracked files, hashes
//     each one, stores the baseline map path→hash. Returns []string{}, nil.
//   - Second call (after the task): hashes files again, returns the delta —
//     files whose hash changed or that are newly present or that were deleted.
//     After returning the delta, the closure resets so the third call starts a
//     fresh baseline (supporting sequential tasks sharing one closure).
//
// If the directory is not a git repository, both calls return an empty list
// with no error.
func GitFilesChanged() FilesChangedFunc {
	var baseline map[string]string // nil means "no baseline captured yet"

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

		current, err := hashAllFiles(absDir)
		if err != nil {
			return []string{}, nil
		}

		if baseline == nil {
			// First call: capture baseline, return empty.
			baseline = current
			return []string{}, nil
		}

		// Second call: compute delta and reset.
		delta := computeDelta(baseline, current)
		baseline = nil // reset for next task
		return delta, nil
	}
}

// hashAllFiles returns a map of relative file path → sha256 hex hash for all
// git-tracked files and untracked (non-ignored) files in absDir.
// Deleted files are represented by an empty string hash.
func hashAllFiles(absDir string) (map[string]string, error) {
	paths := make(map[string]bool)

	// Tracked files (all files git knows about, not just diffs).
	lsTracked := exec.Command("git", "-C", absDir, "ls-files")
	if out, err := lsTracked.Output(); err == nil {
		for _, f := range splitLines(string(out)) {
			if f != "" {
				paths[f] = true
			}
		}
	}

	// Untracked files (new files not yet added).
	lsUntracked := exec.Command("git", "-C", absDir, "ls-files", "--others", "--exclude-standard")
	if out, err := lsUntracked.Output(); err == nil {
		for _, f := range splitLines(string(out)) {
			if f != "" {
				paths[f] = true
			}
		}
	}

	result := make(map[string]string, len(paths))
	for relPath := range paths {
		absPath := filepath.Join(absDir, relPath)
		content, err := os.ReadFile(absPath)
		if err != nil {
			// File doesn't exist (deleted) — use empty sentinel.
			result[relPath] = ""
			continue
		}
		sum := sha256.Sum256(content)
		result[relPath] = fmt.Sprintf("%x", sum)
	}
	return result, nil
}

// computeDelta returns file paths that differ between before and after snapshots:
//   - Files whose hash changed (including files that went from existing to deleted
//     or appeared as new).
//   - Files present in before but absent in after (deleted after baseline).
func computeDelta(before, after map[string]string) []string {
	seen := make(map[string]bool)
	var delta []string

	// Check all files in before snapshot.
	for path, beforeHash := range before {
		afterHash, exists := after[path]
		if !exists {
			// File was in baseline (tracked) but now gone from both tracked
			// and untracked — treat as deleted.
			afterHash = ""
		}
		if beforeHash != afterHash {
			seen[path] = true
			delta = append(delta, path)
		}
	}

	// Check files newly present in after that weren't in before.
	for path := range after {
		if !seen[path] {
			if _, wasBefore := before[path]; !wasBefore {
				delta = append(delta, path)
			}
		}
	}

	sort.Strings(delta)
	if delta == nil {
		delta = []string{}
	}
	return delta
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
