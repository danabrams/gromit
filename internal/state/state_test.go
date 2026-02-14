package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewFile(t *testing.T) {
	f, _ := NewFile("/tmp/test-gromit")
	if f.path != "/tmp/test-gromit/state.json" {
		t.Errorf("expected path /tmp/test-gromit/state.json, got %s", f.path)
	}
}

func TestLoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	if err := f.Load(); err != nil {
		t.Errorf("loading non-existent state should not error: %v", err)
	}

	if !f.LastRetro().IsZero() {
		t.Error("last retro should be zero for new state")
	}
}

func TestRecordRetroAndLoad(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	before := time.Now()
	if err := f.RecordRetro(); err != nil {
		t.Fatalf("recording retro: %v", err)
	}
	after := time.Now()

	// Verify file was created
	if _, err := os.Stat(filepath.Join(dir, "state.json")); err != nil {
		t.Fatalf("state.json should exist: %v", err)
	}

	// Verify the time was recorded
	if f.LastRetro().Before(before) || f.LastRetro().After(after) {
		t.Error("last retro time should be between before and after")
	}

	// Load in a new File and verify persistence
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	// Allow 1 second tolerance for JSON serialization rounding
	diff := f.LastRetro().Sub(f2.LastRetro())
	if diff < -time.Second || diff > time.Second {
		t.Errorf("loaded retro time should match: got %v, want %v", f2.LastRetro(), f.LastRetro())
	}
}

func TestLoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	f, _ := NewFile(dir)
	if err := f.Load(); err == nil {
		t.Error("loading corrupt state should return error")
	}
}

func TestReviewState(t *testing.T) {
	tmpDir := t.TempDir()
	sf, _ := NewFile(tmpDir)

	// Initial state has no review info
	if err := sf.Load(); err != nil {
		t.Fatal(err)
	}
	if sf.LastReviewCommit() != "" {
		t.Errorf("expected empty last review commit, got %q", sf.LastReviewCommit())
	}
	if sf.LastReviewIteration() != 0 {
		t.Errorf("expected 0 last review iteration, got %d", sf.LastReviewIteration())
	}
	if sf.IterationsSinceReview() != 0 {
		t.Errorf("expected 0 iterations since review, got %d", sf.IterationsSinceReview())
	}

	// Record a review
	if err := sf.RecordReview("abc123", 5); err != nil {
		t.Fatal(err)
	}

	// Reload and check
	sf2, _ := NewFile(tmpDir)
	if err := sf2.Load(); err != nil {
		t.Fatal(err)
	}
	if sf2.LastReviewCommit() != "abc123" {
		t.Errorf("expected 'abc123', got %q", sf2.LastReviewCommit())
	}
	if sf2.LastReviewIteration() != 5 {
		t.Errorf("expected 5, got %d", sf2.LastReviewIteration())
	}
}

func TestIncrementIterationsSinceReview(t *testing.T) {
	tmpDir := t.TempDir()
	sf, _ := NewFile(tmpDir)
	if err := sf.Load(); err != nil {
		t.Fatal(err)
	}

	sf.IncrementIterationsSinceReview()
	if sf.IterationsSinceReview() != 1 {
		t.Errorf("expected 1, got %d", sf.IterationsSinceReview())
	}

	sf.IncrementIterationsSinceReview()
	sf.IncrementIterationsSinceReview()
	if sf.IterationsSinceReview() != 3 {
		t.Errorf("expected 3, got %d", sf.IterationsSinceReview())
	}

	// RecordReview resets counter
	if err := sf.RecordReview("def456", 8); err != nil {
		t.Fatal(err)
	}
	if sf.IterationsSinceReview() != 0 {
		t.Errorf("expected 0 after RecordReview, got %d", sf.IterationsSinceReview())
	}
}

func TestSaveAutoStampsUpdatedAt(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	before := time.Now()
	if err := f.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}
	after := time.Now()

	// Check UpdatedAt was stamped
	if f.state.UpdatedAt.Before(before) || f.state.UpdatedAt.After(after) {
		t.Error("UpdatedAt should be between before and after")
	}

	// Load in a new File and verify UpdatedAt persisted
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	// Allow 1 second tolerance for JSON serialization rounding
	diff := f.state.UpdatedAt.Sub(f2.state.UpdatedAt)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("loaded UpdatedAt should match: got %v, want %v", f2.state.UpdatedAt, f.state.UpdatedAt)
	}
}

