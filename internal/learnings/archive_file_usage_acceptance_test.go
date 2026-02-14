//go:build acceptance

package learnings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestArchiveWritesToArchiveFileNotLearningsMd tests that Archive() appends to
// LEARNINGS_ARCHIVE.md instead of adding to the ## Archived section in LEARNINGS.md
func TestArchiveWritesToArchiveFileNotLearningsMd(t *testing.T) {
	// Expected failure: Archive() still writes to f.archived in-memory slice and Save() writes ## Archived section
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add and archive a learning
	learning, err := f.Add("bead-1", "Learning to archive", CategoryPatterns)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if learning == nil {
		t.Fatal("learning should not be nil")
	}

	err = f.Archive(learning.Hash, "no longer relevant")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// LEARNINGS.md should NOT contain ## Archived section
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	learningsContent, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("failed to read LEARNINGS.md: %v", err)
	}

	if strings.Contains(string(learningsContent), "## Archived") {
		t.Error("LEARNINGS.md should not contain ## Archived section after Archive() call")
	}

	// LEARNINGS_ARCHIVE.md should contain the archived learning
	archivePath := filepath.Join(tmpDir, "LEARNINGS_ARCHIVE.md")
	archiveContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("failed to read LEARNINGS_ARCHIVE.md: %v", err)
	}

	archiveStr := string(archiveContent)
	if !strings.Contains(archiveStr, "Learning to archive") {
		t.Error("LEARNINGS_ARCHIVE.md should contain the archived learning content")
	}
	if !strings.Contains(archiveStr, "no longer relevant") {
		t.Error("LEARNINGS_ARCHIVE.md should contain the archive reason")
	}
	if !strings.Contains(archiveStr, "bead-1") {
		t.Error("LEARNINGS_ARCHIVE.md should contain the bead ID")
	}
}

// TestArchiveRemovesFromLearningsMd tests that Archive() removes the learning from LEARNINGS.md
func TestArchiveRemovesFromLearningsMd(t *testing.T) {
	// Expected failure: Archive() still moves to f.archived in-memory instead of removing entirely
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add a learning
	learning, err := f.Add("bead-test", "Test learning content", CategoryConventions)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if learning == nil {
		t.Fatal("learning should not be nil")
	}

	// Verify it's in LEARNINGS.md (provisional section)
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	contentBefore, _ := os.ReadFile(learningsPath)
	if !strings.Contains(string(contentBefore), "Test learning content") {
		t.Fatal("learning should be in LEARNINGS.md before archiving")
	}

	// Archive it
	err = f.Archive(learning.Hash, "test")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Verify it's NOT in LEARNINGS.md anymore (not in provisional, not in archived section)
	contentAfter, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("failed to read LEARNINGS.md: %v", err)
	}

	if strings.Contains(string(contentAfter), "Test learning content") {
		t.Error("archived learning should be removed from LEARNINGS.md")
	}
}

// TestArchiveUpdatesArchivedHashesInState tests that Archive() adds hash to state's archived_hashes
func TestArchiveUpdatesArchivedHashesInState(t *testing.T) {
	// Expected failure: Archive() does not update state's archived_hashes field yet
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// We need to inject a state file for this test
	// The Archive() method should call state.AddArchivedHashes([learning.Hash])
	// This test verifies the behavior exists, but we'll check via GetArchivedHashes()
	// since state integration happens at a higher level

	learning, err := f.Add("bead-state", "State tracking test", CategoryPatterns)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if learning == nil {
		t.Fatal("learning should not be nil")
	}

	// Before archiving, the hash should not be in archivedHashes
	if f.GetArchivedHashes()[learning.Hash] {
		t.Fatal("hash should not be in archived hashes before Archive() call")
	}

	// Archive the learning
	err = f.Archive(learning.Hash, "test")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// After archiving, the hash should be in the archivedHashes set
	archivedHashes := f.GetArchivedHashes()
	if !archivedHashes[learning.Hash] {
		t.Error("archived learning's hash should be added to archivedHashes after Archive() call")
	}
}

