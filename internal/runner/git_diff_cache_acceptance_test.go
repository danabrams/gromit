//go:build acceptance

package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// TestAcceptance_GitDiffCachedInBeadContext verifies that getDiff() caches
// its result in beadContext and reuses the cached value on subsequent calls
// within the same iteration, avoiding redundant git operations.
func TestAcceptance_GitDiffCachedInBeadContext(t *testing.T) {
	// Expected failure: beadContext.cachedDiff field does not exist yet
	// Expected failure: getDiff() does not check beadContext cache before calling gitDiffFn

	var gitDiffCallCount int
	mockDiff := "diff --git a/file.go b/file.go\n+new line"

	r := &Runner{
		gitDiffFn: func(fromCommit string) (string, error) {
			gitDiffCallCount++
			return mockDiff, nil
		},
	}

	// Create a beadContext with startCommit set
	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-1",
			Title:    "Test bead",
			Priority: 1,
		},
		startCommit: "abc123",
	}

	// First call to getDiff - should hit gitDiffFn
	diff1, err := r.getDiffCached(bc)
	if err != nil {
		t.Fatalf("First getDiffCached failed: %v", err)
	}
	if diff1 != mockDiff {
		t.Errorf("First getDiffCached returned wrong diff: got %q, want %q", diff1, mockDiff)
	}
	if gitDiffCallCount != 1 {
		t.Errorf("First getDiffCached: expected 1 git call, got %d", gitDiffCallCount)
	}

	// Second call to getDiff with same beadContext - should use cache
	diff2, err := r.getDiffCached(bc)
	if err != nil {
		t.Fatalf("Second getDiffCached failed: %v", err)
	}
	if diff2 != mockDiff {
		t.Errorf("Second getDiffCached returned wrong diff: got %q, want %q", diff2, mockDiff)
	}
	if gitDiffCallCount != 1 {
		t.Errorf("Second getDiffCached: expected still 1 git call (cached), got %d", gitDiffCallCount)
	}

	// Third call - verify cache is still used
	diff3, err := r.getDiffCached(bc)
	if err != nil {
		t.Fatalf("Third getDiffCached failed: %v", err)
	}
	if diff3 != mockDiff {
		t.Errorf("Third getDiffCached returned wrong diff: got %q, want %q", diff3, mockDiff)
	}
	if gitDiffCallCount != 1 {
		t.Errorf("Third getDiffCached: expected still 1 git call (cached), got %d", gitDiffCallCount)
	}
}

// TestAcceptance_GitDiffCacheClearedBetweenBeads verifies that the cached
// diff is cleared when processing a new bead, ensuring stale diff data
// from a previous bead is not reused.
func TestAcceptance_GitDiffCacheClearedBetweenBeads(t *testing.T) {
	// Expected failure: beadContext.cachedDiff field does not exist yet
	// Expected failure: setupBeadContext() does not initialize cachedDiff to nil/empty

	var gitDiffCallCount int
	mockDiff1 := "diff from bead 1"
	mockDiff2 := "diff from bead 2"
	currentDiff := mockDiff1

	r := &Runner{
		gitDiffFn: func(fromCommit string) (string, error) {
			gitDiffCallCount++
			return currentDiff, nil
		},
	}

	// First bead context
	bc1 := &beadContext{
		bead: &bead.Bead{
			ID:       "bead-1",
			Title:    "First bead",
			Priority: 1,
		},
		startCommit: "commit1",
	}

	// Call getDiff for first bead
	diff1, err := r.getDiffCached(bc1)
	if err != nil {
		t.Fatalf("getDiffCached for bead 1 failed: %v", err)
	}
	if diff1 != mockDiff1 {
		t.Errorf("Bead 1: got diff %q, want %q", diff1, mockDiff1)
	}
	if gitDiffCallCount != 1 {
		t.Errorf("Bead 1: expected 1 git call, got %d", gitDiffCallCount)
	}

	// Change what git diff returns (simulating new bead, new changes)
	currentDiff = mockDiff2

	// Second bead context (different bead)
	bc2 := &beadContext{
		bead: &bead.Bead{
			ID:       "bead-2",
			Title:    "Second bead",
			Priority: 1,
		},
		startCommit: "commit2",
	}

	// Call getDiff for second bead - should NOT use cache from bc1
	diff2, err := r.getDiffCached(bc2)
	if err != nil {
		t.Fatalf("getDiffCached for bead 2 failed: %v", err)
	}
	if diff2 != mockDiff2 {
		t.Errorf("Bead 2: got diff %q, want %q (should be fresh, not cached from bead 1)", diff2, mockDiff2)
	}
	if gitDiffCallCount != 2 {
		t.Errorf("Bead 2: expected 2 total git calls (no cross-bead caching), got %d", gitDiffCallCount)
	}

	// Verify second bead can cache within its own context
	diff2Again, err := r.getDiffCached(bc2)
	if err != nil {
		t.Fatalf("Second getDiffCached for bead 2 failed: %v", err)
	}
	if diff2Again != mockDiff2 {
		t.Errorf("Bead 2 second call: got diff %q, want %q", diff2Again, mockDiff2)
	}
	if gitDiffCallCount != 2 {
		t.Errorf("Bead 2 second call: expected still 2 total git calls (cache within bead), got %d", gitDiffCallCount)
	}
}