func TestCleanExitPersistence(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Set CleanExit to true and save
	f.state.CleanExit = true
	if err := f.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	// Load in a new File and verify CleanExit persisted
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	if !f2.state.CleanExit {
		t.Error("CleanExit should be true after load")
	}

	// Set to false and verify it persists
	f2.state.CleanExit = false
	if err := f2.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	f3, _ := NewFile(dir)
	if err := f3.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	if f3.state.CleanExit {
		t.Error("CleanExit should be false after load")
	}
}

func TestSetCleanExit(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Default state should have CleanExit as false (zero value)
	if f.state.CleanExit {
		t.Error("CleanExit should default to false")
	}

	// Set to true
	f.SetCleanExit(true)
	if !f.state.CleanExit {
		t.Error("CleanExit should be true after SetCleanExit(true)")
	}

	// Set to false
	f.SetCleanExit(false)
	if f.state.CleanExit {
		t.Error("CleanExit should be false after SetCleanExit(false)")
	}
}

func TestSetCleanExitNilSafe(t *testing.T) {
	var f *File
	// Should not panic
	f.SetCleanExit(true)
}

func TestCheckStaleness(t *testing.T) {
	tests := []struct {
		name              string
		cleanExit         bool
		updatedAt         time.Time
		thresholdMinutes  int
		expectStale       bool
		expectReasonMatch string
	}{
		{
			name:              "clean exit with recent timestamp",
			cleanExit:         true,
			updatedAt:         time.Now().Add(-5 * time.Minute),
			thresholdMinutes:  60,
			expectStale:       false,
			expectReasonMatch: "",
		},
		{
			name:              "crash detected (cleanExit false)",
			cleanExit:         false,
			updatedAt:         time.Now().Add(-5 * time.Minute),
			thresholdMinutes:  60,
			expectStale:       true,
			expectReasonMatch: "crash detected",
		},
		{
			name:              "stale timestamp exceeds threshold",
			cleanExit:         true,
			updatedAt:         time.Now().Add(-90 * time.Minute),
			thresholdMinutes:  60,
			expectStale:       true,
			expectReasonMatch: "stale",
		},
		{
			name:              "zero timestamp with cleanExit true",
			cleanExit:         true,
			updatedAt:         time.Time{},
			thresholdMinutes:  60,
			expectStale:       false,
			expectReasonMatch: "",
		},
		{
			name:              "crash with zero timestamp",
			cleanExit:         false,
			updatedAt:         time.Time{},
			thresholdMinutes:  60,
			expectStale:       true,
			expectReasonMatch: "crash detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			f, _ := NewFile(dir)
			f.state.CleanExit = tt.cleanExit
			f.state.UpdatedAt = tt.updatedAt

			isStale, reason := f.CheckStaleness(tt.thresholdMinutes)

			if isStale != tt.expectStale {
				t.Errorf("expected isStale=%v, got %v (reason: %s)", tt.expectStale, isStale, reason)
			}

			if tt.expectReasonMatch != "" {
				if reason == "" {
					t.Errorf("expected reason to contain %q, got empty string", tt.expectReasonMatch)
				} else if !contains(reason, tt.expectReasonMatch) {
					t.Errorf("expected reason to contain %q, got %q", tt.expectReasonMatch, reason)
				}
			} else if reason != "" {
				t.Errorf("expected empty reason, got %q", reason)
			}
		})
	}
}

func TestCheckStalenessNilSafe(t *testing.T) {
	var f *File
	isStale, reason := f.CheckStaleness(60)
	if isStale {
		t.Error("nil File should not be stale")
	}
	if reason != "" {
		t.Errorf("nil File should return empty reason, got %q", reason)
	}
}

func TestAutoHeal(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Set up state with all fields populated
	f.state.IterationsSinceReview = 5
	f.state.LastReviewIteration = 10
	f.state.LastReviewCommit = "abc123"
	f.state.LastRetro = time.Now().Add(-24 * time.Hour)

	// Call AutoHeal
	f.AutoHeal()

	// Check that counters were reset
	if f.state.IterationsSinceReview != 0 {
		t.Errorf("IterationsSinceReview should be reset to 0, got %d", f.state.IterationsSinceReview)
	}
	if f.state.LastReviewIteration != 0 {
		t.Errorf("LastReviewIteration should be reset to 0, got %d", f.state.LastReviewIteration)
	}

	// Check that git anchors and timestamps were preserved
	if f.state.LastReviewCommit != "abc123" {
		t.Errorf("LastReviewCommit should be preserved, got %q", f.state.LastReviewCommit)
	}
	if f.state.LastRetro.IsZero() {
		t.Error("LastRetro should be preserved")
	}
}

