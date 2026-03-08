package debug

import (
	"fmt"
	"os"
	"path/filepath"
)

// PersistLearning appends a learning entry to the LEARNINGS.md file.
// Creates the file if it doesn't exist.
func PersistLearning(learningsPath, entry string) error {
	if learningsPath == "" {
		return ErrEmptyPath
	}

	if entry == "" {
		return nil // Nothing to persist
	}

	// Create parent directories if needed
	dir := filepath.Dir(learningsPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
	}

	// Check if file exists
	var content []byte
	if _, err := os.Stat(learningsPath); err == nil {
		// File exists, read it
		var readErr error
		content, readErr = os.ReadFile(learningsPath)
		if readErr != nil {
			return fmt.Errorf("reading learnings file: %w", readErr)
		}
	}
	// If file doesn't exist, content remains empty

	// Append the new entry
	newContent := string(content) + entry + "\n"
	if err := os.WriteFile(learningsPath, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("writing learnings file: %w", err)
	}

	return nil
}
