//go:build acceptance

package learnings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchiveCreatesArchiveFileAndRemovesFromLearningsMd tests that Archive() writes to
// LEARNINGS_ARCHIVE.md and removes the learning from LEARNINGS.md entirely
func TestArchiveCreatesArchiveFileAndRemovesFromLearningsMd(t *testing.T) {
	// Expected failure: Archive() currently moves learning to f.archived in-memory slice,
	// and Save() writes it to ## Archived section in LEARNINGS.md instead of separate file
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add a learning
	learning, err := f.Add("bead-archive-test", "Learning to be archived", CategoryPatterns)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if learning == nil {
		t.Fatal("learning should not be nil")
	}

	// Archive it
	err = f.Archive(learning.Hash, "no longer relevant")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Verify LEARNINGS_ARCHIVE.md exists and contains the archived learning
	archivePath := filepath.Join(tmpDir, "LEARNINGS_ARCHIVE.md")
	archiveContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("expected LEARNINGS_ARCHIVE.md to exist: %v", err)
	}

	archiveStr := string(archiveContent)
	if !strings.Contains(archiveStr, "Learning to be archived") {
		t.Error("LEARNINGS_ARCHIVE.md should contain the archived learning content")
	}
	if !strings.Contains(archiveStr, "no longer relevant") {
		t.Error("LEARNINGS_ARCHIVE.md should contain the archive reason")
	}
	if !strings.Contains(archiveStr, "bead-archive-test") {
		t.Error("LEARNINGS_ARCHIVE.md should contain the bead ID")
	}

	// Verify LEARNINGS.md does not contain the archived learning or ## Archived section
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	learningsContent, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("failed to read LEARNINGS.md: %v", err)
	}

	learningsStr := string(learningsContent)
	if strings.Contains(learningsStr, "Learning to be archived") {
		t.Error("LEARNINGS.md should not contain the archived learning content")
	}
	if strings.Contains(learningsStr, "## Archived") {
		t.Error("LEARNINGS.md should not contain ## Archived section")
	}
}

// TestSaveOmitsArchivedSectionEvenWhenArchivedExistsInMemory tests that Save() never
// writes an ## Archived section to LEARNINGS.md
func TestSaveOmitsArchivedSectionEvenWhenArchivedExistsInMemory(t *testing.T) {
	// Expected failure: Save() currently writes ## Archived section when f.archived slice is not empty
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add learnings to confirmed and provisional
	l1, _ := f.Add("bead-1", "First learning", CategoryPatterns)
	l2, _ := f.Add("bead-2", "Second learning", CategoryConventions)
	if l1 == nil || l2 == nil {
		t.Fatal("learnings should not be nil")
	}

	// Archive one of them (this will populate f.archived in old implementation)
	err := f.Archive(l1.Hash, "test")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Save explicitly
	err = f.Save()
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Read LEARNINGS.md
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	content, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("failed to read LEARNINGS.md: %v", err)
	}

	contentStr := string(content)

	// Should contain required sections
	if !strings.Contains(contentStr, "## Confirmed") {
		t.Error("LEARNINGS.md should contain ## Confirmed section")
	}
	if !strings.Contains(contentStr, "## Provisional") {
		t.Error("LEARNINGS.md should contain ## Provisional section")
	}

	// Should NOT contain ## Archived section
	if strings.Contains(contentStr, "## Archived") {
		t.Error("LEARNINGS.md should not contain ## Archived section - archived learnings belong in separate file")
	}

	// Should not contain the archived learning content
	if strings.Contains(contentStr, "First learning") {
		t.Error("LEARNINGS.md should not contain archived learning content")
	}
}

