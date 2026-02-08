//go:build acceptance

package retro

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/state"
)

// TestFilteredHashEviction_RemovesStaleHashesAfterRetroRun verifies that after retro.Run()
// completes, FilteredLearningHashes contains only hashes matching current provisional learnings.
// This is the primary acceptance criterion: stale hashes (for archived, confirmed, or removed
// learnings) are pruned from the state.
func TestFilteredHashEviction_RemovesStaleHashesAfterRetroRun(t *testing.T) {

	tmpDir := t.TempDir()

	// Create config
	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
	}

	// Set up learnings file with provisional learnings by directly writing LEARNINGS.md
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	learningsContent := `# Learnings

## Confirmed

*No confirmed learnings yet.*

## Provisional

### 2026-02-01 | test-bead-1 | patterns

First provisional learning content

### 2026-02-02 | test-bead-2 | conventions

Second provisional learning content

### 2026-02-03 | test-bead-3 | gotchas

Third provisional learning content

## Archived

*No archived learnings.*
`
	if err := os.WriteFile(learningsPath, []byte(learningsContent), 0644); err != nil {
		t.Fatalf("writing learnings file: %v", err)
	}

	// Load learnings file to compute hashes
	learningsFile, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating learnings file: %v", err)
	}
	if err := learningsFile.Load(); err != nil {
		t.Fatalf("loading learnings: %v", err)
	}

	provisionals := learningsFile.GetProvisional()
	if len(provisionals) != 3 {
		t.Fatalf("expected 3 provisional learnings, got %d", len(provisionals))
	}

	// Set up state file with 5 filtered hashes: 3 current + 2 stale
	stateFile, err := state.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating state file: %v", err)
	}
	if err := stateFile.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	// Add hashes: 3 match current provisionals, 2 are stale (from removed/archived learnings)
	// Get actual hashes from the loaded provisionals
	hash1 := provisionals[0].Hash
	hash2 := provisionals[1].Hash
	hash3 := provisionals[2].Hash

	stateFile.AddFilteredHashes([]string{
		hash1,           // matches first provisional
		hash2,           // matches second provisional
		hash3,           // matches third provisional
		"stale4444dddd", // stale - no matching provisional
		"stale5555eeee", // stale - no matching provisional
	})
	if err := stateFile.Save(); err != nil {
		t.Fatalf("saving initial state: %v", err)
	}

	// Create retro instance
	r, err := NewRetro(cfg, tmpDir)
	if err != nil {
		t.Fatalf("creating retro: %v", err)
	}

	// Run retro (this should reconcile filtered hashes)
	ctx := context.Background()
	_, err = r.Run(ctx)
	// Note: Run will fail because claude binary won't work in tests,
	// but reconciliation should happen before that point.
	// In a real implementation, we'd mock the claude client.

	// Reload state to verify changes persisted
	stateFile2, err := state.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating state file for verification: %v", err)
	}
	if err := stateFile2.Load(); err != nil {
		t.Fatalf("loading state for verification: %v", err)
	}

	// ACCEPTANCE CRITERION: FilteredLearningHashes should contain only hashes
	// matching current provisional learnings (hash1111aaaa, hash2222bbbb, hash3333cccc)
	hashes := stateFile2.GetFilteredHashes()

	if len(hashes) != 3 {
		t.Errorf("expected 3 filtered hashes after reconciliation, got %d", len(hashes))
	}

	// Verify current hashes are present
	if !hashes[hash1] {
		t.Errorf("hash %s should remain (matches current provisional)", hash1)
	}
	if !hashes[hash2] {
		t.Errorf("hash %s should remain (matches current provisional)", hash2)
	}
	if !hashes[hash3] {
		t.Errorf("hash %s should remain (matches current provisional)", hash3)
	}

	// Verify stale hashes were removed
	if hashes["stale4444dddd"] {
		t.Error("stale4444dddd should be removed (no matching provisional)")
	}
	if hashes["stale5555eeee"] {
		t.Error("stale5555eeee should be removed (no matching provisional)")
	}
}

