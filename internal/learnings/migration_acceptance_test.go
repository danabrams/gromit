//go:build acceptance

package learnings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/state"
)

// TestMigrateArchivedWritesArchiveFileAndUpdatesState tests that migrateArchived()
// writes archived entries to LEARNINGS_ARCHIVE.md and updates state.json with hashes
func TestMigrateArchivedWritesArchiveFileAndUpdatesState(t *testing.T) {
	// Expected failure: migrateArchived() method does not exist yet
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)

	f, _ := NewFile(gromitDir)

	// Manually populate f.archived to simulate parsed archived section
	f.archived = []Learning{
		{
			BeadID:   "bead-old-1",
			Content:  "Old archived learning one\n\n*Archived from provisional: outdated*",
			Category: CategoryConventions,
			Hash:     hashContent("Old archived learning one"),
		},
		{
			BeadID:   "bead-old-2",
			Content:  "Old archived learning two\n\n*Archived from confirmed: superseded*",
			Category: CategoryGotchas,
			Hash:     hashContent("Old archived learning two"),
		},
	}

	// Create state file
	stateFile, _ := state.NewFile(gromitDir)
	err := stateFile.Load()
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	// Call migrateArchived - this method doesn't exist yet
	err = f.migrateArchived(stateFile)
	if err != nil {
		t.Fatalf("migrateArchived failed: %v", err)
	}

	// Verify LEARNINGS_ARCHIVE.md exists and contains archived entries
	archivePath := filepath.Join(gromitDir, "LEARNINGS_ARCHIVE.md")
	archiveContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("expected LEARNINGS_ARCHIVE.md to exist: %v", err)
	}

	archiveStr := string(archiveContent)
	if !strings.Contains(archiveStr, "Old archived learning one") {
		t.Error("archive file should contain first migrated entry")
	}
	if !strings.Contains(archiveStr, "Old archived learning two") {
		t.Error("archive file should contain second migrated entry")
	}

	// Verify state.json contains archived hashes
	archivedHashes := stateFile.GetArchivedHashes()
	hash1 := hashContent("Old archived learning one")
	hash2 := hashContent("Old archived learning two")

	if !archivedHashes[hash1] {
		t.Error("state should contain hash for first archived learning")
	}
	if !archivedHashes[hash2] {
		t.Error("state should contain hash for second archived learning")
	}

	// Verify f.archived slice is cleared after migration
	if len(f.archived) != 0 {
		t.Errorf("expected f.archived to be cleared after migration, got %d entries", len(f.archived))
	}
}

// TestMigrateArchivedIsIdempotent tests that calling migrateArchived multiple times
// doesn't create duplicate entries in the archive file
func TestMigrateArchivedIsIdempotent(t *testing.T) {
	// Expected failure: migrateArchived() method does not exist yet
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)

	f, _ := NewFile(gromitDir)
	stateFile, _ := state.NewFile(gromitDir)
	stateFile.Load()

	// Populate f.archived
	f.archived = []Learning{
		{
			BeadID:   "bead-1",
			Content:  "Archived learning",
			Category: CategoryPatterns,
			Hash:     hashContent("Archived learning"),
		},
	}

	// First migration
	err := f.migrateArchived(stateFile)
	if err != nil {
		t.Fatalf("first migration failed: %v", err)
	}

	// Simulate loading again with archived entries (shouldn't happen, but test idempotency)
	f.archived = []Learning{
		{
			BeadID:   "bead-1",
			Content:  "Archived learning",
			Category: CategoryPatterns,
			Hash:     hashContent("Archived learning"),
		},
	}

	// Second migration - should detect duplicate by hash
	err = f.migrateArchived(stateFile)
	if err != nil {
		t.Fatalf("second migration failed: %v", err)
	}

	// Verify archive file contains entry only once
	archivePath := filepath.Join(gromitDir, "LEARNINGS_ARCHIVE.md")
	archiveContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("failed to read archive file: %v", err)
	}

	// Count occurrences of "Archived learning"
	archiveStr := string(archiveContent)
	count := strings.Count(archiveStr, "Archived learning")
	if count != 1 {
		t.Errorf("expected 1 occurrence of learning in archive file, got %d", count)
	}
}

