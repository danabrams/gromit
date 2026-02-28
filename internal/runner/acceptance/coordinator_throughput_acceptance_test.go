//go:build acceptance

package acceptance_test

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/integrationqueue"
)

// TestCoordinatorThroughputWithTerminalFailures verifies that the coordinator
// processes subsequent ready entries even when earlier entries fail with terminal failures.
// This acceptance test ensures FIFO progression is not blocked by conflict/failed_gates/lane_violation.
func TestCoordinatorThroughputWithTerminalFailures(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := integrationqueue.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Create queue with 4 entries:
	// 1. Will have merge conflict (terminal failure)
	// 2. Will succeed
	// 3. Will have lane violation (terminal failure)
	// 4. Will succeed

	conflictEntry := integrationqueue.Entry{
		Branch:           "feature/conflict-entry",
		SessionID:        "session-conflict",
		OriginCommand:    "refine",
		State:            integrationqueue.StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadbeef",
		ChangedFilesHash: "hash1",
	}
	if err := store.Save(conflictEntry); err != nil {
		t.Fatalf("Save(conflictEntry) error = %v", err)
	}

	successEntry1 := integrationqueue.Entry{
		Branch:           "feature/success-1",
		SessionID:        "session-success-1",
		OriginCommand:    "refine",
		State:            integrationqueue.StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "cafebabe",
		ChangedFilesHash: "hash2",
	}
	if err := store.Save(successEntry1); err != nil {
		t.Fatalf("Save(successEntry1) error = %v", err)
	}

	laneViolationEntry := integrationqueue.Entry{
		Branch:           "feature/lane-violation",
		SessionID:        "session-lane-violation",
		OriginCommand:    "refine",
		State:            integrationqueue.StateReady,
		Lane:             "safe_lane",
		BaseRef:          "main",
		HeadSHA:          "beefdead",
		ChangedFilesHash: "hash3",
	}
	if err := store.Save(laneViolationEntry); err != nil {
		t.Fatalf("Save(laneViolationEntry) error = %v", err)
	}

	successEntry2 := integrationqueue.Entry{
		Branch:           "feature/success-2",
		SessionID:        "session-success-2",
		OriginCommand:    "refine",
		State:            integrationqueue.StateReady,
		Lane:             "code_lane",
		BaseRef:          "main",
		HeadSHA:          "deadc0de",
		ChangedFilesHash: "hash4",
	}
	if err := store.Save(successEntry2); err != nil {
		t.Fatalf("Save(successEntry2) error = %v", err)
	}

	// Create coordinator with selective mock that causes failures
	gitops := &throughputTestMockGitOps{}
	gate := &throughputTestMockGate{}
	coord := integrationqueue.NewCoordinator(store, gitops, gate)

	// First Coordinate call should process conflict entry and success entry 1
	err = coord.Coordinate(ctx)
	if err != nil {
		t.Fatalf("First Coordinate() error = %v, want nil", err)
	}

	payload, err := store.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() after first Coordinate error = %v", err)
	}

	// Verify first entry is in conflict state
	conflict := findEntry(payload.Entries, "feature/conflict-entry")
	if conflict == nil {
		t.Fatal("conflict entry not found")
	}
	if conflict.State != integrationqueue.StateConflict {
		t.Fatalf("conflict entry state = %q, want %q", conflict.State, integrationqueue.StateConflict)
	}

	// Verify second entry is merged
	success1 := findEntry(payload.Entries, "feature/success-1")
	if success1 == nil {
		t.Fatal("success-1 entry not found")
	}
	if success1.State != integrationqueue.StateMerged {
		t.Fatalf("success-1 entry state = %q, want %q", success1.State, integrationqueue.StateMerged)
	}

	// Second Coordinate call should process lane violation entry and success entry 2
	err = coord.Coordinate(ctx)
	if err != nil {
		t.Fatalf("Second Coordinate() error = %v, want nil", err)
	}

	payload, err = store.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() after second Coordinate error = %v", err)
	}

	// Verify third entry is in lane violation state
	laneViolation := findEntry(payload.Entries, "feature/lane-violation")
	if laneViolation == nil {
		t.Fatal("lane violation entry not found")
	}
	if laneViolation.State != integrationqueue.StateLaneViolation {
		t.Fatalf("lane violation entry state = %q, want %q", laneViolation.State, integrationqueue.StateLaneViolation)
	}

	// Verify fourth entry is merged
	success2 := findEntry(payload.Entries, "feature/success-2")
	if success2 == nil {
		t.Fatal("success-2 entry not found")
	}
	if success2.State != integrationqueue.StateMerged {
		t.Fatalf("success-2 entry state = %q, want %q", success2.State, integrationqueue.StateMerged)
	}
}

// throughputTestMockGitOps causes conflict on one branch and lane violation on another
type throughputTestMockGitOps struct{}

func (m *throughputTestMockGitOps) FetchAndRebase(ctx context.Context, entry integrationqueue.Entry) error {
	return nil
}

func (m *throughputTestMockGitOps) MergeToMain(ctx context.Context, entry integrationqueue.Entry) error {
	if entry.Branch == "feature/conflict-entry" {
		return &mergeConflictError{"merge conflict"}
	}
	if entry.Branch == "feature/lane-violation" {
		return &laneViolationError{"lane violation: cannot merge safe_lane with code changes"}
	}
	return nil
}

func (m *throughputTestMockGitOps) Push(ctx context.Context) error {
	return nil
}

func (m *throughputTestMockGitOps) Cleanup(ctx context.Context, entry integrationqueue.Entry) error {
	return nil
}

type mergeConflictError struct {
	msg string
}

func (e *mergeConflictError) Error() string {
	return e.msg
}

type laneViolationError struct {
	msg string
}

func (e *laneViolationError) Error() string {
	return e.msg
}

// throughputTestMockGate always succeeds
type throughputTestMockGate struct{}

func (g *throughputTestMockGate) Run(ctx context.Context, entry integrationqueue.Entry) error {
	return nil
}

func findEntry(entries []integrationqueue.Entry, branch string) *integrationqueue.Entry {
	for i := range entries {
		if entries[i].Branch == branch {
			return &entries[i]
		}
	}
	return nil
}