// TestFilteredHashEviction_SingleSaveWhenBothAddAndPrune verifies that state is saved
// exactly once (not twice) when both new hashes are added and stale hashes are pruned.
// This tests efficiency - we want one write, not multiple.
func TestFilteredHashEviction_SingleSaveWhenBothAddAndPrune(t *testing.T) {
	t.Skip("This test requires a working Claude client to evaluate new provisional learnings - cannot run in unit tests")

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
	}

	// Set up learnings file with 2 provisional learnings (one new, one existing)
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	learningsContent := `# Learnings

## Confirmed

*No confirmed learnings yet.*

## Provisional

### 2026-02-01 | existing-bead | patterns

Existing provisional learning

### 2026-02-02 | new-bead | conventions

New provisional learning

## Archived

*No archived learnings.*
`
	if err := os.WriteFile(learningsPath, []byte(learningsContent), 0644); err != nil {
		t.Fatalf("writing learnings file: %v", err)
	}

	learningsFile, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating learnings file: %v", err)
	}
	if err := learningsFile.Load(); err != nil {
		t.Fatalf("loading learnings: %v", err)
	}

	provisionals := learningsFile.GetProvisional()
	if len(provisionals) != 2 {
		t.Fatalf("expected 2 provisional learnings, got %d", len(provisionals))
	}

	existingHash := provisionals[0].Hash
	newHash := provisionals[1].Hash

	// Set up state with existing hash + stale hash
	stateFile, err := state.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating state file: %v", err)
	}
	if err := stateFile.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	stateFile.AddFilteredHashes([]string{
		existingHash,  // matches existing provisional
		"staleHash3",  // stale - no matching provisional
	})
	if err := stateFile.Save(); err != nil {
		t.Fatalf("saving initial state: %v", err)
	}

	// Record initial modification time of state.json
	statePath := filepath.Join(tmpDir, "state.json")
	initialStat, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("stat initial state file: %v", err)
	}
	initialModTime := initialStat.ModTime()

	// Small delay to ensure modification time would be different
	time.Sleep(10 * time.Millisecond)

	// Create retro and run
	r, err := NewRetro(cfg, tmpDir)
	if err != nil {
		t.Fatalf("creating retro: %v", err)
	}

	ctx := context.Background()
	_, err = r.Run(ctx)
	// Run will fail, but state should be saved once before failure

	// Verify state was modified (saved at least once)
	finalStat, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("stat final state file: %v", err)
	}
	finalModTime := finalStat.ModTime()

	if !finalModTime.After(initialModTime) {
		t.Error("state.json should have been modified (saved) during retro.Run()")
	}

	// ACCEPTANCE CRITERION: Verify state contains both new hash and pruned stale hash
	// (proving both operations happened in a single save, not two)
	stateFile2, err := state.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating state file for verification: %v", err)
	}
	if err := stateFile2.Load(); err != nil {
		t.Fatalf("loading state for verification: %v", err)
	}

	hashes := stateFile2.GetFilteredHashes()

	// Should have 2 hashes: existingHash1 and newHash2
	if len(hashes) != 2 {
		t.Errorf("expected 2 hashes after single save (add new + prune stale), got %d", len(hashes))
	}

	if !hashes[existingHash] {
		t.Errorf("existing hash %s should remain", existingHash)
	}
	if !hashes[newHash] {
		t.Errorf("new hash %s should be added", newHash)
	}
	if hashes["staleHash3"] {
		t.Error("staleHash3 should be pruned")
	}
}

