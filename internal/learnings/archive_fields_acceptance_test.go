//go:build acceptance

package learnings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewFileInitializesArchivePath tests that NewFile sets archivePath to LEARNINGS_ARCHIVE.md
func TestNewFileInitializesArchivePath(t *testing.T) {
	// Expected failure: archivePath field does not exist on File yet
	tmpDir := t.TempDir()
	f, err := NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "LEARNINGS_ARCHIVE.md")
	if f.archivePath != expectedPath {
		t.Errorf("expected archivePath to be %q, got %q", expectedPath, f.archivePath)
	}
}

// TestSetArchivedHashes tests that SetArchivedHashes stores hashes and GetArchivedHashes retrieves them
func TestSetArchivedHashes(t *testing.T) {
	// Expected failure: SetArchivedHashes method does not exist on File yet
	tmpDir := t.TempDir()
	f, err := NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	hashes := []string{"hash1", "hash2", "hash3"}
	f.SetArchivedHashes(hashes)

	retrievedHashes := f.GetArchivedHashes()
	if len(retrievedHashes) != len(hashes) {
		t.Errorf("expected %d hashes, got %d", len(hashes), len(retrievedHashes))
	}

	// Verify all hashes are present
	for _, hash := range hashes {
		if !retrievedHashes[hash] {
			t.Errorf("expected hash %q to be present in GetArchivedHashes result", hash)
		}
	}
}

// TestGetArchivedHashesReturnsEmptySetInitially tests that GetArchivedHashes returns empty set when no hashes set
func TestGetArchivedHashesReturnsEmptySetInitially(t *testing.T) {
	// Expected failure: GetArchivedHashes method does not exist on File yet
	tmpDir := t.TempDir()
	f, err := NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	hashes := f.GetArchivedHashes()
	if hashes == nil {
		t.Fatal("expected non-nil map from GetArchivedHashes")
	}
	if len(hashes) != 0 {
		t.Errorf("expected empty map initially, got %d entries", len(hashes))
	}
}

// TestGetArchivedHashesReturnsMapForO1Lookup tests that GetArchivedHashes returns a map (not slice)
func TestGetArchivedHashesReturnsMapForO1Lookup(t *testing.T) {
	// Expected failure: GetArchivedHashes method does not exist on File yet
	tmpDir := t.TempDir()
	f, err := NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	f.SetArchivedHashes([]string{"hash1", "hash2"})
	hashes := f.GetArchivedHashes()

	// Verify it's a map by checking type assertion capability and O(1) lookup behavior
	if hashes == nil {
		t.Fatal("expected non-nil map from GetArchivedHashes")
	}

	// Test O(1) lookup - should return true for present hash
	if !hashes["hash1"] {
		t.Error("expected hash1 to be present (true value)")
	}

	// Test O(1) lookup - should return false for absent hash
	if hashes["nonexistent"] {
		t.Error("expected nonexistent hash to be absent (false value)")
	}
}

// TestSetArchivedHashesOverwritesPreviousHashes tests that SetArchivedHashes replaces the hash set
func TestSetArchivedHashesOverwritesPreviousHashes(t *testing.T) {
	// Expected failure: SetArchivedHashes method does not exist on File yet
	tmpDir := t.TempDir()
	f, err := NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	// Set initial hashes
	f.SetArchivedHashes([]string{"hash1", "hash2"})

	// Overwrite with new set
	f.SetArchivedHashes([]string{"hash3", "hash4"})

	retrievedHashes := f.GetArchivedHashes()
	if len(retrievedHashes) != 2 {
		t.Errorf("expected 2 hashes after overwrite, got %d", len(retrievedHashes))
	}

	// Old hashes should be gone
	if retrievedHashes["hash1"] || retrievedHashes["hash2"] {
		t.Error("expected old hashes to be removed after SetArchivedHashes")
	}

	// New hashes should be present
	if !retrievedHashes["hash3"] || !retrievedHashes["hash4"] {
		t.Error("expected new hashes to be present after SetArchivedHashes")
	}
}