// TestAddRejectsArchivedDuplicateFromState tests that Add() returns nil when the
// content hash matches a previously archived learning (tracked in state)
func TestAddRejectsArchivedDuplicateFromState(t *testing.T) {
	// Expected failure: Add() currently only checks f.archived slice (in-memory),
	// not the archived hashes from state. After change, it should check both
	// confirmed/provisional (current) and archived hashes (from state)
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	content := "Previously archived learning"

	// Add and immediately archive a learning
	l1, err := f.Add("bead-original", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if l1 == nil {
		t.Fatal("first learning should not be nil")
	}

	err = f.Archive(l1.Hash, "test")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Reload to simulate a new run (clears in-memory state)
	f2, _ := NewFile(tmpDir)
	err = f2.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	// Try to add the same content again
	l2, err := f2.Add("bead-duplicate", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}

	// Should be rejected as duplicate (because hash is in archived set from state)
	if l2 != nil {
		t.Error("Add() should return nil for content that was previously archived, even after reload")
	}

	// Should not add to any section
	if len(f2.provisional) != 0 {
		t.Errorf("expected 0 provisional, got %d", len(f2.provisional))
	}
	if len(f2.confirmed) != 0 {
		t.Errorf("expected 0 confirmed, got %d", len(f2.confirmed))
	}
}

// TestAddFilteredGenericWritesToArchiveFileNotLearningsMd tests that Add() with a
// filter function that returns isGeneric=true writes to LEARNINGS_ARCHIVE.md
func TestAddFilteredGenericWritesToArchiveFileNotLearningsMd(t *testing.T) {
	// Expected failure: Add() currently writes filtered generic learnings to f.archived
	// in-memory slice, which then appears in LEARNINGS.md ## Archived section
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Set filter that classifies everything as generic
	f.SetFilter(func(content string) (bool, error) {
		return true, nil // Generic
	})

	learning, err := f.Add("bead-generic", "Always write unit tests", CategoryPatterns)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	// Should return nil (filtered as generic)
	if learning != nil {
		t.Error("filtered generic learning should return nil from Add()")
	}

	// Should be in LEARNINGS_ARCHIVE.md
	archivePath := filepath.Join(tmpDir, "LEARNINGS_ARCHIVE.md")
	archiveContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("expected LEARNINGS_ARCHIVE.md to exist: %v", err)
	}

	archiveStr := string(archiveContent)
	if !strings.Contains(archiveStr, "Always write unit tests") {
		t.Error("LEARNINGS_ARCHIVE.md should contain filtered generic learning")
	}
	if !strings.Contains(archiveStr, "filtered: generic engineering advice") {
		t.Error("LEARNINGS_ARCHIVE.md should contain filter reason")
	}

	// Should NOT be in LEARNINGS.md (neither provisional nor archived section)
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	learningsContent, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("failed to read LEARNINGS.md: %v", err)
	}

	learningsStr := string(learningsContent)
	if strings.Contains(learningsStr, "Always write unit tests") {
		t.Error("LEARNINGS.md should not contain filtered generic learning")
	}
}

// TestLoadTriggersOnTimeMigrationOfArchivedSection tests that Load() moves
// ## Archived section from LEARNINGS.md to LEARNINGS_ARCHIVE.md on first run
func TestLoadTriggersOnTimeMigrationOfArchivedSection(t *testing.T) {
	// Expected failure: Load() does not implement migration logic yet
	tmpDir := t.TempDir()

	// Create LEARNINGS.md with ## Archived section (simulating old format)
	learningsContent := `# Learnings

---

## Confirmed

### 2026-02-01 | bead-1 | patterns
Active confirmed learning

---

## Provisional

*No provisional learnings.*

---

## Archived

### 2026-01-10 | bead-old-1 | conventions
Old archived learning one

*Archived from provisional: outdated*

### 2026-01-12 | bead-old-2 | gotchas
Old archived learning two

*Archived from confirmed: superseded*
`
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	err := os.WriteFile(learningsPath, []byte(learningsContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Load triggers migration
	f, _ := NewFile(tmpDir)
	err = f.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	// LEARNINGS_ARCHIVE.md should exist with archived entries
	archivePath := filepath.Join(tmpDir, "LEARNINGS_ARCHIVE.md")
	archiveContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("expected LEARNINGS_ARCHIVE.md to be created during migration: %v", err)
	}

	archiveStr := string(archiveContent)
	if !strings.Contains(archiveStr, "Old archived learning one") {
		t.Error("archive file should contain first migrated entry")
	}
	if !strings.Contains(archiveStr, "Old archived learning two") {
		t.Error("archive file should contain second migrated entry")
	}
	if !strings.Contains(archiveStr, "bead-old-1") {
		t.Error("archive file should contain first bead ID")
	}
	if !strings.Contains(archiveStr, "bead-old-2") {
		t.Error("archive file should contain second bead ID")
	}

	// LEARNINGS.md should no longer have ## Archived section
	learningsContentAfter, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("failed to read LEARNINGS.md after migration: %v", err)
	}

	learningsStr := string(learningsContentAfter)
	if strings.Contains(learningsStr, "## Archived") {
		t.Error("LEARNINGS.md should not contain ## Archived section after migration")
	}
	if strings.Contains(learningsStr, "Old archived learning one") {
		t.Error("LEARNINGS.md should not contain migrated archived content")
	}

	// Confirmed section should still be present
	if !strings.Contains(learningsStr, "Active confirmed learning") {
		t.Error("LEARNINGS.md should still contain confirmed learning after migration")
	}
}