func TestAutoHealNilSafe(t *testing.T) {
	var f *File
	// Should not panic
	f.AutoHeal()
}

func TestGetFilteredHashes(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Empty state returns empty map
	hashes := f.GetFilteredHashes()
	if hashes == nil {
		t.Error("GetFilteredHashes should return empty map, not nil")
	}
	if len(hashes) != 0 {
		t.Errorf("expected empty map, got %d entries", len(hashes))
	}

	// Add some hashes
	f.state.FilteredLearningHashes = []string{"hash1", "hash2", "hash3"}
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

func TestGetFilteredHashesNilSafe(t *testing.T) {
	var f *File
	hashes := f.GetFilteredHashes()
	if hashes != nil {
		t.Error("nil File should return nil map")
	}
}

func TestAddFilteredHashes(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Add initial hashes
	f.AddFilteredHashes([]string{"hash1", "hash2"})

	if len(f.state.FilteredLearningHashes) != 2 {
		t.Errorf("expected 2 hashes, got %d", len(f.state.FilteredLearningHashes))
	}

	// Add more hashes with one duplicate
	f.AddFilteredHashes([]string{"hash2", "hash3", "hash4"})

	// Should have 4 unique hashes total
	if len(f.state.FilteredLearningHashes) != 4 {
		t.Errorf("expected 4 hashes after deduplication, got %d", len(f.state.FilteredLearningHashes))
	}

	// Verify all unique hashes are present
	hashMap := f.GetFilteredHashes()
	expectedHashes := []string{"hash1", "hash2", "hash3", "hash4"}
	for _, hash := range expectedHashes {
		if !hashMap[hash] {
			t.Errorf("expected hash %s to be present", hash)
		}
	}
}

func TestAddFilteredHashesNilSafe(t *testing.T) {
	var f *File
	// Should not panic
	f.AddFilteredHashes([]string{"hash1", "hash2"})
}

func TestFilteredLearningHashesPersistence(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Add hashes and save
	f.AddFilteredHashes([]string{"hash1", "hash2", "hash3"})
	if err := f.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	// Load in a new File and verify persistence
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	if len(f2.state.FilteredLearningHashes) != 3 {
		t.Errorf("expected 3 hashes after load, got %d", len(f2.state.FilteredLearningHashes))
	}

	hashMap := f2.GetFilteredHashes()
	if !hashMap["hash1"] || !hashMap["hash2"] || !hashMap["hash3"] {
		t.Error("all hashes should be present after load")
	}
}

func TestReconcileFilteredHashes_RemovesStaleHashes(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Start with 5 hashes
	f.state.FilteredLearningHashes = []string{"hash1", "hash2", "hash3", "hash4", "hash5"}

	// Current set only has 3 of them (hash2, hash3, hash5 are gone)
	currentHashes := map[string]bool{
		"hash1": true,
		"hash4": true,
	}

	// ReconcileFilteredHashes should remove hash2, hash3, hash5
	pruned := f.ReconcileFilteredHashes(currentHashes)

	if !pruned {
		t.Error("expected ReconcileFilteredHashes to return true when hashes were removed")
	}

	// Verify only hash1 and hash4 remain
	if len(f.state.FilteredLearningHashes) != 2 {
		t.Errorf("expected 2 hashes after reconciliation, got %d", len(f.state.FilteredLearningHashes))
	}

	hashMap := f.GetFilteredHashes()
	if !hashMap["hash1"] {
		t.Error("hash1 should remain after reconciliation")
	}
	if !hashMap["hash4"] {
		t.Error("hash4 should remain after reconciliation")
	}
	if hashMap["hash2"] {
		t.Error("hash2 should be removed")
	}
	if hashMap["hash3"] {
		t.Error("hash3 should be removed")
	}
	if hashMap["hash5"] {
		t.Error("hash5 should be removed")
	}
}

func TestReconcileFilteredHashes_NoChangesWhenAllMatch(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Start with 3 hashes
	f.state.FilteredLearningHashes = []string{"hash1", "hash2", "hash3"}

	// Current set has exactly the same hashes
	currentHashes := map[string]bool{
		"hash1": true,
		"hash2": true,
		"hash3": true,
	}

	// ReconcileFilteredHashes should return false (no pruning)
	pruned := f.ReconcileFilteredHashes(currentHashes)

	if pruned {
		t.Error("expected ReconcileFilteredHashes to return false when no hashes were removed")
	}

	// Verify all 3 hashes remain
	if len(f.state.FilteredLearningHashes) != 3 {
		t.Errorf("expected 3 hashes after reconciliation, got %d", len(f.state.FilteredLearningHashes))
	}

	hashMap := f.GetFilteredHashes()
	if !hashMap["hash1"] || !hashMap["hash2"] || !hashMap["hash3"] {
		t.Error("all original hashes should remain when current set matches")
	}
}

func TestReconcileFilteredHashes_EmptyCurrentSet(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Start with some hashes
	f.state.FilteredLearningHashes = []string{"hash1", "hash2", "hash3"}

	// Current set is empty - all hashes should be removed
	currentHashes := map[string]bool{}

	pruned := f.ReconcileFilteredHashes(currentHashes)

	if !pruned {
		t.Error("expected ReconcileFilteredHashes to return true when all hashes were removed")
	}

	// Verify all hashes were removed
	if len(f.state.FilteredLearningHashes) != 0 {
		t.Errorf("expected 0 hashes after reconciliation with empty current set, got %d", len(f.state.FilteredLearningHashes))
	}
}

func TestReconcileFilteredHashes_EmptyInitialState(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Start with no hashes
	f.state.FilteredLearningHashes = []string{}

	// Current set has some hashes (but none match because initial is empty)
	currentHashes := map[string]bool{
		"hash1": true,
		"hash2": true,
	}

	pruned := f.ReconcileFilteredHashes(currentHashes)

	if pruned {
		t.Error("expected ReconcileFilteredHashes to return false when initial state was empty")
	}

	// Verify state remains empty
	if len(f.state.FilteredLearningHashes) != 0 {
		t.Errorf("expected 0 hashes after reconciliation from empty initial state, got %d", len(f.state.FilteredLearningHashes))
	}
}

func TestReconcileFilteredHashes_NilSafe(t *testing.T) {
	var f *File
	currentHashes := map[string]bool{
		"hash1": true,
	}

	// Should not panic
	pruned := f.ReconcileFilteredHashes(currentHashes)

	if pruned {
		t.Error("nil File should return false")
	}
}

func TestReconcileFilteredHashes_Persistence(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Add hashes
	f.state.FilteredLearningHashes = []string{"hash1", "hash2", "hash3", "hash4"}

	// Reconcile to remove hash2 and hash4
	currentHashes := map[string]bool{
		"hash1": true,
		"hash3": true,
	}
	f.ReconcileFilteredHashes(currentHashes)

	// Save state
	if err := f.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	// Load in a new File and verify reconciliation persisted
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	if len(f2.state.FilteredLearningHashes) != 2 {
		t.Errorf("expected 2 hashes after load, got %d", len(f2.state.FilteredLearningHashes))
	}

	hashMap := f2.GetFilteredHashes()
	if !hashMap["hash1"] || !hashMap["hash3"] {
		t.Error("hash1 and hash3 should be present after load")
	}
	if hashMap["hash2"] || hashMap["hash4"] {
		t.Error("hash2 and hash4 should not be present after load")
	}
}

// TestProviderCountsInitialization verifies that ProviderCounts map is initialized when loading state
// Expected failure: ProviderCounts field does not exist on State struct yet
func TestProviderCountsInitialization(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Load state - should initialize ProviderCounts as empty map
	if err := f.Load(); err != nil {
		t.Fatal(err)
	}

	counts := f.GetProviderCounts()
	if counts == nil {
		t.Error("GetProviderCounts should return empty map, not nil")
	}
	if len(counts) != 0 {
		t.Errorf("expected empty provider counts, got %d entries", len(counts))
	}
}

// TestIncrementProviderCount verifies that provider invocation counts can be incremented
// Expected failure: IncrementProviderCount method does not exist on File yet
func TestIncrementProviderCount(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Increment claude count multiple times
	f.IncrementProviderCount("claude")
	f.IncrementProviderCount("claude")
	f.IncrementProviderCount("claude")

	// Increment openai count once
	f.IncrementProviderCount("openai")

	counts := f.GetProviderCounts()
	if counts["claude"] != 3 {
		t.Errorf("expected claude count 3, got %d", counts["claude"])
	}
	if counts["openai"] != 1 {
		t.Errorf("expected openai count 1, got %d", counts["openai"])
	}
}

// TestIncrementProviderCountPersistence verifies that provider counts persist across save/load
// Expected failure: ProviderCounts field does not exist on State struct yet
func TestIncrementProviderCountPersistence(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Increment counts
	f.IncrementProviderCount("claude")
	f.IncrementProviderCount("claude")
	f.IncrementProviderCount("openai")

	// Save state
	if err := f.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	// Load in new File
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	counts := f2.GetProviderCounts()
	if counts["claude"] != 2 {
		t.Errorf("expected claude count 2 after load, got %d", counts["claude"])
	}
	if counts["openai"] != 1 {
		t.Errorf("expected openai count 1 after load, got %d", counts["openai"])
	}
}

// TestGetProviderCountsNilSafe verifies that GetProviderCounts handles nil receiver safely
// Expected failure: GetProviderCounts method does not exist on File yet
func TestGetProviderCountsNilSafe(t *testing.T) {
	var f *File
	counts := f.GetProviderCounts()
	if counts != nil {
		t.Error("nil File should return nil from GetProviderCounts")
	}
}

// TestResetProviderCounts verifies that provider counts can be reset to zero
// Expected failure: ResetProviderCounts method does not exist on File yet
func TestResetProviderCounts(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Build up some counts
	f.IncrementProviderCount("claude")
	f.IncrementProviderCount("claude")
	f.IncrementProviderCount("openai")
	f.IncrementProviderCount("gemini")

	// Reset counts
	f.ResetProviderCounts()

	counts := f.GetProviderCounts()
	if len(counts) != 0 {
		t.Errorf("expected empty counts after reset, got %d entries", len(counts))
	}
}

// TestResetProviderCountsPersistence verifies that reset counts persist across save/load
// Expected failure: ResetProviderCounts method does not exist on File yet
func TestResetProviderCountsPersistence(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Build up counts and save
	f.IncrementProviderCount("claude")
	f.IncrementProviderCount("openai")
	if err := f.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	// Reset and save again
	f.ResetProviderCounts()
	if err := f.Save(); err != nil {
		t.Fatalf("saving state after reset: %v", err)
	}

	// Load in new File and verify counts are empty
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	counts := f2.GetProviderCounts()
	if len(counts) != 0 {
		t.Errorf("expected empty counts after load, got %d entries", len(counts))
	}
}

// TestSetProviderUnavailable verifies that a provider can be marked as unavailable
// Expected failure: SetProviderUnavailable method does not exist on File yet
func TestSetProviderUnavailable(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Initially provider should be available
	if !f.IsProviderAvailable("claude") {
		t.Error("provider should be available initially")
	}

	// Mark provider unavailable until a future time
	until := time.Now().Add(30 * time.Minute)
	f.SetProviderUnavailable("claude", until)

	// Provider should now be unavailable
	if f.IsProviderAvailable("claude") {
		t.Error("provider should be unavailable after SetProviderUnavailable")
	}
}

// TestSetProviderUnavailablePersistence verifies that unavailable status persists across save/load
// Expected failure: ProviderUnavailableUntil field does not exist on State struct yet
func TestSetProviderUnavailablePersistence(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Mark provider unavailable
	until := time.Now().Add(1 * time.Hour)
	f.SetProviderUnavailable("claude", until)

	// Save state
	if err := f.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	// Load in new File
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	// Provider should still be unavailable
	if f2.IsProviderAvailable("claude") {
		t.Error("provider should still be unavailable after load")
	}
}

// TestIsProviderAvailableAfterCooldown verifies that provider becomes available after cooldown expires
// Expected failure: IsProviderAvailable method does not exist on File yet
func TestIsProviderAvailableAfterCooldown(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Mark provider unavailable until a past time
	pastTime := time.Now().Add(-10 * time.Minute)
	f.SetProviderUnavailable("claude", pastTime)

	// Provider should be available because cooldown has passed
	if !f.IsProviderAvailable("claude") {
		t.Error("provider should be available after cooldown expires")
	}
}

// TestIsProviderAvailableNilSafe verifies that IsProviderAvailable handles nil receiver safely
// Expected failure: IsProviderAvailable method does not exist on File yet
func TestIsProviderAvailableNilSafe(t *testing.T) {
	var f *File
	// Should not panic and should return true (available by default)
	if !f.IsProviderAvailable("claude") {
		t.Error("nil File should return true for IsProviderAvailable")
	}
}

// TestClearProviderUnavailable verifies that unavailable status can be cleared
// Expected failure: ClearProviderUnavailable method does not exist on File yet
func TestClearProviderUnavailable(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Mark provider unavailable
	until := time.Now().Add(1 * time.Hour)
	f.SetProviderUnavailable("claude", until)

	// Provider should be unavailable
	if f.IsProviderAvailable("claude") {
		t.Error("provider should be unavailable after SetProviderUnavailable")
	}

	// Clear unavailable status
	f.ClearProviderUnavailable("claude")

	// Provider should now be available
	if !f.IsProviderAvailable("claude") {
		t.Error("provider should be available after ClearProviderUnavailable")
	}
}

// TestClearProviderUnavailablePersistence verifies that cleared status persists across save/load
// Expected failure: ClearProviderUnavailable method does not exist on File yet
func TestClearProviderUnavailablePersistence(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Mark provider unavailable
	until := time.Now().Add(1 * time.Hour)
	f.SetProviderUnavailable("claude", until)

	// Clear status and save
	f.ClearProviderUnavailable("claude")
	if err := f.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	// Load in new File
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	// Provider should be available
	if !f2.IsProviderAvailable("claude") {
		t.Error("provider should be available after load")
	}
}

// TestMultipleProvidersUnavailable verifies that multiple providers can be tracked independently
// Expected failure: ProviderUnavailableUntil field does not exist on State struct yet
func TestMultipleProvidersUnavailable(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Mark claude unavailable (far future)
	claudeUntil := time.Now().Add(2 * time.Hour)
	f.SetProviderUnavailable("claude", claudeUntil)

	// Mark openai unavailable (past)
	openaiUntil := time.Now().Add(-10 * time.Minute)
	f.SetProviderUnavailable("openai", openaiUntil)

	// Claude should be unavailable
	if f.IsProviderAvailable("claude") {
		t.Error("claude should be unavailable")
	}

	// OpenAI should be available (cooldown expired)
	if !f.IsProviderAvailable("openai") {
		t.Error("openai should be available (cooldown expired)")
	}

	// Gemini was never marked unavailable, should be available
	if !f.IsProviderAvailable("gemini") {
		t.Error("gemini should be available (never marked unavailable)")
	}
}

// TestProviderRoutingStateRoundTrip verifies complete round-trip of provider routing state
// Expected failure: ProviderCounts and ProviderUnavailableUntil fields do not exist on State struct yet
func TestProviderRoutingStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Set up provider counts
	f.IncrementProviderCount("claude")
	f.IncrementProviderCount("claude")
	f.IncrementProviderCount("claude")
	f.IncrementProviderCount("openai")
	f.IncrementProviderCount("openai")
	f.IncrementProviderCount("gemini")

	// Mark one provider unavailable
	claudeUntil := time.Now().Add(30 * time.Minute)
	f.SetProviderUnavailable("claude", claudeUntil)

	// Save state
	if err := f.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	// Load in new File
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	// Verify counts persisted
	counts := f2.GetProviderCounts()
	if counts["claude"] != 3 {
		t.Errorf("expected claude count 3, got %d", counts["claude"])
	}
	if counts["openai"] != 2 {
		t.Errorf("expected openai count 2, got %d", counts["openai"])
	}
	if counts["gemini"] != 1 {
		t.Errorf("expected gemini count 1, got %d", counts["gemini"])
	}

	// Verify unavailable status persisted
	if f2.IsProviderAvailable("claude") {
		t.Error("claude should still be unavailable after load")
	}
	if !f2.IsProviderAvailable("openai") {
		t.Error("openai should be available after load")
	}
	if !f2.IsProviderAvailable("gemini") {
		t.Error("gemini should be available after load")
	}
}

