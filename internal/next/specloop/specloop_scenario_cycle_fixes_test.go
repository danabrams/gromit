package specloop

import (
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_CycleInFixesChain(t *testing.T) {
	// Seed: t-002 has fixes: "t-001" and t-001 has fixes: "t-002" (a cycle).
	// Their ChainIDs reflect the mutual reference.
	lineage := map[string]runstore.TaskLineageEntry{
		"t-001": {ChainIDs: []string{"t-002"}, ConsecutiveFails: 1},
		"t-002": {ChainIDs: []string{"t-001"}, ConsecutiveFails: 1},
	}

	// Invoke: resolveLineageRoot for t-002 must return without infinite looping.
	done := make(chan string, 1)
	go func() {
		done <- resolveLineageRoot(lineage, "t-002")
	}()

	select {
	case root := <-done:
		// Assert: returns one of the two tasks (max-depth guard breaks the cycle)
		if root != "t-001" && root != "t-002" {
			t.Fatalf("expected root to be t-001 or t-002, got %q", root)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("resolveLineageRoot did not return within 2s — likely infinite loop")
	}

	// Also verify that UpdateTaskLineage handles the cyclic fixes gracefully
	// and creates lineage entries normally.
	// After Issue 3 (no mirror entries), only root-keyed entries are maintained.
	// With a cycle, one task becomes the root and the other appears in ChainIDs.
	tasks := []runstore.Task{
		{TaskID: "t-001", Fixes: "t-002", Status: "failed"},
		{TaskID: "t-002", Fixes: "t-001", Status: "failed"},
	}

	freshLineage := make(map[string]runstore.TaskLineageEntry)
	UpdateTaskLineage(freshLineage, tasks, []string{"t-001", "t-002"})

	// With cyclic fixes, exactly one root entry should be created.
	// Both task IDs should appear somewhere (root key + ChainIDs).
	if len(freshLineage) == 0 {
		t.Fatal("expected at least one lineage entry after cyclic failures")
	}

	// Find which entry is the root
	var rootEntry runstore.TaskLineageEntry
	var rootID string
	for id, entry := range freshLineage {
		rootID = id
		rootEntry = entry
		break
	}
	if rootEntry.ConsecutiveFails != 1 {
		t.Errorf("root (%s): expected ConsecutiveFails=1, got %d", rootID, rootEntry.ConsecutiveFails)
	}
	if len(rootEntry.ChainIDs) == 0 {
		t.Errorf("root (%s): expected non-empty ChainIDs", rootID)
	}
}
