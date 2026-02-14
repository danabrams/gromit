package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewInteractiveFile verifies that NewInteractiveFile creates an InteractiveFile instance with correct path
func TestNewInteractiveFile(t *testing.T) {
	f, err := NewInteractiveFile("/tmp/test-gromit")
	if err != nil {
		t.Fatalf("NewInteractiveFile should not error: %v", err)
	}
	// Path should be .gromit/interactive-state.json
	expectedPath := "/tmp/test-gromit/interactive-state.json"
	if f.path != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, f.path)
	}
}

// TestInteractiveFileLoadNonExistent verifies that Load returns nil error when file doesn't exist
func TestInteractiveFileLoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewInteractiveFile(dir)

	if err := f.Load(); err != nil {
		t.Errorf("loading non-existent interactive-state should not error: %v", err)
	}

	if !f.LastRetro().IsZero() {
		t.Error("last retro should be zero for new interactive state")
	}
}

// TestInteractiveFileRecordRetroAndLoad verifies that RecordRetro records timestamp and persists
func TestInteractiveFileRecordRetroAndLoad(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewInteractiveFile(dir)

	before := time.Now()
	if err := f.RecordRetro(); err != nil {
		t.Fatalf("recording retro: %v", err)
	}
	after := time.Now()

	// Verify file was created
	if _, err := os.Stat(filepath.Join(dir, "interactive-state.json")); err != nil {
		t.Fatalf("interactive-state.json should exist: %v", err)
	}

	// Verify the time was recorded
	if f.LastRetro().Before(before) || f.LastRetro().After(after) {
		t.Error("last retro time should be between before and after")
	}

	// Load in a new InteractiveFile and verify persistence
	f2, _ := NewInteractiveFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading interactive state: %v", err)
	}

	// Allow 1 second tolerance for JSON serialization rounding
	diff := f.LastRetro().Sub(f2.LastRetro())
	if diff < -time.Second || diff > time.Second {
		t.Errorf("loaded retro time should match: got %v, want %v", f2.LastRetro(), f.LastRetro())
	}
}

// TestInteractiveFileLastReviewCommit verifies that LastReviewCommit is accessible
func TestInteractiveFileLastReviewCommit(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewInteractiveFile(dir)

	// Initial state should be empty
	if f.LastReviewCommit() != "" {
		t.Errorf("expected empty last review commit, got %q", f.LastReviewCommit())
	}

	// Record a review with commit hash
	if err := f.RecordReview("abc123", 5); err != nil {
		t.Fatal(err)
	}

	if f.LastReviewCommit() != "abc123" {
		t.Errorf("expected 'abc123', got %q", f.LastReviewCommit())
	}
}

// TestInteractiveFileLastReviewIteration verifies that LastReviewIteration is accessible
func TestInteractiveFileLastReviewIteration(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewInteractiveFile(dir)

	// Initial state should be 0
	if f.LastReviewIteration() != 0 {
		t.Errorf("expected 0, got %d", f.LastReviewIteration())
	}

	// Record a review with iteration number
	if err := f.RecordReview("abc123", 7); err != nil {
		t.Fatal(err)
	}

	if f.LastReviewIteration() != 7 {
		t.Errorf("expected 7, got %d", f.LastReviewIteration())
	}
}

// TestInteractiveFileRecordReview verifies that RecordReview stores commit and iteration
func TestInteractiveFileRecordReview(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewInteractiveFile(dir)

	// Record a review
	if err := f.RecordReview("def456", 10); err != nil {
		t.Fatal(err)
	}

	// Reload and check persistence
	f2, _ := NewInteractiveFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatal(err)
	}

	if f2.LastReviewCommit() != "def456" {
		t.Errorf("expected 'def456', got %q", f2.LastReviewCommit())
	}
	if f2.LastReviewIteration() != 10 {
		t.Errorf("expected 10, got %d", f2.LastReviewIteration())
	}
}

