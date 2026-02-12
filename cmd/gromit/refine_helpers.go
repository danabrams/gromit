package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// getSpecFiles returns a list of .md files in the specs directory.
// Creates the directory if it doesn't exist.
func getSpecFiles(specsDir string) ([]string, error) {
	return listMarkdownFiles(specsDir)
}

// listMarkdownFiles returns all .md files in the given directory.
// Creates the directory if it doesn't exist.
func listMarkdownFiles(dir string) ([]string, error) {
	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	return files, nil
}

// containsSpec checks if a string slice contains a value
func containsSpec(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// formatTypeLabel formats type as colored label
func formatTypeLabel(ideaType string) string {
	typeMap := map[string]string{
		"feature": "[feature]",
		"bug":     "[bug]    ",
		"chore":   "[chore]  ",
		"unknown": "[unknown]",
	}

	if label, ok := typeMap[ideaType]; ok {
		return label
	}
	return fmt.Sprintf("[%-7s]", ideaType)
}

// extractSpecTitle reads a spec file and returns the first level-1 markdown heading text.
// Returns empty string if file is missing, empty, or has no level-1 heading.
// Handles frontmatter blocks (YAML between --- markers).
func extractSpecTitle(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inFrontmatter := false
	firstLine := true

	for scanner.Scan() {
		line := scanner.Text()

		// Check for frontmatter start/end on first line or after frontmatter start
		if firstLine && line == "---" {
			inFrontmatter = true
			firstLine = false
			continue
		}
		firstLine = false

		if inFrontmatter {
			if line == "---" {
				inFrontmatter = false
			}
			continue
		}

		// Look for level-1 heading
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}

	if err := scanner.Err(); err != nil {
		return ""
	}

	return ""
}
