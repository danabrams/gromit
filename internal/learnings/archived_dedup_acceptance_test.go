//go:build acceptance

package learnings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAddSkipsArchivedDuplicateByHash verifies that Add() returns (nil, nil)
// when the new learning's content hash matches an existing archived learning's hash.
func TestAddSkipsArchivedDuplicateByHash(t *testing.T) {
	// Expected failure: Add() does not check f.archived for hash matches yet
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	content := "Always check error returns properly"

	// First, add a learning and archive it
	learning1, err := f.Add("bead-1", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if learning1 == nil {
		t.Fatal("first learning should not be nil")
	}

	// Archive the learning
	err = f.Archive(learning1.Hash, "test reason")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Verify it's in archived
	if len(f.archived) != 1 {
		t.Fatalf("expected 1 archived learning, got %d", len(f.archived))
	}
	if len(f.provisional) != 0 {
		t.Fatalf("expected 0 provisional after archive, got %d", len(f.provisional))
	}

	// Try to add the same content again - should return (nil, nil) because hash matches archived
	learning2, err := f.Add("bead-2", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}

	// Expected: learning2 should be nil (skipped)
	if learning2 != nil {
		t.Error("add should return nil when content hash matches an archived learning")
	}

	// Should still have only 1 archived, nothing added to provisional or confirmed
	if len(f.archived) != 1 {
		t.Errorf("expected 1 archived (unchanged), got %d", len(f.archived))
	}
	if len(f.provisional) != 0 {
		t.Errorf("expected 0 provisional, got %d", len(f.provisional))
	}
	if len(f.confirmed) != 0 {
		t.Errorf("expected 0 confirmed, got %d", len(f.confirmed))
	}
}

// TestAddSkipsArchivedDuplicateNormalized verifies that Add() skips archived
// duplicates even when whitespace and case differ.
func TestAddSkipsArchivedDuplicateNormalized(t *testing.T) {
	// Expected failure: Add() does not check f.archived for hash matches yet
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	content1 := "Always  check   error   returns"
	content2 := "always check error returns" // Different case and spacing, same normalized hash

	// Add and archive first learning
	learning1, err := f.Add("bead-1", content1, CategoryPatterns)
	if err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if learning1 == nil {
		t.Fatal("first learning should not be nil")
	}

	err = f.Archive(learning1.Hash, "archived for testing")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Try to add normalized duplicate
	learning2, err := f.Add("bead-2", content2, CategoryPatterns)
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}

	// Expected: should be skipped because normalized hashes match
	if learning2 != nil {
		t.Error("add should return nil for normalized duplicate in archived")
	}

	// Verify counts unchanged
	if len(f.archived) != 1 {
		t.Errorf("expected 1 archived (unchanged), got %d", len(f.archived))
	}
	if len(f.provisional) != 0 {
		t.Errorf("expected 0 provisional, got %d", len(f.provisional))
	}
}

// TestAddDoesNotSaveWhenSkippingArchivedDuplicate verifies that Save() is not
// called when skipping an archived duplicate.
func TestAddDoesNotSaveWhenSkippingArchivedDuplicate(t *testing.T) {
	// Expected failure: Add() does not check f.archived for hash matches yet
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	content := "Always validate input data"

	// Add and archive a learning
	learning1, err := f.Add("bead-1", content, CategoryConventions)
	if err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if learning1 == nil {
		t.Fatal("first learning should not be nil")
	}

	err = f.Archive(learning1.Hash, "no longer relevant")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Get the file's modification time after archiving
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	stat1, err := os.Stat(learningsPath)
	if err != nil {
		t.Fatalf("could not stat LEARNINGS.md: %v", err)
	}
	modTime1 := stat1.ModTime()

	// Try to add duplicate - should skip without saving
	learning2, err := f.Add("bead-2", content, CategoryConventions)
	if err != nil {
		t.Fatalf("second add returned error: %v", err)
	}
	if learning2 != nil {
		t.Fatal("duplicate should return nil")
	}

	// Check that file was not modified (Save() was not called)
	stat2, err := os.Stat(learningsPath)
	if err != nil {
		t.Fatalf("could not stat LEARNINGS.md after duplicate add: %v", err)
	}
	modTime2 := stat2.ModTime()

	if !modTime2.Equal(modTime1) {
		t.Error("LEARNINGS.md should not be saved when skipping archived duplicate")
	}
}