// TestAppendToArchiveFileCreatesFileIfMissing tests that appendToArchiveFile creates the archive file
func TestAppendToArchiveFileCreatesFileIfMissing(t *testing.T) {
	// Expected failure: appendToArchiveFile helper does not exist yet
	tmpDir := t.TempDir()
	f, err := NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	testLearning := Learning{
		BeadID:   "bead-123",
		Content:  "Test archived learning",
		Category: CategoryPatterns,
	}

	// Archive file should not exist initially
	archivePath := filepath.Join(tmpDir, "LEARNINGS_ARCHIVE.md")
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatal("archive file should not exist initially")
	}

	// appendToArchiveFile should create the file
	err = f.appendToArchiveFile(testLearning)
	if err != nil {
		t.Fatalf("appendToArchiveFile failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Fatal("expected archive file to be created")
	}
}

// TestAppendToArchiveFileAppendsLearning tests that appendToArchiveFile appends learning content
func TestAppendToArchiveFileAppendsLearning(t *testing.T) {
	// Expected failure: appendToArchiveFile helper does not exist yet
	tmpDir := t.TempDir()
	f, err := NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	learning1 := Learning{
		BeadID:   "bead-1",
		Content:  "First archived learning",
		Category: CategoryPatterns,
	}
	learning2 := Learning{
		BeadID:   "bead-2",
		Content:  "Second archived learning",
		Category: CategoryConventions,
	}

	// Append first learning
	err = f.appendToArchiveFile(learning1)
	if err != nil {
		t.Fatalf("first append failed: %v", err)
	}

	// Append second learning
	err = f.appendToArchiveFile(learning2)
	if err != nil {
		t.Fatalf("second append failed: %v", err)
	}

	// Read archive file and verify both learnings are present
	archivePath := filepath.Join(tmpDir, "LEARNINGS_ARCHIVE.md")
	content, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("failed to read archive file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "First archived learning") {
		t.Error("expected archive file to contain first learning content")
	}
	if !strings.Contains(contentStr, "Second archived learning") {
		t.Error("expected archive file to contain second learning content")
	}
	if !strings.Contains(contentStr, "bead-1") {
		t.Error("expected archive file to contain first learning bead ID")
	}
	if !strings.Contains(contentStr, "bead-2") {
		t.Error("expected archive file to contain second learning bead ID")
	}
}

// TestAppendToArchiveFileUsesArchivePath tests that appendToArchiveFile writes to the archivePath field
func TestAppendToArchiveFileUsesArchivePath(t *testing.T) {
	// Expected failure: appendToArchiveFile helper does not exist yet, and archivePath field does not exist
	tmpDir := t.TempDir()
	f, err := NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	testLearning := Learning{
		BeadID:   "bead-test",
		Content:  "Test learning",
		Category: CategoryGotchas,
	}

	err = f.appendToArchiveFile(testLearning)
	if err != nil {
		t.Fatalf("appendToArchiveFile failed: %v", err)
	}

	// Verify file exists at the archivePath location
	expectedPath := filepath.Join(tmpDir, "LEARNINGS_ARCHIVE.md")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected archive file to exist at %q", expectedPath)
	}

	// Also verify via the archivePath field
	if _, err := os.Stat(f.archivePath); os.IsNotExist(err) {
		t.Errorf("expected archive file to exist at f.archivePath (%q)", f.archivePath)
	}
}

// TestAppendToArchiveFileFormatsLearningCorrectly tests that appendToArchiveFile uses correct format
func TestAppendToArchiveFileFormatsLearningCorrectly(t *testing.T) {
	// Expected failure: appendToArchiveFile helper does not exist yet
	tmpDir := t.TempDir()
	f, err := NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	testLearning := Learning{
		BeadID:   "bead-format-test",
		Content:  "Test learning content with reason\n\n*Archived from provisional: test reason*",
		Category: CategoryPatterns,
	}

	err = f.appendToArchiveFile(testLearning)
	if err != nil {
		t.Fatalf("appendToArchiveFile failed: %v", err)
	}

	// Read archive file
	archivePath := filepath.Join(tmpDir, "LEARNINGS_ARCHIVE.md")
	content, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("failed to read archive file: %v", err)
	}

	contentStr := string(content)
	// Should contain header with pipe-delimited format: ### YYYY-MM-DD | bead-id | category
	if !strings.Contains(contentStr, "###") {
		t.Error("expected archive file to contain header marker (###)")
	}
	if !strings.Contains(contentStr, "bead-format-test") {
		t.Error("expected archive file to contain bead ID")
	}
	if !strings.Contains(contentStr, CategoryPatterns) {
		t.Error("expected archive file to contain category")
	}
	if !strings.Contains(contentStr, "Test learning content") {
		t.Error("expected archive file to contain learning content")
	}
	if !strings.Contains(contentStr, "*Archived from provisional: test reason*") {
		t.Error("expected archive file to preserve archival reason annotation")
	}
}