// TestNormalizeNilFieldsInitializesMaps verifies that NormalizeNilFields initializes provider maps
// Expected failure: NormalizeNilFields method does not exist yet, or does not initialize provider maps
func TestNormalizeNilFieldsInitializesMaps(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Manually set state to have nil maps (simulating deserialization of old state files)
	f.state.ProviderCounts = nil
	f.state.ProviderUnavailableUntil = nil

	// Call NormalizeNilFields to initialize them
	f.state.NormalizeNilFields()

	// Verify maps are initialized (not nil)
	if f.state.ProviderCounts == nil {
		t.Error("ProviderCounts should be initialized to empty map, not nil")
	}
	if f.state.ProviderUnavailableUntil == nil {
		t.Error("ProviderUnavailableUntil should be initialized to empty map, not nil")
	}

	// Should be able to use maps without panic
	f.IncrementProviderCount("claude")
	f.SetProviderUnavailable("openai", time.Now().Add(1*time.Hour))

	counts := f.GetProviderCounts()
	if counts["claude"] != 1 {
		t.Errorf("expected claude count 1, got %d", counts["claude"])
	}
	if f.IsProviderAvailable("openai") {
		t.Error("openai should be unavailable")
	}
}

// TestProviderCountsWithZeroValues verifies that providers with zero counts are handled correctly
// Expected failure: GetProviderCounts method does not exist on File yet
func TestProviderCountsWithZeroValues(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Increment and get counts
	f.IncrementProviderCount("claude")

	counts := f.GetProviderCounts()

	// Provider with count should be present
	if counts["claude"] != 1 {
		t.Errorf("expected claude count 1, got %d", counts["claude"])
	}

	// Provider never incremented should return zero (map zero-value behavior)
	if counts["openai"] != 0 {
		t.Errorf("expected openai count 0 (zero value), got %d", counts["openai"])
	}
}

