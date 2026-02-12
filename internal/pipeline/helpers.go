package pipeline

import (
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
