//go:build acceptance

package retro

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/state"
)

// hashEvictionEnv holds the test environment for filtered hash eviction tests.
type hashEvictionEnv struct {
	tmpDir       string
	provisionals []learnings.Learning
}

// setupHashEviction creates a test environment with config and learnings.
// If learningsContent is non-empty, it writes a LEARNINGS.md and loads provisionals.
// If empty, it creates an empty learnings file.
func setupHashEviction(t *testing.T, learningsContent string) hashEvictionEnv {
	t.Helper()

	tmpDir := t.TempDir()

	var provisionals []learnings.Learning

	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	lf, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating learnings file: %v", err)
	}

	if learningsContent != "" {
		if err := os.WriteFile(learningsPath, []byte(learningsContent), 0644); err != nil {
			t.Fatalf("writing learnings file: %v", err)
		}
		if err := lf.Load(); err != nil {
			t.Fatalf("loading learnings: %v", err)
		}
		provisionals = lf.GetProvisional()
	} else {
		if err := lf.Save(); err != nil {
			t.Fatalf("saving empty learnings: %v", err)
		}
	}

	return hashEvictionEnv{
		tmpDir:       tmpDir,
		provisionals: provisionals,
	}
}

// addFilteredHashes writes the given hashes to state in the test environment.
func (env hashEvictionEnv) addFilteredHashes(t *testing.T, hashes []string) {
	t.Helper()
	sf, err := state.NewFile(env.tmpDir)
	if err != nil {
		t.Fatalf("creating state file: %v", err)
	}
	if err := sf.Load(); err != nil {
		t.Fatalf("loading state: %v", err)
	}
	sf.AddFilteredHashes(hashes)
	if err := sf.Save(); err != nil {
		t.Fatalf("saving state: %v", err)
	}
}

// runRetro creates a Retro instance and runs it, returning the filtered hashes from state.
func (env hashEvictionEnv) runRetro(t *testing.T) map[string]bool {
	t.Helper()
	// Create provider from config
	mockProvider := &mockProvider{
		runResult: &provider.Result{
			Success: true,
			Output:  `{"category":"patterns","project_relevant":true}`,
		},
	}
	r, err := NewRetroWithProvider(mockProvider, env.tmpDir)
	if err != nil {
		t.Fatalf("creating retro: %v", err)
	}
	ctx := context.Background()
	if _, err := r.Run(ctx, nil); err != nil {
		t.Fatalf("running retro: %v", err)
	}

	sf, err := state.NewFile(env.tmpDir)
	if err != nil {
		t.Fatalf("creating state file for verification: %v", err)
	}
	if err := sf.Load(); err != nil {
		t.Fatalf("loading state for verification: %v", err)
	}
	return sf.GetFilteredHashes()
}

func assertHashesUnchanged(t *testing.T, initialHashes, finalHashes []string) {
	t.Helper()

	if len(initialHashes) != len(finalHashes) {
		t.Fatalf("state.json should not change filtered_learning_hashes when no hashes need to be added or pruned; initial=%v final=%v", initialHashes, finalHashes)
	}
	for i, hash := range initialHashes {
		if finalHashes[i] != hash {
			t.Fatalf("state.json should not change filtered_learning_hashes when no hashes need to be added or pruned; initial=%v final=%v", initialHashes, finalHashes)
		}
	}
}

func mustHaveProvisionals(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %d provisional learnings, got %d", want, got)
	}
}

type stateFixture struct {
	FilteredLearningHashes []string `json:"filtered_learning_hashes"`
}

func readFilteredHashesFromState(t *testing.T, statePath string) []string {
	t.Helper()

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("reading state file: %v", err)
	}

	var sf stateFixture
	if err := json.Unmarshal(data, &sf); err != nil {
		t.Fatalf("decoding state file: %v", err)
	}
	return sf.FilteredLearningHashes
}

func sortedCopy(values []string) []string {
	out := make([]string, len(values))
	copy(out, values)
	sort.Strings(out)
	return out
}

// TestFilteredHashEviction_RemovesStaleHashesAfterRetroRun verifies that after retro.Run()
// completes, FilteredLearningHashes contains only hashes matching current provisional learnings.
func TestFilteredHashEviction_RemovesStaleHashesAfterRetroRun(t *testing.T) {
	env := setupHashEviction(t, `# Learnings

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
`)

	mustHaveProvisionals(t, len(env.provisionals), 3)

	hash1 := env.provisionals[0].Hash
	hash2 := env.provisionals[1].Hash
	hash3 := env.provisionals[2].Hash

	env.addFilteredHashes(t, []string{hash1, hash2, hash3, "stale4444dddd", "stale5555eeee"})

	hashes := env.runRetro(t)

	if len(hashes) != 3 {
		t.Errorf("expected 3 filtered hashes after reconciliation, got %d", len(hashes))
	}
	if !hashes[hash1] {
		t.Errorf("hash %s should remain (matches current provisional)", hash1)
	}
	if !hashes[hash2] {
		t.Errorf("hash %s should remain (matches current provisional)", hash2)
	}
	if !hashes[hash3] {
		t.Errorf("hash %s should remain (matches current provisional)", hash3)
	}
	if hashes["stale4444dddd"] {
		t.Error("stale4444dddd should be removed (no matching provisional)")
	}
	if hashes["stale5555eeee"] {
		t.Error("stale5555eeee should be removed (no matching provisional)")
	}
}

// TestFilteredHashEviction_NoSaveWhenNoChanges verifies that no save occurs
// if there are no new hashes to add and no stale hashes to prune.
func TestFilteredHashEviction_NoSaveWhenNoChanges(t *testing.T) {
	env := setupHashEviction(t, `# Learnings

## Confirmed

*No confirmed learnings yet.*

## Provisional

### 2026-02-01 | bead-1 | patterns

First learning

### 2026-02-02 | bead-2 | conventions

Second learning

## Archived

*No archived learnings.*
`)

	mustHaveProvisionals(t, len(env.provisionals), 2)

	hash1 := env.provisionals[0].Hash
	hash2 := env.provisionals[1].Hash

	env.addFilteredHashes(t, []string{hash1, hash2})

	// Record initial state content for deterministic comparison.
	statePath := filepath.Join(env.tmpDir, "state.json")
	initialHashes := sortedCopy(readFilteredHashesFromState(t, statePath))

	hashes := env.runRetro(t)
	finalHashes := sortedCopy(readFilteredHashesFromState(t, statePath))

	assertHashesUnchanged(t, initialHashes, finalHashes)
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
	env := setupHashEviction(t, "")
	env.addFilteredHashes(t, []string{"hash1", "hash2", "hash3"})

	hashes := env.runRetro(t)

	if len(hashes) != 0 {
		t.Errorf("expected 0 filtered hashes when no provisional learnings exist, got %d", len(hashes))
	}
}
