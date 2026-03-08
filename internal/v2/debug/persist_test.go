package debug

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPersistLearning_AppendsToLearningsFile adds learning entry to LEARNINGS.md.
func TestPersistLearning_AppendsToLearningsFile(t *testing.T) {
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	// Create initial LEARNINGS.md
	initialContent := "# Learnings\n\n## Existing Entry\n\nOld pattern.\n"
	if err := os.WriteFile(learningsPath, []byte(initialContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Persist a learning
	entry := "## New Pattern\n\nThis is a new learning.\n"
	err := PersistLearning(learningsPath, entry)
	if err != nil {
		t.Fatalf("PersistLearning failed: %v", err)
	}

	// Verify the learning was appended
	content, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "New Pattern") {
		t.Error("learning not found in LEARNINGS.md")
	}
	if !strings.Contains(contentStr, "Existing Entry") {
		t.Error("existing learning was removed")
	}
}

// TestPersistLearning_CreatesFileIfMissing creates LEARNINGS.md if it doesn't exist.
func TestPersistLearning_CreatesFileIfMissing(t *testing.T) {
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")

	entry := "## First Learning\n\nInitial pattern.\n"
	err := PersistLearning(learningsPath, entry)
	if err != nil {
		t.Fatalf("PersistLearning failed: %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "First Learning") {
		t.Error("learning not persisted to new file")
	}
}

// TestPersistLearning_HandlesEmptyPath returns error for empty path.
func TestPersistLearning_HandlesEmptyPath(t *testing.T) {
	err := PersistLearning("", "entry")
	if err == nil {
		t.Error("expected error for empty path, got nil")
	}
}
