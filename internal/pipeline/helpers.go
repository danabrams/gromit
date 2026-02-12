package pipeline

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ListMarkdownFiles returns all .md files in the given directory.
// Creates the directory if it doesn't exist.
func ListMarkdownFiles(dir string) ([]string, error) {
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

// DiffFiles returns files in after that are not in before.
func DiffFiles(before, after []string) []string {
	beforeSet := make(map[string]bool)
	for _, f := range before {
		beforeSet[f] = true
	}

	var diff []string
	for _, f := range after {
		if !beforeSet[f] {
			diff = append(diff, f)
		}
	}

	return diff
}

// ExtractSpecTitle reads a spec file and returns the first level-1 markdown heading text.
// Returns empty string if file is missing, empty, or has no level-1 heading.
// Handles frontmatter blocks (YAML between --- markers).
func ExtractSpecTitle(path string) string {
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

// WriteTempPrompt writes a prompt to a temporary file and returns the path and a cleanup function.
// The cleanup function should be called to remove the temp file when done.
func WriteTempPrompt(tmpDir, prompt string) (path string, cleanup func(), err error) {
	// Ensure tmp directory exists
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("creating tmp dir: %w", err)
	}

	promptFile, err := os.CreateTemp(tmpDir, "prompt-*.md")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp prompt file: %w", err)
	}
	promptPath := promptFile.Name()

	if _, err := promptFile.WriteString(prompt); err != nil {
		promptFile.Close()
		os.Remove(promptPath)
		return "", nil, fmt.Errorf("writing prompt file: %w", err)
	}
	promptFile.Close()

	cleanup = func() {
		os.Remove(promptPath)
	}

	return promptPath, cleanup, nil
}
