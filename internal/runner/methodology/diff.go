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

// DetectTouchedPackages extracts unique Go package paths from git diff output.
// Only .go files are considered; non-Go files are ignored.
func DetectTouchedPackages(diff string) []string {
	files := ParseDiffFiles(diff)
	if len(files) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	packages := make([]string, 0, len(files))

	for _, filePath := range files {
		if !strings.HasSuffix(filePath, ".go") {
			continue
		}

		lastSlash := strings.LastIndex(filePath, "/")
		pkgPath := "."
		if lastSlash > 0 {
			pkgPath = filePath[:lastSlash]
		}

		if seen[pkgPath] {
			continue
		}
		seen[pkgPath] = true
		packages = append(packages, pkgPath)
	}

	return packages
}