// TestAddRejectsArchivedDuplicateViaState tests that Add() checks archived hashes from state
func TestAddRejectsArchivedDuplicateViaState(t *testing.T) {
	// Expected failure: Add() does not check archivedHashes field yet
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	content := "Previously archived learning"
	hash := hashContent(content)

	// Simulate that this hash was previously archived (set in archivedHashes)
	f.SetArchivedHashes([]string{hash})

	// Try to add a learning with the same content
	learning, err := f.Add("bead-new", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	// Should be rejected as duplicate
	if learning != nil {
		t.Error("Add() should return nil when hash matches an archived hash in state")
	}

	// Should not add to any section
	if len(f.provisional) != 0 {
		t.Errorf("expected 0 provisional, got %d", len(f.provisional))
	}
	if len(f.confirmed) != 0 {
		t.Errorf("expected 0 confirmed, got %d", len(f.confirmed))
	}
}

// TestSaveOmitsArchivedSection tests that Save() does not write ## Archived section
func TestSaveOmitsArchivedSection(t *testing.T) {
	// Expected failure: Save() still writes ## Archived section when f.archived is not empty
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add some learnings
	f.Add("bead-1", "Confirmed learning", CategoryPatterns)
	f.Add("bead-2", "Provisional learning", CategoryConventions)

	// Save the file
	err := f.Save()
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

	// Should contain ## Confirmed and ## Provisional
	if !strings.Contains(contentStr, "## Confirmed") {
		t.Error("LEARNINGS.md should contain ## Confirmed section")
	}
	if !strings.Contains(contentStr, "## Provisional") {
		t.Error("LEARNINGS.md should contain ## Provisional section")
	}

	// Should NOT contain ## Archived
	if strings.Contains(contentStr, "## Archived") {
		t.Error("LEARNINGS.md should not contain ## Archived section after Save()")
	}
}

// TestLoadIgnoresArchivedSection tests that Load() does not populate f.archived from ## Archived section
func TestLoadIgnoresArchivedSection(t *testing.T) {
	// Expected failure: Load() still parses ## Archived section into f.archived slice
	tmpDir := t.TempDir()

	// Create a LEARNINGS.md file with an ## Archived section
	learningsContent := `# Learnings

---

## Confirmed

### 2026-02-01 | bead-1 | patterns
Confirmed learning

---

## Provisional

### 2026-02-02 | bead-2 | conventions
Provisional learning

---

## Archived

### 2026-01-15 | bead-archived | gotchas
Old archived learning

*Archived from provisional: outdated*
`
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	err := os.WriteFile(learningsPath, []byte(learningsContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Load the file
	f, _ := NewFile(tmpDir)
	err = f.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	// f.archived should be empty (not populated from ## Archived section)
	if len(f.archived) != 0 {
		t.Errorf("expected Load() to ignore ## Archived section, but f.archived has %d entries", len(f.archived))
	}

	// Confirmed and provisional should still be loaded
	if len(f.confirmed) != 1 {
		t.Errorf("expected 1 confirmed, got %d", len(f.confirmed))
	}
	if len(f.provisional) != 1 {
		t.Errorf("expected 1 provisional, got %d", len(f.provisional))
	}
}

// TestMigrationMovesArchivedSectionToArchiveFile tests one-time migration during Load()
func TestMigrationMovesArchivedSectionToArchiveFile(t *testing.T) {
	// Expected failure: Load() does not trigger migration when ## Archived section is found
	tmpDir := t.TempDir()

	// Create a LEARNINGS.md with archived entries
	learningsContent := `# Learnings

---

## Confirmed

### 2026-02-01 | bead-1 | patterns
Confirmed learning

---

## Provisional

*No provisional learnings.*

---

## Archived

### 2026-01-10 | bead-old-1 | conventions
First archived learning

*Archived from provisional: reason 1*

### 2026-01-12 | bead-old-2 | gotchas
Second archived learning

*Archived from confirmed: reason 2*
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

	// LEARNINGS_ARCHIVE.md should now exist with the archived entries
	archivePath := filepath.Join(tmpDir, "LEARNINGS_ARCHIVE.md")
	archiveContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("expected LEARNINGS_ARCHIVE.md to be created during migration: %v", err)
	}

	archiveStr := string(archiveContent)
	if !strings.Contains(archiveStr, "First archived learning") {
		t.Error("archive file should contain first archived learning after migration")
	}
	if !strings.Contains(archiveStr, "Second archived learning") {
		t.Error("archive file should contain second archived learning after migration")
	}
	if !strings.Contains(archiveStr, "bead-old-1") {
		t.Error("archive file should contain first bead ID")
	}
	if !strings.Contains(archiveStr, "bead-old-2") {
		t.Error("archive file should contain second bead ID")
	}

	// LEARNINGS.md should no longer contain ## Archived section
	learningsContentAfter, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("failed to read LEARNINGS.md after migration: %v", err)
	}

	if strings.Contains(string(learningsContentAfter), "## Archived") {
		t.Error("LEARNINGS.md should not contain ## Archived section after migration")
	}

	// Archived hashes should be in archivedHashes field
	hash1 := hashContent("First archived learning\n\n*Archived from provisional: reason 1*")
	hash2 := hashContent("Second archived learning\n\n*Archived from confirmed: reason 2*")

	archivedHashes := f.GetArchivedHashes()
	if !archivedHashes[hash1] {
		t.Error("first archived learning's hash should be in archivedHashes after migration")
	}
	if !archivedHashes[hash2] {
		t.Error("second archived learning's hash should be in archivedHashes after migration")
	}
}

// TestMigrationIsIdempotent tests that running migration twice does not duplicate entries
func TestMigrationIsIdempotent(t *testing.T) {
	// Expected failure: migration logic does not exist yet
	tmpDir := t.TempDir()

	// Create LEARNINGS.md with archived section
	learningsContent := `# Learnings

---

## Confirmed

*No confirmed learnings yet.*

---

## Provisional

*No provisional learnings.*

---

## Archived

### 2026-01-10 | bead-test | patterns
Test archived learning
`
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	err := os.WriteFile(learningsPath, []byte(learningsContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// First load - triggers migration
	f1, _ := NewFile(tmpDir)
	err = f1.Load()
	if err != nil {
		t.Fatalf("first load failed: %v", err)
	}

	// Read archive file content after first migration
	archivePath := filepath.Join(tmpDir, "LEARNINGS_ARCHIVE.md")
	archiveAfterFirst, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("archive file should exist after first load: %v", err)
	}

	// Count occurrences of the bead ID in archive file
	firstCount := strings.Count(string(archiveAfterFirst), "bead-test")
	if firstCount != 1 {
		t.Fatalf("expected 1 occurrence of bead-test after first migration, got %d", firstCount)
	}

	// Second load - should not duplicate entries
	f2, _ := NewFile(tmpDir)
	err = f2.Load()
	if err != nil {
		t.Fatalf("second load failed: %v", err)
	}

	// Read archive file again
	archiveAfterSecond, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("failed to read archive after second load: %v", err)
	}

	// Should still have only 1 occurrence (no duplication)
	secondCount := strings.Count(string(archiveAfterSecond), "bead-test")
	if secondCount != 1 {
		t.Errorf("expected 1 occurrence of bead-test after second migration, got %d (migration should be idempotent)", secondCount)
	}
}

// TestAddFilteredGenericUsesArchiveFile tests that Add() with filter puts generic learnings in archive file
func TestAddFilteredGenericUsesArchiveFile(t *testing.T) {
	// Expected failure: Add() still writes filtered-generic learnings to f.archived in-memory, not to archive file
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Set filter that classifies as generic
	f.SetFilter(func(content string) (bool, error) {
		return true, nil // Generic
	})

	learning, err := f.Add("bead-generic", "Always write tests", CategoryPatterns)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	// Should return nil (filtered as generic)
	if learning != nil {
		t.Error("filtered generic learning should return nil")
	}

	// Should be in archive file, not LEARNINGS.md
	archivePath := filepath.Join(tmpDir, "LEARNINGS_ARCHIVE.md")
	archiveContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("expected archive file to exist after adding filtered generic learning: %v", err)
	}

	archiveStr := string(archiveContent)
	if !strings.Contains(archiveStr, "Always write tests") {
		t.Error("filtered generic learning should be in archive file")
	}
	if !strings.Contains(archiveStr, "filtered: generic engineering advice") {
		t.Error("filtered generic learning should have filter reason in archive file")
	}

	// Should NOT be in LEARNINGS.md
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	learningsContent, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("failed to read LEARNINGS.md: %v", err)
	}

	if strings.Contains(string(learningsContent), "Always write tests") {
		t.Error("filtered generic learning should not be in LEARNINGS.md")
	}

	// Hash should be in archivedHashes
	hash := hashContent("Always write tests\n\n*Archived from new: filtered: generic engineering advice*")
	if !f.GetArchivedHashes()[hash] {
		t.Error("filtered generic learning's hash should be in archivedHashes")
	}
}

// TestHashExistsChecksArchivedHashes tests that hashExists() checks archivedHashes map
func TestHashExistsChecksArchivedHashes(t *testing.T) {
	// Expected failure: hashExists() does not check archivedHashes field yet
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	testContent := "Test learning content"
	testHash := hashContent(testContent)

	// Add hash to archivedHashes
	f.SetArchivedHashes([]string{testHash})

	// hashExists should return true
	if !f.hashExists(testHash) {
		t.Error("hashExists() should return true for hash in archivedHashes")
	}

	// Different hash should return false
	differentHash := hashContent("Different content")
	if f.hashExists(differentHash) {
		t.Error("hashExists() should return false for hash not in any section")
	}
}

// TestArchiveFilePreservesFormatting tests that archive file uses same format as old ## Archived section
func TestArchiveFilePreservesFormatting(t *testing.T) {
	// Expected failure: appendToArchiveFile helper does not exist yet
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Create a learning with specific properties
	learning := Learning{
		Date:     time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC),
		BeadID:   "bead-format",
		Content:  "Test content\n\n*Archived from provisional: test reason*",
		Category: CategoryPatterns,
		Hash:     hashContent("Test content\n\n*Archived from provisional: test reason*"),
	}

	// Append to archive file
	err := f.appendToArchiveFile(learning)
	if err != nil {
		t.Fatalf("appendToArchiveFile failed: %v", err)
	}

	// Read archive file and verify format
	archivePath := filepath.Join(tmpDir, "LEARNINGS_ARCHIVE.md")
	content, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("failed to read archive file: %v", err)
	}

	contentStr := string(content)

	// Should have ### header with pipe-delimited format
	expectedHeader := "### 2026-02-10 | bead-format | patterns"
	if !strings.Contains(contentStr, expectedHeader) {
		t.Errorf("archive file should contain header %q", expectedHeader)
	}

	// Should contain the content
	if !strings.Contains(contentStr, "Test content") {
		t.Error("archive file should contain learning content")
	}

	// Should preserve the archival reason
	if !strings.Contains(contentStr, "*Archived from provisional: test reason*") {
		t.Error("archive file should preserve archival reason annotation")
	}
}