// TestLoadDetectsArchivedSectionAndTriggersMigration tests that Load() detects
// a non-empty archived slice after parsing and calls migrateArchived
func TestLoadDetectsArchivedSectionAndTriggersMigration(t *testing.T) {
	// Expected failure: Load() does not implement migration trigger logic yet
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)

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

### 2026-01-10 | bead-old | patterns
Archived learning content

*Archived from provisional: outdated*
`
	learningsPath := filepath.Join(gromitDir, "LEARNINGS.md")
	err := os.WriteFile(learningsPath, []byte(learningsContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create state file
	stateFile, _ := state.NewFile(gromitDir)
	stateFile.Load()
	stateFile.Save()

	// Load should trigger migration
	f, _ := NewFile(gromitDir)
	err = f.LoadWithState(stateFile)
	if err != nil {
		t.Fatalf("LoadWithState failed: %v", err)
	}

	// Verify archive file was created
	archivePath := filepath.Join(gromitDir, "LEARNINGS_ARCHIVE.md")
	_, err = os.Stat(archivePath)
	if err != nil {
		t.Errorf("expected LEARNINGS_ARCHIVE.md to exist after migration: %v", err)
	}

	// Verify state contains archived hash
	archivedHashes := stateFile.GetArchivedHashes()
	hash := hashContent("Archived learning content")
	if !archivedHashes[hash] {
		t.Error("state should contain hash for migrated archived learning")
	}

	// Verify LEARNINGS.md no longer has archived section
	learningsContentAfter, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("failed to read LEARNINGS.md after migration: %v", err)
	}

	learningsStr := string(learningsContentAfter)
	if strings.Contains(learningsStr, "## Archived") {
		t.Error("LEARNINGS.md should not contain ## Archived section after migration")
	}
}

// TestFilterProvisionalWritesToArchiveFile tests that FilterProvisional() writes
// generic learnings to LEARNINGS_ARCHIVE.md and updates state, not f.archived
func TestFilterProvisionalWritesToArchiveFile(t *testing.T) {
	// Expected failure: FilterProvisional() currently moves to f.archived in-memory slice
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)

	f, _ := NewFile(gromitDir)
	stateFile, _ := state.NewFile(gromitDir)
	stateFile.Load()

	// Add provisional learnings
	f.provisional = []Learning{
		{
			BeadID:   "bead-1",
			Content:  "Always test",
			Category: CategoryPatterns,
			Hash:     hashContent("Always test"),
		},
		{
			BeadID:   "bead-2",
			Content:  "Use DRY",
			Category: CategoryPatterns,
			Hash:     hashContent("Use DRY"),
		},
	}

	// Filter that marks all as generic
	filter := func(content string) (bool, error) {
		return true, nil
	}

	// Run filter with state
	_, err := f.FilterProvisionalWithState(filter, stateFile.GetFilteredHashes(), stateFile)
	if err != nil {
		t.Fatalf("FilterProvisionalWithState failed: %v", err)
	}

	// Verify LEARNINGS_ARCHIVE.md contains filtered learnings
	archivePath := filepath.Join(gromitDir, "LEARNINGS_ARCHIVE.md")
	archiveContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("expected LEARNINGS_ARCHIVE.md to exist: %v", err)
	}

	archiveStr := string(archiveContent)
	if !strings.Contains(archiveStr, "Always test") {
		t.Error("archive file should contain first filtered learning")
	}
	if !strings.Contains(archiveStr, "Use DRY") {
		t.Error("archive file should contain second filtered learning")
	}
	if !strings.Contains(archiveStr, "filtered: generic engineering advice") {
		t.Error("archive file should contain filter reason")
	}

	// Verify state contains archived hashes
	archivedHashes := stateFile.GetArchivedHashes()
	hash1 := hashContent("Always test")
	hash2 := hashContent("Use DRY")

	if !archivedHashes[hash1] {
		t.Error("state should contain hash for first filtered learning")
	}
	if !archivedHashes[hash2] {
		t.Error("state should contain hash for second filtered learning")
	}

	// Verify f.archived is empty (not used anymore)
	if len(f.archived) != 0 {
		t.Errorf("expected f.archived to be empty (learnings go to file), got %d", len(f.archived))
	}

	// Verify f.provisional is empty
	if len(f.provisional) != 0 {
		t.Errorf("expected f.provisional to be empty after filtering, got %d", len(f.provisional))
	}
}

// TestAddChecksArchivedHashesFromState tests that Add() checks archived hashes
// from state.json, not just in-memory f.archived slice
func TestAddChecksArchivedHashesFromState(t *testing.T) {
	// Expected failure: Add() currently only checks f.archived slice, not state hashes
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)

	// Create state with archived hash
	stateFile, _ := state.NewFile(gromitDir)
	stateFile.Load()

	content := "Previously archived content"
	hash := hashContent(content)
	stateFile.AddArchivedHashes([]string{hash})
	stateFile.Save()

	// Create learnings file
	f, _ := NewFile(gromitDir)
	f.Load()

	// Try to add the same content (hash matches archived hash in state)
	learning, err := f.AddWithState("bead-new", content, CategoryPatterns, stateFile)
	if err != nil {
		t.Fatalf("AddWithState failed: %v", err)
	}

	// Should be rejected as duplicate
	if learning != nil {
		t.Error("AddWithState should return nil for content matching archived hash in state")
	}

	// Should not be added to any section
	if len(f.provisional) != 0 {
		t.Errorf("expected 0 provisional, got %d", len(f.provisional))
	}
	if len(f.confirmed) != 0 {
		t.Errorf("expected 0 confirmed, got %d", len(f.confirmed))
	}
}

// TestAppendToArchiveFileCreatesFileIfNotExists tests that appendToArchiveFile
// creates LEARNINGS_ARCHIVE.md if it doesn't exist
func TestAppendToArchiveFileCreatesFileIfNotExists(t *testing.T) {
	// Expected failure: appendToArchiveFile() method does not exist yet
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)

	f, _ := NewFile(gromitDir)

	// Create a learning to append
	learning := Learning{
		BeadID:   "bead-1",
		Content:  "First archived entry",
		Category: CategoryPatterns,
		Hash:     hashContent("First archived entry"),
	}

	// Call appendToArchiveFile - this method doesn't exist yet
	err := f.appendToArchiveFile(learning)
	if err != nil {
		t.Fatalf("appendToArchiveFile failed: %v", err)
	}

	// Verify LEARNINGS_ARCHIVE.md was created
	archivePath := filepath.Join(gromitDir, "LEARNINGS_ARCHIVE.md")
	archiveContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("expected LEARNINGS_ARCHIVE.md to be created: %v", err)
	}

	archiveStr := string(archiveContent)
	if !strings.Contains(archiveStr, "First archived entry") {
		t.Error("archive file should contain the appended learning")
	}
	if !strings.Contains(archiveStr, "bead-1") {
		t.Error("archive file should contain the bead ID")
	}
}

// TestAppendToArchiveFileAppendsToExistingFile tests that appendToArchiveFile
// appends to an existing LEARNINGS_ARCHIVE.md without overwriting
func TestAppendToArchiveFileAppendsToExistingFile(t *testing.T) {
	// Expected failure: appendToArchiveFile() method does not exist yet
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)

	// Create existing archive file
	archivePath := filepath.Join(gromitDir, "LEARNINGS_ARCHIVE.md")
	existingContent := `# Archived Learnings