// TestAddDoesNotCallFilterForArchivedDuplicate verifies that the filter function
// is not called when content matches an existing archived learning.
func TestAddDoesNotCallFilterForArchivedDuplicate(t *testing.T) {
	// Expected failure: Add() does not check f.archived for hash matches yet
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	content := "Always use meaningful variable names"

	// Add and archive a learning (without filter)
	learning1, err := f.Add("bead-1", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if learning1 == nil {
		t.Fatal("first learning should not be nil")
	}

	err = f.Archive(learning1.Hash, "archived for test")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Set up a filter that tracks whether it's called
	filterCalled := false
	f.SetFilter(func(content string) (bool, error) {
		filterCalled = true
		return false, nil // Return project-specific
	})

	// Try to add duplicate - filter should NOT be called
	learning2, err := f.Add("bead-2", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}
	if learning2 != nil {
		t.Error("duplicate should return nil")
	}

	// Verify filter was not called
	if filterCalled {
		t.Error("filter function should not be called for archived duplicates")
	}

	// Verify counts unchanged
	if len(f.archived) != 1 {
		t.Errorf("expected 1 archived (unchanged), got %d", len(f.archived))
	}
	if len(f.provisional) != 0 {
		t.Errorf("expected 0 provisional, got %d", len(f.provisional))
	}
}

// TestAddArchivedDedupBeforeFilter verifies that archived dedup check happens
// before the filter is called, even when filter would archive the content.
func TestAddArchivedDedupBeforeFilter(t *testing.T) {
	// Expected failure: Add() does not check f.archived for hash matches yet
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	content := "Always write unit tests"

	// Add and archive manually (simulating a previously filtered learning)
	learning1, err := f.Add("bead-1", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if learning1 == nil {
		t.Fatal("first learning should not be nil")
	}

	err = f.Archive(learning1.Hash, "filtered: generic engineering advice")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Set up a filter that would also archive as generic
	filterCallCount := 0
	f.SetFilter(func(content string) (bool, error) {
		filterCallCount++
		return true, nil // Would mark as generic
	})

	// Try to add the same content again
	learning2, err := f.Add("bead-2", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}

	// Should be skipped due to archived dedup, not filter
	if learning2 != nil {
		t.Error("duplicate should return nil due to archived dedup")
	}

	// Filter should not have been called (dedup happens first)
	if filterCallCount > 0 {
		t.Errorf("filter should not be called when archived dedup matches, but was called %d times", filterCallCount)
	}

	// Should still have exactly 1 archived entry (not 2)
	if len(f.archived) != 1 {
		t.Errorf("expected 1 archived entry (not duplicated), got %d", len(f.archived))
	}
}

// TestExistingConfirmedAndProvisionalDedupUnchanged verifies that the existing
// dedup behavior for confirmed and provisional learnings remains unchanged.
func TestExistingConfirmedAndProvisionalDedupUnchanged(t *testing.T) {
	// This test verifies existing behavior is preserved - it should pass both before and after
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	content := "Always handle errors gracefully"

	// Test confirmed dedup
	f.confirmed = append(f.confirmed, Learning{
		BeadID:   "bead-confirmed",
		Content:  content,
		Category: CategoryPatterns,
		Hash:     hashContent(content),
	})

	learning, err := f.Add("bead-1", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if learning != nil {
		t.Error("should skip duplicate of confirmed learning")
	}

	// Clear confirmed, test provisional dedup
	f.confirmed = []Learning{}
	f.provisional = append(f.provisional, Learning{
		BeadID:   "bead-provisional",
		Content:  content,
		Category: CategoryPatterns,
		Hash:     hashContent(content),
	})

	learning, err = f.Add("bead-2", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if learning != nil {
		t.Error("should skip duplicate of provisional learning")
	}
}

// TestArchivedDedupImplementationLocation verifies that the archived dedup check
// is implemented in the Add() method at the correct location (after provisional
// dedup check, before filter call).
func TestArchivedDedupImplementationLocation(t *testing.T) {
	// Expected failure: archived dedup loop does not exist in Add() yet
	// Read learnings.go and verify the implementation structure
	source, err := os.ReadFile("learnings.go")
	if err != nil {
		t.Fatalf("could not read learnings.go: %v", err)
	}

	sourceStr := string(source)

	// Find the Add() method
	addMethodStart := strings.Index(sourceStr, "func (f *File) Add(")
	if addMethodStart == -1 {
		t.Fatal("could not find Add() method")
	}

	// Find the next method to bound our search to just Add()
	nextMethodStart := strings.Index(sourceStr[addMethodStart+10:], "\nfunc ")
	if nextMethodStart == -1 {
		nextMethodStart = len(sourceStr) - addMethodStart - 10
	}
	addMethodEnd := addMethodStart + 10 + nextMethodStart
	addMethodBody := sourceStr[addMethodStart:addMethodEnd]

	// Find the provisional dedup loop (around line 124-128)
	provisionalDedupPattern := "for _, l := range f.provisional {"
	provisionalDedupIndex := strings.Index(addMethodBody, provisionalDedupPattern)
	if provisionalDedupIndex == -1 {
		t.Fatal("could not find provisional dedup loop in Add() method")
	}

	// Find the filter check (around line 131)
	filterPattern := "if f.filterFunc != nil {"
	filterIndex := strings.Index(addMethodBody, filterPattern)
	if filterIndex == -1 {
		t.Fatal("could not find filter check in Add() method")
	}

	// Expected: archived dedup loop should exist between provisional dedup and filter
	archivedDedupPattern := "for _, l := range f.archived {"
	archivedDedupIndex := strings.Index(addMethodBody, archivedDedupPattern)

	// The loop must exist
	if archivedDedupIndex == -1 {
		t.Error("archived dedup loop not found in Add() method")
	}

	// If it exists, verify it's in the right location
	if archivedDedupIndex != -1 {
		if archivedDedupIndex <= provisionalDedupIndex {
			t.Error("archived dedup loop should come after provisional dedup loop")
		}
		if archivedDedupIndex >= filterIndex {
			t.Error("archived dedup loop should come before filter check")
		}

		// Extract just the section between archived loop start and filter check
		archivedDedupSection := addMethodBody[archivedDedupIndex:filterIndex]
		if !strings.Contains(archivedDedupSection, "return nil, nil") {
			t.Error("archived dedup loop should return (nil, nil) on hash match")
		}
	}
}