// TestGetArchivedHashes verifies that archived learning hashes can be retrieved as a map
// Expected failure: GetArchivedHashes method does not exist on File yet
func TestGetArchivedHashes(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Empty state returns empty map
	hashes := f.GetArchivedHashes()
	if hashes == nil {
		t.Error("GetArchivedHashes should return empty map, not nil")
	}
	if len(hashes) != 0 {
		t.Errorf("expected empty map, got %d entries", len(hashes))
	}

	// Add some archived hashes directly to state
	f.state.ArchivedLearningHashes = []string{"archived1", "archived2", "archived3"}
	hashes = f.GetArchivedHashes()

	if len(hashes) != 3 {
		t.Errorf("expected 3 hashes, got %d", len(hashes))
	}
	if !hashes["archived1"] {
		t.Error("archived1 should be in map")
	}
	if !hashes["archived2"] {
		t.Error("archived2 should be in map")
	}
	if !hashes["archived3"] {
		t.Error("archived3 should be in map")
	}
}

// TestGetArchivedHashesNilSafe verifies that GetArchivedHashes handles nil receiver safely
// Expected failure: GetArchivedHashes method does not exist on File yet
func TestGetArchivedHashesNilSafe(t *testing.T) {
	var f *File
	hashes := f.GetArchivedHashes()
	if hashes != nil {
		t.Error("nil File should return nil map")
	}
}