// TestAcceptance_AllGetDiffCallsUseCachedVersion verifies that all existing
// call sites that use getDiff() are updated to use the cached version via
// beadContext, preventing multiple git diff invocations per iteration.
func TestAcceptance_AllGetDiffCallsUseCachedVersion(t *testing.T) {
	// Expected failure: review methods don't call getDiffCached(bc)
	// Expected failure: runRefactorPhase doesn't call getDiffCached(bc)
	// Expected failure: verifyTestsFailWithRetry doesn't call getDiffCached(bc)
	// Expected failure: showPartialProgress doesn't receive cached diff as parameter

	tests := []struct {
		name        string
		operation   string
		expectCalls int // expected number of git diff calls for the operation
	}{
		{
			name:        "light review uses cached diff",
			operation:   "lightReview",
			expectCalls: 1, // Only the initial cache population
		},
		{
			name:        "thorough review uses cached diff",
			operation:   "thoroughReview",
			expectCalls: 1,
		},
		{
			name:        "refactor phase uses cached diff",
			operation:   "refactor",
			expectCalls: 1,
		},
		{
			name:        "test verification uses cached diff",
			operation:   "testVerification",
			expectCalls: 1,
		},
		{
			name:        "partial progress display uses cached diff",
			operation:   "partialProgress",
			expectCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gitDiffCallCount int
			mockDiff := "diff --git a/example.go"

			cfg := &config.Config{}
			cfg.SetDefaults()
			cfg.NormalizeNilFields()

			// Create minimal runner with mock gitDiffFn
			r := &Runner{
				cfg: cfg,
				gitDiffFn: func(fromCommit string) (string, error) {
					gitDiffCallCount++
					return mockDiff, nil
				},
				output: &strings.Builder{},
			}

			bc := &beadContext{
				bead: &bead.Bead{
					ID:       "test-bead",
					Title:    "Test",
					Priority: 1,
				},
				startCommit: "abc123",
				model:       "sonnet",
				tier:        provider.TierMedium,
			}

			ctx := context.Background()

			// Simulate the operation that should use cached diff
			switch tt.operation {
			case "lightReview":
				// Simulate light review - should use getDiffCached
				_, _ = r.getDiffCached(bc)
				_, _ = r.getDiffCached(bc) // Second call in same context
			case "thoroughReview":
				// Simulate thorough review - should use getDiffCached
				_, _ = r.getDiffCached(bc)
				_, _ = r.getDiffCached(bc)
			case "refactor":
				// Simulate refactor phase - should use getDiffCached
				_, _ = r.getDiffCached(bc)
			case "testVerification":
				// Simulate ATDD verification - should use getDiffCached
				_, _ = r.getDiffCached(bc)
			case "partialProgress":
				// Simulate partial progress display - should receive cached diff
				_, _ = r.getDiffCached(bc)
			}

			// All operations should result in exactly 1 git call due to caching
			if gitDiffCallCount != tt.expectCalls {
				t.Errorf("%s: expected %d git diff call(s), got %d (caching not working)",
					tt.operation, tt.expectCalls, gitDiffCallCount)
			}

			// Verify we can still get the cached value
			cachedDiff, err := r.getDiffCached(bc)
			if err != nil {
				t.Errorf("Failed to get cached diff: %v", err)
			}
			if cachedDiff != mockDiff {
				t.Errorf("Cached diff incorrect: got %q, want %q", cachedDiff, mockDiff)
			}
			// Still should be only 1 call total
			if gitDiffCallCount != tt.expectCalls {
				t.Errorf("After retrieving cached diff, call count changed to %d", gitDiffCallCount)
			}
		})
	}
}