// TestInteractiveFileGetFilteredHashes verifies that GetFilteredHashes returns a map
func TestInteractiveFileGetFilteredHashes(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewInteractiveFile(dir)

	// Empty state returns empty map
	hashes := f.GetFilteredHashes()
	if hashes == nil {
		t.Error("GetFilteredHashes should return empty map, not nil")
	}
	if len(hashes) != 0 {
		t.Errorf("expected empty map, got %d entries", len(hashes))
	}

	// Add some hashes
	f.AddFilteredHashes([]string{"hash1", "hash2", "hash3"})
	hashes = f.GetFilteredHashes()

	if len(hashes) != 3 {
		t.Errorf("expected 3 hashes, got %d", len(hashes))
	}
	if !hashes["hash1"] {
		t.Error("hash1 should be in map")
	}
	if !hashes["hash2"] {
		t.Error("hash2 should be in map")
	}
	if !hashes["hash3"] {
		t.Error("hash3 should be in map")
	}
}

// TestInteractiveFileAddFilteredHashes verifies that AddFilteredHashes adds hashes with deduplication
func TestInteractiveFileAddFilteredHashes(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewInteractiveFile(dir)

	// Add initial hashes
	f.AddFilteredHashes([]string{"hash1", "hash2"})

	hashes := f.GetFilteredHashes()
	if len(hashes) != 2 {
		t.Errorf("expected 2 hashes, got %d", len(hashes))
	}

	// Add more hashes with one duplicate
	f.AddFilteredHashes([]string{"hash2", "hash3", "hash4"})

	// Should have 4 unique hashes total
	hashes = f.GetFilteredHashes()
	if len(hashes) != 4 {
		t.Errorf("expected 4 hashes after deduplication, got %d", len(hashes))
	}

	// Verify all unique hashes are present
	expectedHashes := []string{"hash1", "hash2", "hash3", "hash4"}
	for _, hash := range expectedHashes {
		if !hashes[hash] {
			t.Errorf("expected hash %s to be present", hash)
		}
	}
}

// TestInteractiveFileFilteredHashesPersistence verifies that filtered hashes persist across save/load
func TestInteractiveFileFilteredHashesPersistence(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewInteractiveFile(dir)

	// Add hashes and save
	f.AddFilteredHashes([]string{"hash1", "hash2", "hash3"})
	if err := f.Save(); err != nil {
		t.Fatalf("saving interactive state: %v", err)
	}

	// Load in a new InteractiveFile and verify persistence
	f2, _ := NewInteractiveFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading interactive state: %v", err)
	}

	hashes := f2.GetFilteredHashes()
	if len(hashes) != 3 {
		t.Errorf("expected 3 hashes after load, got %d", len(hashes))
	}

	if !hashes["hash1"] || !hashes["hash2"] || !hashes["hash3"] {
		t.Error("all hashes should be present after load")
	}
}

// TestInteractiveFileSaveAutoStampsUpdatedAt verifies that Save auto-stamps UpdatedAt field
func TestInteractiveFileSaveAutoStampsUpdatedAt(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewInteractiveFile(dir)

	before := time.Now()
	if err := f.Save(); err != nil {
		t.Fatalf("saving interactive state: %v", err)
	}
	after := time.Now()

	// Check UpdatedAt was stamped
	updatedAt := f.state.UpdatedAt
	if updatedAt.Before(before) || updatedAt.After(after) {
		t.Error("UpdatedAt should be between before and after")
	}

	// Load in a new InteractiveFile and verify UpdatedAt persisted
	f2, _ := NewInteractiveFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading interactive state: %v", err)
	}

	// Allow 1 second tolerance for JSON serialization rounding
	diff := updatedAt.Sub(f2.state.UpdatedAt)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("loaded UpdatedAt should match: got %v, want %v", f2.state.UpdatedAt, updatedAt)
	}
}