// TestAddArchivedHashes verifies that archived hashes can be added with deduplication
// Expected failure: AddArchivedHashes method does not exist on File yet
func TestAddArchivedHashes(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Add initial hashes
	f.AddArchivedHashes([]string{"archived1", "archived2"})

	if len(f.state.ArchivedLearningHashes) != 2 {
		t.Errorf("expected 2 hashes, got %d", len(f.state.ArchivedLearningHashes))
	}

	// Add more hashes with one duplicate
	f.AddArchivedHashes([]string{"archived2", "archived3", "archived4"})

	// Should have 4 unique hashes total
	if len(f.state.ArchivedLearningHashes) != 4 {
		t.Errorf("expected 4 hashes after deduplication, got %d", len(f.state.ArchivedLearningHashes))
	}

	// Verify all unique hashes are present
	hashMap := f.GetArchivedHashes()
	expectedHashes := []string{"archived1", "archived2", "archived3", "archived4"}
	for _, hash := range expectedHashes {
		if !hashMap[hash] {
			t.Errorf("expected hash %s to be present", hash)
		}
	}
}

// TestAddArchivedHashesNilSafe verifies that AddArchivedHashes handles nil receiver safely
// Expected failure: AddArchivedHashes method does not exist on File yet
func TestAddArchivedHashesNilSafe(t *testing.T) {
	var f *File
	// Should not panic
	f.AddArchivedHashes([]string{"archived1", "archived2"})
}