// TestArchivedHashesFieldInitializedAsEmptySlice tests that archivedHashes field is initialized
func TestArchivedHashesFieldInitializedAsEmptySlice(t *testing.T) {
	// Expected failure: archivedHashes field does not exist on File yet
	tmpDir := t.TempDir()
	f, err := NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	// archivedHashes field should be initialized (not nil)
	if f.archivedHashes == nil {
		t.Error("expected archivedHashes field to be initialized (non-nil slice)")
	}

	// Should be empty initially
	if len(f.archivedHashes) != 0 {
		t.Errorf("expected archivedHashes to be empty initially, got %d elements", len(f.archivedHashes))
	}
}

// TestSetArchivedHashesStoresInternalSlice tests that SetArchivedHashes updates the internal slice
func TestSetArchivedHashesStoresInternalSlice(t *testing.T) {
	// Expected failure: SetArchivedHashes method and archivedHashes field do not exist yet
	tmpDir := t.TempDir()
	f, err := NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	testHashes := []string{"hash-a", "hash-b", "hash-c"}
	f.SetArchivedHashes(testHashes)

	// Internal field should match what was set
	if len(f.archivedHashes) != len(testHashes) {
		t.Errorf("expected archivedHashes field to have %d elements, got %d", len(testHashes), len(f.archivedHashes))
	}

	for i, expectedHash := range testHashes {
		if f.archivedHashes[i] != expectedHash {
			t.Errorf("archivedHashes[%d] = %q, expected %q", i, f.archivedHashes[i], expectedHash)
		}
	}
}

// TestGetArchivedHashesConvertsSliceToMap tests that GetArchivedHashes converts internal slice to map
func TestGetArchivedHashesConvertsSliceToMap(t *testing.T) {
	// Expected failure: GetArchivedHashes method and archivedHashes field do not exist yet
	tmpDir := t.TempDir()
	f, err := NewFile(tmpDir)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}

	testHashes := []string{"hash-x", "hash-y"}
	f.SetArchivedHashes(testHashes)

	// GetArchivedHashes should return a map, not the internal slice
	hashMap := f.GetArchivedHashes()

	// Verify it's a map with correct entries
	if len(hashMap) != len(testHashes) {
		t.Errorf("expected map with %d entries, got %d", len(testHashes), len(hashMap))
	}

	for _, hash := range testHashes {
		if !hashMap[hash] {
			t.Errorf("expected hash %q to be in returned map", hash)
		}
	}
}

// TestSetArchivedHashesNilReceiver tests that SetArchivedHashes is safe with nil receiver
func TestSetArchivedHashesNilReceiver(t *testing.T) {
	// Expected failure: SetArchivedHashes method does not exist yet
	var f *File
	// Should not panic
	f.SetArchivedHashes([]string{"hash1"})
}

// TestGetArchivedHashesNilReceiver tests that GetArchivedHashes returns empty map with nil receiver
func TestGetArchivedHashesNilReceiver(t *testing.T) {
	// Expected failure: GetArchivedHashes method does not exist yet
	var f *File
	hashes := f.GetArchivedHashes()

	// Should return empty map, not nil
	if hashes == nil {
		t.Fatal("expected non-nil map from GetArchivedHashes on nil receiver")
	}
	if len(hashes) != 0 {
		t.Errorf("expected empty map from nil receiver, got %d entries", len(hashes))
	}
}