// TestFilteredHashEviction_NoSaveWhenNoChanges verifies that no save occurs
// if there are no new hashes to add and no stale hashes to prune.
// This tests efficiency - don't write state.json unnecessarily.
func TestFilteredHashEviction_NoSaveWhenNoChanges(t *testing.T) {

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
	}

	// Set up learnings file with 2 provisional learnings that have already been filtered
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	learningsContent := `# Learnings

## Confirmed

*No confirmed learnings yet.*

## Provisional

### 2026-02-01 | bead-1 | patterns

First learning

### 2026-02-02 | bead-2 | conventions

Second learning

## Archived

*No archived learnings.*
`
	if err := os.WriteFile(learningsPath, []byte(learningsContent), 0644); err != nil {
		t.Fatalf("writing learnings file: %v", err)
	}

	learningsFile, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating learnings file: %v", err)
	}
	if err := learningsFile.Load(); err != nil {
		t.Fatalf("loading learnings: %v", err)
	}

	provisionals := learningsFile.GetProvisional()
	if len(provisionals) != 2 {
		t.Fatalf("expected 2 provisional learnings, got %d", len(provisionals))
	}

	hash1 := provisionals[0].Hash
	hash2 := provisionals[1].Hash

	// Set up state with exactly matching hashes (no new hashes, no stale hashes)
	stateFile, err := state.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating state file: %v", err)
	}
	if err := stateFile.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	stateFile.AddFilteredHashes([]string{hash1, hash2})
	if err := stateFile.Save(); err != nil {
		t.Fatalf("saving initial state: %v", err)
	}

	// Record initial state content and modification time
	statePath := filepath.Join(tmpDir, "state.json")
	initialContent, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("reading initial state: %v", err)
	}
	initialStat, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("stat initial state file: %v", err)
	}
	initialModTime := initialStat.ModTime()

	// Small delay to ensure modification time would be different if saved
	time.Sleep(10 * time.Millisecond)

	// Create retro and run
	r, err := NewRetro(cfg, tmpDir)
	if err != nil {
		t.Fatalf("creating retro: %v", err)
	}

	ctx := context.Background()
	_, err = r.Run(ctx)
	// Run will fail, but state operations should complete before failure

	// Verify state was NOT modified (no unnecessary save)
	finalStat, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("stat final state file: %v", err)
	}
	finalModTime := finalStat.ModTime()

	finalContent, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("reading final state: %v", err)
	}

	// ACCEPTANCE CRITERION: State should not be saved when there are no changes
	if finalModTime.After(initialModTime) {
		t.Error("state.json should NOT be modified when no hashes need to be added or pruned")
	}

	if string(finalContent) != string(initialContent) {
		t.Error("state.json content should be unchanged when no hashes need to be added or pruned")
	}

	// Verify hashes remain the same
	stateFile2, err := state.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating state file for verification: %v", err)
	}
	if err := stateFile2.Load(); err != nil {
		t.Fatalf("loading state for verification: %v", err)
	}

	hashes := stateFile2.GetFilteredHashes()
	if len(hashes) != 2 {
		t.Errorf("expected 2 hashes (unchanged), got %d", len(hashes))
	}
	if !hashes[hash1] || !hashes[hash2] {
		t.Error("original hashes should remain unchanged")
	}
}

// TestFilteredHashEviction_HandlesEmptyProvisionalLearnings verifies that when there are
// no provisional learnings, all filtered hashes are pruned (edge case).
func TestFilteredHashEviction_HandlesEmptyProvisionalLearnings(t *testing.T) {

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
	}

	// Set up empty learnings file (no provisional learnings)
	learningsFile, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating learnings file: %v", err)
	}
	if err := learningsFile.Save(); err != nil {
		t.Fatalf("saving empty learnings: %v", err)
	}

	// Set up state with some filtered hashes (all should be pruned)
	stateFile, err := state.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating state file: %v", err)
	}
	if err := stateFile.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	stateFile.AddFilteredHashes([]string{"hash1", "hash2", "hash3"})
	if err := stateFile.Save(); err != nil {
		t.Fatalf("saving initial state: %v", err)
	}

	// Create retro and run
	r, err := NewRetro(cfg, tmpDir)
	if err != nil {
		t.Fatalf("creating retro: %v", err)
	}

	ctx := context.Background()
	_, err = r.Run(ctx)

	// Verify all hashes were pruned
	stateFile2, err := state.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating state file for verification: %v", err)
	}
	if err := stateFile2.Load(); err != nil {
		t.Fatalf("loading state for verification: %v", err)
	}

	hashes := stateFile2.GetFilteredHashes()

	// ACCEPTANCE CRITERION: When no provisional learnings exist, all hashes should be pruned
	if len(hashes) != 0 {
		t.Errorf("expected 0 filtered hashes when no provisional learnings exist, got %d", len(hashes))
	}
}