// TestArchivedLearningHashesPersistence verifies that archived hashes persist across save/load
// Expected failure: ArchivedLearningHashes field does not exist on State struct yet
func TestArchivedLearningHashesPersistence(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Add hashes and save
	f.AddArchivedHashes([]string{"archived1", "archived2", "archived3"})
	if err := f.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	// Load in a new File and verify persistence
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	if len(f2.state.ArchivedLearningHashes) != 3 {
		t.Errorf("expected 3 hashes after load, got %d", len(f2.state.ArchivedLearningHashes))
	}

	hashMap := f2.GetArchivedHashes()
	if !hashMap["archived1"] || !hashMap["archived2"] || !hashMap["archived3"] {
		t.Error("all hashes should be present after load")
	}
}

// TestNormalizeNilFieldsInitializesArchivedHashes verifies that NormalizeNilFields initializes archived hashes slice
// Expected failure: NormalizeNilFields does not initialize ArchivedLearningHashes field yet
func TestNormalizeNilFieldsInitializesArchivedHashes(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Manually set state to have nil slice (simulating deserialization of old state files)
	f.state.ArchivedLearningHashes = nil

	// Call NormalizeNilFields to initialize it
	f.state.NormalizeNilFields()

	// Verify slice is initialized (not nil)
	if f.state.ArchivedLearningHashes == nil {
		t.Error("ArchivedLearningHashes should be initialized to empty slice, not nil")
	}

	// Verify it's an empty slice
	if len(f.state.ArchivedLearningHashes) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(f.state.ArchivedLearningHashes))
	}

	// Should be able to append without panic
	f.AddArchivedHashes([]string{"archived1"})
	if len(f.state.ArchivedLearningHashes) != 1 {
		t.Errorf("expected 1 hash after adding, got %d", len(f.state.ArchivedLearningHashes))
	}
}