// TestInteractiveStateStruct verifies that InteractiveState struct has required fields
func TestInteractiveStateStruct(t *testing.T) {
	// Create an InteractiveState with all fields populated
	state := InteractiveState{
		LastRetro:              time.Now(),
		LastReviewCommit:       "abc123",
		LastReviewIteration:    5,
		FilteredLearningHashes: []string{"hash1", "hash2"},
		UpdatedAt:              time.Now(),
	}

	// Verify fields are accessible
	if state.LastRetro.IsZero() {
		t.Error("LastRetro should not be zero")
	}
	if state.LastReviewCommit != "abc123" {
		t.Errorf("expected LastReviewCommit='abc123', got %q", state.LastReviewCommit)
	}
	if state.LastReviewIteration != 5 {
		t.Errorf("expected LastReviewIteration=5, got %d", state.LastReviewIteration)
	}
	if len(state.FilteredLearningHashes) != 2 {
		t.Errorf("expected 2 filtered hashes, got %d", len(state.FilteredLearningHashes))
	}
	if state.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

// TestInteractiveStateJSONSerialization verifies that InteractiveState has proper JSON tags
func TestInteractiveStateJSONSerialization(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewInteractiveFile(dir)

	// Set up state with all fields
	if err := f.RecordRetro(); err != nil {
		t.Fatal(err)
	}
	if err := f.RecordReview("xyz789", 15); err != nil {
		t.Fatal(err)
	}
	f.AddFilteredHashes([]string{"filtered1", "filtered2"})

	// Save
	if err := f.Save(); err != nil {
		t.Fatalf("saving interactive state: %v", err)
	}

	// Read raw JSON
	data, err := os.ReadFile(filepath.Join(dir, "interactive-state.json"))
	if err != nil {
		t.Fatalf("reading interactive-state file: %v", err)
	}

	jsonStr := string(data)

	// Verify JSON field names (snake_case)
	if !contains(jsonStr, "last_retro") {
		t.Error("JSON should contain 'last_retro' field")
	}
	if !contains(jsonStr, "last_review_commit") {
		t.Error("JSON should contain 'last_review_commit' field")
	}
	if !contains(jsonStr, "last_review_iteration") {
		t.Error("JSON should contain 'last_review_iteration' field")
	}
	if !contains(jsonStr, "filtered_learning_hashes") {
		t.Error("JSON should contain 'filtered_learning_hashes' field")
	}
	if !contains(jsonStr, "updated_at") {
		t.Error("JSON should contain 'updated_at' field")
	}

	// Verify values are in JSON
	if !contains(jsonStr, "xyz789") {
		t.Error("JSON should contain 'xyz789' commit hash")
	}
	if !contains(jsonStr, "filtered1") {
		t.Error("JSON should contain 'filtered1' hash")
	}
}

// TestInteractiveFileNilSafety verifies that InteractiveFile methods handle nil receiver safely
func TestInteractiveFileNilSafety(t *testing.T) {
	var f *InteractiveFile

	// All methods should be nil-safe and not panic
	if !f.LastRetro().IsZero() {
		t.Error("nil InteractiveFile should return zero time")
	}
	if f.LastReviewCommit() != "" {
		t.Error("nil InteractiveFile should return empty string")
	}
	if f.LastReviewIteration() != 0 {
		t.Error("nil InteractiveFile should return 0")
	}

	hashes := f.GetFilteredHashes()
	if hashes != nil {
		t.Error("nil InteractiveFile should return nil from GetFilteredHashes")
	}

	// Mutation methods should not panic
	f.AddFilteredHashes([]string{"hash1"})
}

// TestInteractiveFileLoadCorruptFile verifies that Load returns error for corrupt JSON
func TestInteractiveFileLoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "interactive-state.json"), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	f, _ := NewInteractiveFile(dir)
	if err := f.Load(); err == nil {
		t.Error("loading corrupt interactive-state should return error")
	}
}

