package methodology

import "strings"

// ParseDiffFiles extracts file paths from git diff output.
// Returns a slice of file paths in the order they appear.
func ParseDiffFiles(diff string) []string {
	if diff == "" {
		return nil
	}

	var files []string
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				filePath := strings.TrimPrefix(parts[3], "b/")
				files = append(files, filePath)
			}
		}
	}
	return files
}

// IsTestOnlyDiff returns true if the diff is empty or only contains changes
// to test files (*_test.go).
func IsTestOnlyDiff(diff string) bool {
	if strings.TrimSpace(diff) == "" {
		return true
	}

	files := ParseDiffFiles(diff)
	for _, filePath := range files {
		if !strings.HasSuffix(filePath, "_test.go") {
			return false
		}
	}
	return true
}