// TestArchivedHashesJSONTag verifies that ArchivedLearningHashes has correct JSON serialization
// Expected failure: ArchivedLearningHashes field does not have JSON tag yet
func TestArchivedHashesJSONTag(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Add archived hashes
	f.AddArchivedHashes([]string{"archived1", "archived2"})

	// Save and verify JSON contains the field
	if err := f.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	// Read raw JSON
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("reading state file: %v", err)
	}

	jsonStr := string(data)
	// Verify field is present in JSON with correct name
	if !contains(jsonStr, "archived_learning_hashes") {
		t.Error("JSON should contain 'archived_learning_hashes' field")
	}

	// Verify values are in JSON
	if !contains(jsonStr, "archived1") {
		t.Error("JSON should contain 'archived1' value")
	}
	if !contains(jsonStr, "archived2") {
		t.Error("JSON should contain 'archived2' value")
	}
}

// TestArchivedAndFilteredHashesIndependence verifies that archived and filtered hashes are tracked independently
// Expected failure: ArchivedLearningHashes field does not exist on State struct yet
func TestArchivedAndFilteredHashesIndependence(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Add filtered hashes
	f.AddFilteredHashes([]string{"filtered1", "filtered2"})

	// Add archived hashes (using same values to test independence)
	f.AddArchivedHashes([]string{"filtered1", "archived1"})

	// Verify filtered hashes
	filteredMap := f.GetFilteredHashes()
	if len(filteredMap) != 2 {
		t.Errorf("expected 2 filtered hashes, got %d", len(filteredMap))
	}
	if !filteredMap["filtered1"] || !filteredMap["filtered2"] {
		t.Error("filtered hashes should contain filtered1 and filtered2")
	}

	// Verify archived hashes
	archivedMap := f.GetArchivedHashes()
	if len(archivedMap) != 2 {
		t.Errorf("expected 2 archived hashes, got %d", len(archivedMap))
	}
	if !archivedMap["filtered1"] || !archivedMap["archived1"] {
		t.Error("archived hashes should contain filtered1 and archived1")
	}

	// Verify they are truly independent - filtered1 appears in both
	if !filteredMap["filtered1"] {
		t.Error("filtered1 should be in filtered hashes")
	}
	if !archivedMap["filtered1"] {
		t.Error("filtered1 should be in archived hashes")
	}
}

// TestArchivedHashesEmptySliceSerialization verifies that empty archived hashes serialize correctly
// Expected failure: ArchivedLearningHashes field does not exist on State struct yet
func TestArchivedHashesEmptySliceSerialization(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)

	// Initialize with NormalizeNilFields
	f.state.NormalizeNilFields()

	// Save without adding any archived hashes
	if err := f.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	// Load in new File
	f2, _ := NewFile(dir)
	if err := f2.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	// After normalization, should have empty slice not nil
	f2.state.NormalizeNilFields()
	if f2.state.ArchivedLearningHashes == nil {
		t.Error("ArchivedLearningHashes should be empty slice after load and normalization, not nil")
	}
	if len(f2.state.ArchivedLearningHashes) != 0 {
		t.Errorf("expected 0 archived hashes, got %d", len(f2.state.ArchivedLearningHashes))
	}
}

// Helper function for substring matching
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