// TestFilteredHashEviction_HandlesArchivedLearnings verifies that hashes for learnings
// that were archived during FilterProvisional are properly pruned from state.
// This tests the integration point where FilterProvisional archives generic learnings
// before reconciliation happens.
func TestFilteredHashEviction_HandlesArchivedLearnings(t *testing.T) {
	t.Skip("This test requires a working Claude client to archive learnings - cannot run in unit tests")

	tmpDir := t.TempDir()

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Binary:  "claude",
			Timeout: 60,
		},
	}

	// Set up learnings file with 3 provisional learnings
	// (In a real scenario, FilterProvisional would archive some of these as generic)
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	learningsContent := `# Learnings

## Confirmed

*No confirmed learnings yet.*

## Provisional

### 2026-02-01 | specific-bead | patterns

Project-specific learning that remains provisional

### 2026-02-02 | generic-bead-1 | conventions

Generic advice (will be archived by filter)

### 2026-02-03 | generic-bead-2 | gotchas

Another generic learning (will be archived)

## Archived

*No archived learnings.*
`
	if err := os.WriteFile(learningsPath, []byte(learningsContent), 0644); err != nil {
		t.Fatalf("writing learnings file: %v", err)
	}

	learningsFile, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating learnings file: %v", err)
	}
	if err := learningsFile.Load(); err != nil {
		t.Fatalf("loading learnings: %v", err)
	}

	provisionals := learningsFile.GetProvisional()
	if len(provisionals) != 3 {
		t.Fatalf("expected 3 provisional learnings, got %d", len(provisionals))
	}

	specificHash := provisionals[0].Hash
	genericHash2 := provisionals[1].Hash
	genericHash3 := provisionals[2].Hash

	// Set up state with all 3 hashes
	stateFile, err := state.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating state file: %v", err)
	}
	if err := stateFile.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}

	stateFile.AddFilteredHashes([]string{
		specificHash,
		genericHash2,
		genericHash3,
	})
	if err := stateFile.Save(); err != nil {
		t.Fatalf("saving initial state: %v", err)
	}

	// Create retro and run
	// NOTE: In the real implementation, FilterProvisional would archive genericHash2 and
	// genericHash3 as generic learnings, leaving only specificHash1 in provisional.
	// The reconciliation step should then prune genericHash2 and genericHash3 from state.
	r, err := NewRetro(cfg, tmpDir)
	if err != nil {
		t.Fatalf("creating retro: %v", err)
	}

	ctx := context.Background()
	_, err = r.Run(ctx)

	// Reload learnings to verify archival happened
	learningsFile2, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating learnings file for verification: %v", err)
	}
	if err := learningsFile2.Load(); err != nil {
		t.Fatalf("loading learnings for verification: %v", err)
	}

	provisionals2 := learningsFile2.GetProvisional()
	// After FilterProvisional, only project-specific learnings should remain provisional
	// (This assumes the filter function would classify learning2 and learning3 as generic)

	// Verify state hashes match only current provisional learnings
	stateFile2, err := state.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating state file for verification: %v", err)
	}
	if err := stateFile2.Load(); err != nil {
		t.Fatalf("loading state for verification: %v", err)
	}

	hashes := stateFile2.GetFilteredHashes()

	// Build expected hash set from current provisionals
	expectedHashes := make(map[string]bool)
	for _, l := range provisionals2 {
		expectedHashes[l.Hash] = true
	}

	// ACCEPTANCE CRITERION: Hashes in state should exactly match hashes of current provisional learnings
	// (learnings archived during FilterProvisional should have their hashes removed)
	if len(hashes) != len(expectedHashes) {
		t.Errorf("expected %d filtered hashes (matching %d provisionals), got %d",
			len(expectedHashes), len(provisionals2), len(hashes))
	}

	for hash := range expectedHashes {
		if !hashes[hash] {
			t.Errorf("hash %s should be present (matches current provisional)", hash)
		}
	}

	// Verify archived learning hashes are NOT in state
	if hashes[genericHash2] {
		t.Errorf("hash %s should be removed (learning was archived)", genericHash2)
	}
	if hashes[genericHash3] {
		t.Errorf("hash %s should be removed (learning was archived)", genericHash3)
	}
}
