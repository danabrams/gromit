package specloop

import (
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_LineageChainResolution_DepthGreaterThan2(t *testing.T) {
	// Seed: build a TaskLineage map representing:
	//   t-028 has fixes: "t-015"
	//   t-015 has fixes: "t-001"
	//   t-001 has no fixes (root)
	//
	// The ChainIDs reflect how UpdateTaskLineage builds chains:
	//   t-001 is a root task → ChainIDs: ["t-001"]
	//   t-015 fixes t-001  → ChainIDs: ["t-001", "t-015"]
	//   t-028 fixes t-015  → ChainIDs: ["t-001", "t-015", "t-028"]
	lineage := map[string]runstore.TaskLineageEntry{
		"t-001": {ChainIDs: []string{"t-001"}},
		"t-015": {ChainIDs: []string{"t-001", "t-015"}},
		"t-028": {ChainIDs: []string{"t-001", "t-015", "t-028"}},
	}

	// Invoke
	root := resolveLineageRoot(lineage, "t-028")

	// Assert: chain traversal t-028 → t-015 → t-001 (no fixes) → root found
	if root != "t-001" {
		t.Errorf("resolveLineageRoot(t-028) = %q, want %q", root, "t-001")
	}
}

func TestScenario_LineageChainResolution_DepthGreaterThan2_ViaUpdateTaskLineage(t *testing.T) {
	// Seed: simulate the actual UpdateTaskLineage flow that produces a depth-3 chain.
	// After Issue 3 (no mirror entries), only root-keyed entries are maintained.
	// t-001 is the root; t-015 and t-028 appear in t-001's ChainIDs.
	lineage := make(map[string]runstore.TaskLineageEntry)

	// Step 1: t-001 fails (root task, no fixes)
	tasks1 := []runstore.Task{
		{TaskID: "t-001"},
	}
	UpdateTaskLineage(lineage, tasks1, []string{"t-001"})

	// Step 2: t-015 fixes t-001 and also fails
	tasks2 := []runstore.Task{
		{TaskID: "t-001"},
		{TaskID: "t-015", Fixes: "t-001"},
	}
	UpdateTaskLineage(lineage, tasks2, []string{"t-015"})

	// Step 3: t-028 fixes t-015 and also fails
	tasks3 := []runstore.Task{
		{TaskID: "t-001"},
		{TaskID: "t-015", Fixes: "t-001"},
		{TaskID: "t-028", Fixes: "t-015"},
	}
	UpdateTaskLineage(lineage, tasks3, []string{"t-028"})

	// Verify the chain structure built by UpdateTaskLineage.
	// After Issue 3, only the root entry (t-001) is in the lineage map.
	// t-015 and t-028 appear in t-001's ChainIDs.
	rootEntry, exists := lineage["t-001"]
	if !exists {
		t.Fatal("expected root lineage entry for t-001")
	}
	if len(rootEntry.ChainIDs) < 3 {
		t.Fatalf("expected chain length >= 3 (t-001, t-015, t-028), got %d: %v", len(rootEntry.ChainIDs), rootEntry.ChainIDs)
	}
	if rootEntry.ChainIDs[0] != "t-001" {
		t.Errorf("chain root = %q, want t-001; full chain: %v", rootEntry.ChainIDs[0], rootEntry.ChainIDs)
	}
	// Verify t-028 is in the chain
	hasT028 := false
	for _, chainID := range rootEntry.ChainIDs {
		if chainID == "t-028" {
			hasT028 = true
			break
		}
	}
	if !hasT028 {
		t.Errorf("t-028 not found in t-001 ChainIDs: %v", rootEntry.ChainIDs)
	}
}