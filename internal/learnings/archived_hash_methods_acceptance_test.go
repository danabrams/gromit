//go:build acceptance

package learnings

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSetArchivedHashesEnablesExternalDedupChecks verifies that SetArchivedHashes()
// allows external callers to pass in archived hashes from state.json, and these
// hashes are then used in duplicate detection during Add().
func TestSetArchivedHashesEnablesExternalDedupChecks(t *testing.T) {
	// Expected failure: SetArchivedHashes() method does not exist on learnings.File
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Simulate archived hashes from state.json
	archivedHashes := map[string]bool{
		"archived_hash_alpha": true,
		"archived_hash_beta":  true,
	}

	// Set the archived hashes from state
	f.SetArchivedHashes(archivedHashes)

	// Try to add a learning with content that matches an archived hash
	// (We'll use the actual hash computation to ensure it matches)
	content := "Some archived content"
	expectedHash := hashContent(content)

	// Manually add to the archived hashes map to simulate it came from state
	archivedHashes[expectedHash] = true
	f.SetArchivedHashes(archivedHashes)

	// Try to add the learning - should be rejected because hash is in archived set
	result, err := f.Add("bead-test", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	// Expected: result should be nil (duplicate rejected)
	if result != nil {
		t.Error("Add() should return nil when content hash matches an archived hash from state")
	}

	// Verify nothing was added to provisional or confirmed
	if len(f.provisional) != 0 {
		t.Errorf("expected 0 provisional, got %d", len(f.provisional))
	}
	if len(f.confirmed) != 0 {
		t.Errorf("expected 0 confirmed, got %d", len(f.confirmed))
	}
}

// TestGetArchivedHashesReturnsAllArchivedHashes verifies that GetArchivedHashes()
// returns all hashes from learnings that have been archived, for persistence to state.
func TestGetArchivedHashesReturnsAllArchivedHashes(t *testing.T) {
	// Expected failure: GetArchivedHashes() method does not exist on learnings.File
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add some learnings
	learning1, _ := f.Add("bead-1", "First learning", CategoryPatterns)
	learning2, _ := f.Add("bead-2", "Second learning", CategoryConventions)
	learning3, _ := f.Add("bead-3", "Third learning", CategoryGotchas)

	if learning1 == nil || learning2 == nil || learning3 == nil {
		t.Fatal("learnings should not be nil")
	}

	// Archive two of them
	_ = f.Archive(learning1.Hash, "archived first")
	_ = f.Archive(learning2.Hash, "archived second")

	// Get archived hashes
	archivedHashes := f.GetArchivedHashes()

	// Should return a map with the two archived hashes
	if len(archivedHashes) != 2 {
		t.Errorf("expected 2 archived hashes, got %d", len(archivedHashes))
	}

	if !archivedHashes[learning1.Hash] {
		t.Errorf("archived hashes should contain hash %s", learning1.Hash)
	}
	if !archivedHashes[learning2.Hash] {
		t.Errorf("archived hashes should contain hash %s", learning2.Hash)
	}
	if archivedHashes[learning3.Hash] {
		t.Error("archived hashes should not contain non-archived learning hash")
	}
}

// TestSetArchivedHashesMergesWithExistingArchived verifies that SetArchivedHashes()
// works correctly alongside existing archived learnings loaded from LEARNINGS.md.
func TestSetArchivedHashesMergesWithExistingArchived(t *testing.T) {
	// Expected failure: SetArchivedHashes() method does not exist on learnings.File
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)

	// Create a learnings file with archived content
	f, _ := NewFile(gromitDir)
	learning1, _ := f.Add("bead-1", "Learning to archive", CategoryPatterns)
	if learning1 == nil {
		t.Fatal("learning should not be nil")
	}
	_ = f.Archive(learning1.Hash, "test archive")

	// Reload the learnings file (simulating a fresh load from disk)
	f2, _ := NewFile(gromitDir)
	err := f2.Load()
	if err != nil {
		t.Fatalf("failed to load learnings: %v", err)
	}

	// Verify the archived learning was loaded
	if len(f2.archived) != 1 {
		t.Fatalf("expected 1 archived learning after load, got %d", len(f2.archived))
	}

	// Set additional archived hashes from state (simulating state.json)
	externalHashes := map[string]bool{
		"external_hash_001": true,
		"external_hash_002": true,
	}
	f2.SetArchivedHashes(externalHashes)

	// Try to add content matching each hash - all should be rejected
	duplicate1, _ := f2.Add("bead-dup1", "Learning to archive", CategoryPatterns)
	if duplicate1 != nil {
		t.Error("should reject duplicate matching archived learning from file")
	}

	// Try adding with a hash that matches external (we need to use actual content
	// that hashes to the external values - for simplicity, just verify the external
	// hashes are in the set)
	allHashes := f2.GetArchivedHashes()
	if !allHashes[learning1.Hash] {
		t.Error("GetArchivedHashes should include hashes from archived learnings")
	}
	if !allHashes["external_hash_001"] {
		t.Error("GetArchivedHashes should include externally set hashes")
	}
	if !allHashes["external_hash_002"] {
		t.Error("GetArchivedHashes should include externally set hashes")
	}
}