// TestMigrationPreventsDuplicateArchivedEntries tests that migrated archived
// learnings are rejected as duplicates when added again
func TestMigrationPreventsDuplicateArchivedEntries(t *testing.T) {
	// Expected failure: Load() does not track migrated hashes, so Add() won't reject them
	tmpDir := t.TempDir()

	// Create LEARNINGS.md with ## Archived section
	learningsContent := `# Learnings

---

## Confirmed

*No confirmed learnings yet.*

---

## Provisional

*No provisional learnings.*

---

## Archived

### 2026-01-10 | bead-archived | patterns
This was already archived
`
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	err := os.WriteFile(learningsPath, []byte(learningsContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Load triggers migration
	f, _ := NewFile(tmpDir)
	err = f.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	// Try to add the same content that was in archived section
	learning, err := f.Add("bead-new", "This was already archived", CategoryPatterns)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	// Should be rejected as duplicate
	if learning != nil {
		t.Error("Add() should reject content that was migrated from ## Archived section")
	}
}

// TestMultipleArchiveOperationsAppendToArchiveFile tests that calling Archive()
// multiple times appends all learnings to LEARNINGS_ARCHIVE.md
func TestMultipleArchiveOperationsAppendToArchiveFile(t *testing.T) {
	// Expected failure: Archive() currently updates f.archived in-memory slice
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add and archive multiple learnings
	l1, _ := f.Add("bead-1", "First learning", CategoryPatterns)
	l2, _ := f.Add("bead-2", "Second learning", CategoryConventions)
	l3, _ := f.Add("bead-3", "Third learning", CategoryGotchas)

	f.Archive(l1.Hash, "reason 1")
	f.Archive(l2.Hash, "reason 2")
	f.Archive(l3.Hash, "reason 3")

	// Verify all three are in LEARNINGS_ARCHIVE.md
	archivePath := filepath.Join(tmpDir, "LEARNINGS_ARCHIVE.md")
	archiveContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("expected LEARNINGS_ARCHIVE.md to exist: %v", err)
	}

	archiveStr := string(archiveContent)
	if !strings.Contains(archiveStr, "First learning") {
		t.Error("archive file should contain first archived learning")
	}
	if !strings.Contains(archiveStr, "Second learning") {
		t.Error("archive file should contain second archived learning")
	}
	if !strings.Contains(archiveStr, "Third learning") {
		t.Error("archive file should contain third archived learning")
	}
	if !strings.Contains(archiveStr, "bead-1") || !strings.Contains(archiveStr, "bead-2") || !strings.Contains(archiveStr, "bead-3") {
		t.Error("archive file should contain all three bead IDs")
	}

	// Verify none are in LEARNINGS.md
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	learningsContent, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("failed to read LEARNINGS.md: %v", err)
	}

	learningsStr := string(learningsContent)
	if strings.Contains(learningsStr, "First learning") || strings.Contains(learningsStr, "Second learning") || strings.Contains(learningsStr, "Third learning") {
		t.Error("LEARNINGS.md should not contain any of the archived learnings")
	}
}

// TestArchiveThenReloadThenAddDuplicate tests the full cycle: archive a learning,
// reload the file (clearing in-memory state), then try to add a duplicate
func TestArchiveThenReloadThenAddDuplicate(t *testing.T) {
	// Expected failure: After reload, the archived hash is not tracked, so duplicate is not detected
	tmpDir := t.TempDir()

	// First session: add and archive
	f1, _ := NewFile(tmpDir)
	content := "Unique learning content for dedup test"
	l1, err := f1.Add("bead-session-1", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if l1 == nil {
		t.Fatal("first learning should not be nil")
	}

	err = f1.Archive(l1.Hash, "test archive")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Second session: reload and try to add duplicate
	f2, _ := NewFile(tmpDir)
	err = f2.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	l2, err := f2.Add("bead-session-2", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}

	// Should be rejected as duplicate
	if l2 != nil {
		t.Error("Add() should reject content that was archived in a previous session")
	}

	// Should have zero learnings in provisional/confirmed
	if len(f2.provisional) != 0 {
		t.Errorf("expected 0 provisional, got %d", len(f2.provisional))
	}
	if len(f2.confirmed) != 0 {
		t.Errorf("expected 0 confirmed, got %d", len(f2.confirmed))
	}
}
