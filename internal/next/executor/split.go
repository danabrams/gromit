package executor

import (
	"path/filepath"
)

// NeedsSplit returns true if the changed files suggest the task scope is too
// broad and should be split into smaller tasks.
//
// Heuristics:
//   - 3+ distinct parent directories in changedFiles triggers split.
//   - Changed file count exceeding 2x the expected area directory count triggers split.
func NeedsSplit(changedFiles, expectedArea []string) bool {
	dirs := distinctParentDirs(changedFiles)

	if len(dirs) >= 3 {
		return true
	}

	expectedCount := len(expectedArea)
	if expectedCount == 0 {
		expectedCount = 1
	}
	if len(changedFiles) > 2*expectedCount {
		return true
	}

	return false
}

func distinctParentDirs(files []string) map[string]struct{} {
	dirs := make(map[string]struct{})
	for _, f := range files {
		dir := filepath.Dir(f)
		dirs[dir] = struct{}{}
	}
	return dirs
}