// TestSetArchivedHashesWorksWithEmptyFile verifies that SetArchivedHashes()
// works correctly on a fresh learnings file with no existing content.
func TestSetArchivedHashesWorksWithEmptyFile(t *testing.T) {
	// Expected failure: SetArchivedHashes() method does not exist on learnings.File
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Set archived hashes on a fresh file
	archivedHashes := map[string]bool{
		"hash_001": true,
		"hash_002": true,
		"hash_003": true,
	}
	f.SetArchivedHashes(archivedHashes)

	// Get the hashes back
	retrieved := f.GetArchivedHashes()

	if len(retrieved) != 3 {
		t.Errorf("expected 3 archived hashes, got %d", len(retrieved))
	}

	for hash := range archivedHashes {
		if !retrieved[hash] {
			t.Errorf("retrieved hashes should contain %s", hash)
		}
	}
}

// TestGetArchivedHashesReturnsEmptyMapWhenNoArchives verifies that
// GetArchivedHashes() returns an empty map (not nil) when there are no archived hashes.
func TestGetArchivedHashesReturnsEmptyMapWhenNoArchives(t *testing.T) {
	// Expected failure: GetArchivedHashes() method does not exist on learnings.File
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add a provisional learning (not archived)
	_, _ = f.Add("bead-1", "Provisional learning", CategoryPatterns)

	// Get archived hashes - should be empty but not nil
	archivedHashes := f.GetArchivedHashes()

	if archivedHashes == nil {
		t.Error("GetArchivedHashes should return non-nil map even when empty")
	}

	if len(archivedHashes) != 0 {
		t.Errorf("expected 0 archived hashes, got %d", len(archivedHashes))
	}
}

// TestSetArchivedHashesIntegrationWithHashExists verifies that SetArchivedHashes()
// properly integrates with the existing hashExists() duplicate detection logic.
func TestSetArchivedHashesIntegrationWithHashExists(t *testing.T) {
	// Expected failure: SetArchivedHashes() method does not exist, so external
	// archived hashes cannot be integrated into duplicate detection
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add and confirm a learning
	learning1, _ := f.Add("bead-1", "Confirmed learning", CategoryPatterns)
	if learning1 == nil {
		t.Fatal("learning should not be nil")
	}

	// Add a provisional learning
	learning2, _ := f.Add("bead-2", "Provisional learning", CategoryConventions)
	if learning2 == nil {
		t.Fatal("learning should not be nil")
	}

	// Set external archived hashes
	externalHashes := map[string]bool{
		"external_archived_hash": true,
	}
	f.SetArchivedHashes(externalHashes)

	// Create content that will hash to a specific value for testing
	testContent := "Test content for hashing"
	testHash := hashContent(testContent)

	// Add test hash to external archived hashes
	externalHashes[testHash] = true
	f.SetArchivedHashes(externalHashes)

	// Try to add content with that hash - should be rejected
	duplicate, err := f.Add("bead-dup", testContent, CategoryPatterns)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	if duplicate != nil {
		t.Error("Add() should reject content when hash matches external archived hash")
	}

	// Verify the counts haven't changed
	confirmed, provisional := f.Stats()
	if confirmed != 1 {
		t.Errorf("expected 1 confirmed, got %d", confirmed)
	}
	if provisional != 1 {
		t.Errorf("expected 1 provisional, got %d", provisional)
	}
}

// TestArchivedHashesPersistedAfterFiltering verifies that when provisional
// learnings are filtered and archived, their hashes appear in GetArchivedHashes()
// for subsequent persistence to state.json.
func TestArchivedHashesPersistedAfterFiltering(t *testing.T) {
	// Expected failure: GetArchivedHashes() method does not exist, so filtered
	// and archived learnings cannot be retrieved for persistence
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add a provisional learning
	learning, err := f.Add("bead-1", "Always write unit tests", CategoryPatterns)
	if err != nil {
		t.Fatalf("failed to add learning: %v", err)
	}
	if learning == nil {
		t.Fatal("learning should not be nil")
	}

	// Apply filtering that marks it as generic
	filterFn := func(content string) (bool, error) {
		return true, nil // Mark as generic
	}

	alreadyFiltered := make(map[string]bool)
	_, err = f.FilterProvisional(filterFn, alreadyFiltered)
	if err != nil {
		t.Fatalf("filter failed: %v", err)
	}

	// After filtering, the learning should be archived
	if len(f.archived) == 0 {
		t.Fatal("filtered learning should be archived")
	}

	// Get archived hashes - should include the filtered learning's hash
	archivedHashes := f.GetArchivedHashes()
	if !archivedHashes[learning.Hash] {
		t.Error("archived hashes should contain filtered learning's hash")
	}
}