### 2026-01-01 | bead-old | patterns
Old archived entry
`
	err := os.WriteFile(archivePath, []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("failed to create existing archive file: %v", err)
	}

	f, _ := NewFile(gromitDir)

	// Create a new learning to append
	learning := Learning{
		BeadID:   "bead-new",
		Content:  "New archived entry",
		Category: CategoryConventions,
		Hash:     hashContent("New archived entry"),
	}

	// Call appendToArchiveFile
	err = f.appendToArchiveFile(learning)
	if err != nil {
		t.Fatalf("appendToArchiveFile failed: %v", err)
	}

	// Verify both old and new entries exist
	archiveContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("failed to read archive file: %v", err)
	}

	archiveStr := string(archiveContent)
	if !strings.Contains(archiveStr, "Old archived entry") {
		t.Error("archive file should still contain old entry")
	}
	if !strings.Contains(archiveStr, "New archived entry") {
		t.Error("archive file should contain new entry")
	}
	if !strings.Contains(archiveStr, "bead-old") {
		t.Error("archive file should still contain old bead ID")
	}
	if !strings.Contains(archiveStr, "bead-new") {
		t.Error("archive file should contain new bead ID")
	}
}

// TestArchiveUpdatesStateArchivedHashes tests that Archive() adds the archived
// learning's hash to state.json
func TestArchiveUpdatesStateArchivedHashes(t *testing.T) {
	// Expected failure: Archive() currently only updates f.archived, not state
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)

	f, _ := NewFile(gromitDir)
	stateFile, _ := state.NewFile(gromitDir)
	stateFile.Load()

	// Add a learning
	content := "Learning to archive"
	learning, err := f.Add("bead-1", content, CategoryPatterns)
	if err != nil || learning == nil {
		t.Fatalf("failed to add learning: %v", err)
	}

	// Archive it with state
	err = f.ArchiveWithState(learning.Hash, "test reason", stateFile)
	if err != nil {
		t.Fatalf("ArchiveWithState failed: %v", err)
	}

	// Verify state contains the archived hash
	archivedHashes := stateFile.GetArchivedHashes()
	if !archivedHashes[learning.Hash] {
		t.Error("state should contain archived learning hash after Archive()")
	}

	// Verify LEARNINGS_ARCHIVE.md contains the learning
	archivePath := filepath.Join(gromitDir, "LEARNINGS_ARCHIVE.md")
	archiveContent, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("expected LEARNINGS_ARCHIVE.md to exist: %v", err)
	}

	archiveStr := string(archiveContent)
	if !strings.Contains(archiveStr, content) {
		t.Error("archive file should contain archived learning")
	}
}

// TestMigrationClearsArchivedSliceAndResaves tests that after migration,
// the f.archived slice is cleared and LEARNINGS.md is re-saved
func TestMigrationClearsArchivedSliceAndResaves(t *testing.T) {
	// Expected failure: Load()/migrateArchived() doesn't clear f.archived and resave
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)

	// Create LEARNINGS.md with archived section
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

### 2026-01-10 | bead-old | conventions
Archived learning
`
	learningsPath := filepath.Join(gromitDir, "LEARNINGS.md")
	err := os.WriteFile(learningsPath, []byte(learningsContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create state file
	stateFile, _ := state.NewFile(gromitDir)
	stateFile.Load()

	// Load with state (should trigger migration)
	f, _ := NewFile(gromitDir)
	err = f.LoadWithState(stateFile)
	if err != nil {
		t.Fatalf("LoadWithState failed: %v", err)
	}

	// Verify f.archived is cleared
	if len(f.archived) != 0 {
		t.Errorf("expected f.archived to be cleared after migration, got %d entries", len(f.archived))
	}

	// Reload LEARNINGS.md to verify it was resaved without archived section
	learningsContentAfter, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("failed to read LEARNINGS.md: %v", err)
	}

	learningsStr := string(learningsContentAfter)
	if strings.Contains(learningsStr, "## Archived") {
		t.Error("LEARNINGS.md should not contain ## Archived section after migration")
	}
	if strings.Contains(learningsStr, "Archived learning") {
		t.Error("LEARNINGS.md should not contain archived learning content after migration")
	}

	// Confirmed learning should still be present
	if !strings.Contains(learningsStr, "Confirmed learning") {
		t.Error("LEARNINGS.md should still contain confirmed learning after migration")
	}
}