// TestAcceptance_GetDiffCachedReturnsErrorFromGitDiff verifies that
// getDiffCached properly propagates errors from the underlying git diff
// operation and doesn't cache error results.
func TestAcceptance_GetDiffCachedReturnsErrorFromGitDiff(t *testing.T) {
	// Expected failure: getDiffCached method does not exist
	// Expected failure: error handling in getDiffCached not implemented

	mockErr := "git diff failed: repository not found"
	var callCount int

	r := &Runner{
		gitDiffFn: func(fromCommit string) (string, error) {
			callCount++
			return "", &gitError{msg: mockErr}
		},
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-1",
			Title:    "Test",
			Priority: 1,
		},
		startCommit: "abc123",
	}

	// First call should return error
	_, err := r.getDiffCached(bc)
	if err == nil {
		t.Fatal("Expected error from getDiffCached, got nil")
	}
	if !strings.Contains(err.Error(), mockErr) {
		t.Errorf("Expected error containing %q, got %q", mockErr, err.Error())
	}

	// Second call should also return error (errors should not be cached)
	_, err = r.getDiffCached(bc)
	if err == nil {
		t.Fatal("Expected error from second getDiffCached, got nil")
	}
	if callCount != 2 {
		t.Errorf("Expected 2 git calls (errors not cached), got %d", callCount)
	}
}

// TestAcceptance_GetDiffCachedHandlesEmptyDiff verifies that empty diffs
// are cached correctly, avoiding redundant git calls even when there are
// no changes.
func TestAcceptance_GetDiffCachedHandlesEmptyDiff(t *testing.T) {
	// Expected failure: getDiffCached does not exist
	// Expected failure: empty diff caching behavior not implemented

	var callCount int

	r := &Runner{
		gitDiffFn: func(fromCommit string) (string, error) {
			callCount++
			return "", nil // Empty diff - no changes
		},
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-1",
			Title:    "Test",
			Priority: 1,
		},
		startCommit: "abc123",
	}

	// First call
	diff1, err := r.getDiffCached(bc)
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}
	if diff1 != "" {
		t.Errorf("Expected empty diff, got %q", diff1)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}

	// Second call - empty diff should still be cached
	diff2, err := r.getDiffCached(bc)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}
	if diff2 != "" {
		t.Errorf("Expected empty diff from cache, got %q", diff2)
	}
	if callCount != 1 {
		t.Errorf("Expected still 1 call (empty diff cached), got %d", callCount)
	}
}

// TestAcceptance_ProcessBeadClearsCacheBetweenIterations verifies that
// processBead clears the diff cache when starting a new bead, preventing
// cache pollution across different work items in a run loop.
func TestAcceptance_ProcessBeadClearsCacheBetweenIterations(t *testing.T) {
	// Expected failure: setupBeadContext() doesn't initialize cachedDiff field
	// Expected failure: beadContext struct lacks cachedDiff field

	var gitDiffCallCount int
	currentDiff := "initial diff"

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	mockBeadClient := &mockBeadClient{
		ShowFn: func(id string) (*bead.Bead, error) {
			return &bead.Bead{
				ID:       id,
				Title:    "Test bead",
				Priority: 1,
			}, nil
		},
		GetParentFn: func(b *bead.Bead) (*bead.Bead, error) {
			return nil, nil
		},
	}

	r := &Runner{
		cfg:   cfg,
		beads: mockBeadClient,
		gitDiffFn: func(fromCommit string) (string, error) {
			gitDiffCallCount++
			return currentDiff, nil
		},
		output: &strings.Builder{},
	}

	// Simulate first iteration's setupBeadContext
	bc1, _, cancel1, err := r.setupBeadContext(
		context.Background(),
		&bead.Bead{ID: "bead-1", Title: "First", Priority: 1},
		1,
		time.Now().Add(1*time.Hour),
		nil,
	)
	if cancel1 != nil {
		defer cancel1()
	}
	if err != nil {
		t.Fatalf("setupBeadContext for bead 1 failed: %v", err)
	}

	// Get diff for first bead - should populate cache
	diff1, _ := r.getDiffCached(bc1)
	if gitDiffCallCount != 1 {
		t.Errorf("First bead: expected 1 git call, got %d", gitDiffCallCount)
	}

	// Change what git returns (simulating new bead with different changes)
	currentDiff = "second diff"

	// Simulate second iteration's setupBeadContext - should create fresh context
	bc2, _, cancel2, err := r.setupBeadContext(
		context.Background(),
		&bead.Bead{ID: "bead-2", Title: "Second", Priority: 1},
		2,
		time.Now().Add(1*time.Hour),
		nil,
	)
	if cancel2 != nil {
		defer cancel2()
	}
	if err != nil {
		t.Fatalf("setupBeadContext for bead 2 failed: %v", err)
	}

	// Get diff for second bead - should call git again (not use bc1's cache)
	diff2, _ := r.getDiffCached(bc2)
	if gitDiffCallCount != 2 {
		t.Errorf("Second bead: expected 2 total git calls (cache cleared), got %d", gitDiffCallCount)
	}
	if diff2 == diff1 {
		t.Errorf("Second bead returned same diff as first (cache not cleared)")
	}
	if diff2 != "second diff" {
		t.Errorf("Second bead: got %q, want %q", diff2, "second diff")
	}
}

// gitError is a helper type for testing error handling
type gitError struct {
	msg string
}

func (e *gitError) Error() string {
	return e.msg
}