// TestInteractiveFileIsolatedFromStateFile verifies that InteractiveFile uses separate file
func TestInteractiveFileIsolatedFromStateFile(t *testing.T) {
	dir := t.TempDir()

	// Create regular state file
	stateFile, _ := NewFile(dir)
	stateFile.state.LastReviewCommit = "state-commit"
	if err := stateFile.Save(); err != nil {
		t.Fatal(err)
	}

	// Create interactive state file with different data
	interactiveFile, _ := NewInteractiveFile(dir)
	if err := interactiveFile.RecordReview("interactive-commit", 20); err != nil {
		t.Fatal(err)
	}

	// Verify both files exist
	statePath := filepath.Join(dir, "state.json")
	interactivePath := filepath.Join(dir, "interactive-state.json")

	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state.json should exist: %v", err)
	}
	if _, err := os.Stat(interactivePath); err != nil {
		t.Fatalf("interactive-state.json should exist: %v", err)
	}

	// Verify they contain different data
	stateData, _ := os.ReadFile(statePath)
	interactiveData, _ := os.ReadFile(interactivePath)

	stateStr := string(stateData)
	interactiveStr := string(interactiveData)

	if !contains(stateStr, "state-commit") {
		t.Error("state.json should contain 'state-commit'")
	}
	if contains(stateStr, "interactive-commit") {
		t.Error("state.json should NOT contain 'interactive-commit'")
	}

	if !contains(interactiveStr, "interactive-commit") {
		t.Error("interactive-state.json should contain 'interactive-commit'")
	}
	if contains(interactiveStr, "state-commit") {
		t.Error("interactive-state.json should NOT contain 'state-commit'")
	}
}

// TestInteractiveStateNormalizeNilFields verifies that NormalizeNilFields initializes slices
func TestInteractiveStateNormalizeNilFields(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewInteractiveFile(dir)

	// Manually set state to have nil slice
	f.state.FilteredLearningHashes = nil

	// Call NormalizeNilFields to initialize it
	f.state.NormalizeNilFields()

	// Verify slice is initialized (not nil)
	if f.state.FilteredLearningHashes == nil {
		t.Error("FilteredLearningHashes should be initialized to empty slice, not nil")
	}

	// Verify it's an empty slice
	if len(f.state.FilteredLearningHashes) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(f.state.FilteredLearningHashes))
	}

	// Should be able to append without panic
	f.AddFilteredHashes([]string{"hash1"})
	if len(f.state.FilteredLearningHashes) != 1 {
		t.Errorf("expected 1 hash after adding, got %d", len(f.state.FilteredLearningHashes))
	}
}

// TestInteractiveFileRoundTrip verifies complete round-trip of all interactive state fields
func TestInteractiveFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewInteractiveFile(dir)

	// Set up complete state
	retroTime := time.Now().Add(-24 * time.Hour)
	f.state.LastRetro = retroTime
	if err := f.RecordReview("full-commit-hash", 42); err != nil {
		t.Fatal(err)
	}
	f.AddFilteredHashes([]string{"hash_a", "hash_b", "hash_c"})

	// Save
	if err := f.Save(); err != nil {
		t.Fatalf("saving interactive state: %v", err)
	}

	// Load in new InteractiveFile
	f2, _ := NewInteractiveFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading interactive state: %v", err)
	}

	// Verify LastRetro (allow 1 second tolerance)
	diff := retroTime.Sub(f2.LastRetro())
	if diff < -time.Second || diff > time.Second {
		t.Errorf("LastRetro mismatch: got %v, want %v", f2.LastRetro(), retroTime)
	}

	// Verify LastReviewCommit
	if f2.LastReviewCommit() != "full-commit-hash" {
		t.Errorf("expected 'full-commit-hash', got %q", f2.LastReviewCommit())
	}

	// Verify LastReviewIteration
	if f2.LastReviewIteration() != 42 {
		t.Errorf("expected 42, got %d", f2.LastReviewIteration())
	}

	// Verify FilteredLearningHashes
	hashes := f2.GetFilteredHashes()
	if len(hashes) != 3 {
		t.Errorf("expected 3 hashes, got %d", len(hashes))
	}
	if !hashes["hash_a"] || !hashes["hash_b"] || !hashes["hash_c"] {
		t.Error("all hashes should be present after load")
	}

	// Verify UpdatedAt was set
	if f2.state.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set after load")
	}
}
