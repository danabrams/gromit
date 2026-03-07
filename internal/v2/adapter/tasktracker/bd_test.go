package tasktracker

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestNextBead_ReturnsBeadFromClient(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	client := &bead.Client{
		RunFn: func(args ...string) (string, error) {
			if len(args) == 0 || args[0] != "ready" {
				return "", fmt.Errorf("unexpected command: %v", args)
			}
			return `[{
                "id": "bead-1",
                "title": "Next Bead",
                "description": "description",
                "priority": 2,
                "status": "open",
                "labels": ["spec:test"],
                "depends_on": [{"id": "parent"}],
                "blocked_by": [{"id": "dep"}]
            }]`, nil
		},
	}

	adapter := NewBDAdapter(client)
	resp, err := adapter.NextBead(ctx, NextBeadRequest{})
	if err != nil {
		t.Fatalf("NextBead failed: %v", err)
	}
	if resp == nil || resp.Bead == nil {
		t.Fatal("NextBead returned nil bead")
	}
	if resp.Bead.ID != "bead-1" {
		t.Fatalf("unexpected bead ID: %s", resp.Bead.ID)
	}
	if !containsString(resp.Bead.DependsOn, "parent") {
		t.Fatalf("missing depends_on: %v", resp.Bead.DependsOn)
	}
	if !containsString(resp.Bead.BlockedBy, "dep") {
		t.Fatalf("missing blocked_by: %v", resp.Bead.BlockedBy)
	}
}

func TestCreateBead_UsesBdCreateCommand(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var recordedArgs []string
	client := &bead.Client{
		RunFn: func(args ...string) (string, error) {
			recordedArgs = append(recordedArgs, args...)
			if len(args) == 0 || args[0] != "create" {
				return "", fmt.Errorf("unexpected command: %v", args)
			}
			return `{
				"id": "created-bead",
				"title": "Created",
				"description": "desc",
				"priority": 1,
				"status": "open",
				"labels": ["alpha"],
				"depends_on": [{"id": "dep-1"}],
				"blocked_by": []
			}`, nil
		},
	}

	adapter := NewBDAdapter(client)
	resp, err := adapter.CreateBead(ctx, CreateBeadRequest{
		Title:        "Created",
		Description:  "desc",
		Priority:     1,
		Labels:       []string{"alpha"},
		Dependencies: []string{"dep-1"},
	})
	if err != nil {
		t.Fatalf("CreateBead failed: %v", err)
	}
	if resp == nil || resp.Bead == nil {
		t.Fatal("CreateBead returned nil bead")
	}
	if resp.Bead.ID != "created-bead" {
		t.Fatalf("unexpected bead ID: %s", resp.Bead.ID)
	}
	if !containsString(recordedArgs, "--deps") || !containsString(recordedArgs, "dep-1") {
		t.Fatalf("deps flags missing from args: %v", recordedArgs)
	}
	if !containsString(recordedArgs, "--label") || !containsString(recordedArgs, "alpha") {
		t.Fatalf("labels missing from args: %v", recordedArgs)
	}
	if !strings.Contains(strings.Join(recordedArgs, " "), "--body-file") {
		t.Fatalf("expected --body-file in args: %v", recordedArgs)
	}
}

func TestShowBead_UsesBdShowCommand(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var recordedArgs []string
	client := &bead.Client{
		RunFn: func(args ...string) (string, error) {
			recordedArgs = append(recordedArgs, args...)
			if len(args) < 3 || args[0] != "show" {
				return "", fmt.Errorf("unexpected command: %v", args)
			}
			return `{
				"id": "shown-bead",
				"title": "Shown",
				"description": "desc",
				"priority": 2,
				"status": "open",
				"labels": ["alpha"],
				"blocked_by": [{"id": "back"}],
				"depends_on": [{"id": "front"}]
			}`, nil
		},
	}

	adapter := NewBDAdapter(client)
	resp, err := adapter.ShowBead(ctx, "shown-bead")
	if err != nil {
		t.Fatalf("ShowBead failed: %v", err)
	}
	if resp == nil {
		t.Fatal("ShowBead returned nil bead")
	}
	if resp.ID != "shown-bead" {
		t.Fatalf("unexpected bead ID: %s", resp.ID)
	}
	if !containsString(resp.DependsOn, "front") {
		t.Fatalf("missing depends_on: %v", resp.DependsOn)
	}
	if !containsString(resp.BlockedBy, "back") {
		t.Fatalf("missing blocked_by: %v", resp.BlockedBy)
	}
	if !containsString(recordedArgs, "shown-bead") || !containsString(recordedArgs, "--json") {
		t.Fatalf("unexpected show args: %v", recordedArgs)
	}
}

func TestCloseBead_UsesBdCloseCommand(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	beadID := "close-me"
	called := false
	client := &bead.Client{
		RunFn: func(args ...string) (string, error) {
			if len(args) < 2 || args[0] != "close" {
				return "", fmt.Errorf("unexpected command: %v", args)
			}
			if args[1] != beadID {
				return "", fmt.Errorf("unexpected bead ID: %v", args)
			}
			called = true
			return "", nil
		},
	}

	adapter := NewBDAdapter(client)
	resp, err := adapter.CloseBead(ctx, CloseBeadRequest{BeadID: beadID})
	if err != nil {
		t.Fatalf("CloseBead failed: %v", err)
	}
	if resp == nil || !resp.Closed {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !called {
		t.Fatal("bd close was not invoked")
	}
}

func TestQueryBeads_FiltersByLabelsAndParent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var recordedArgs []string
	client := &bead.Client{
		RunFn: func(args ...string) (string, error) {
			recordedArgs = append(recordedArgs, args...)
			if len(args) == 0 || args[0] != "list" {
				return "", fmt.Errorf("unexpected command: %v", args)
			}
			return `[
				{"id": "keep", "title": "Keep", "status": "open", "parent": "parent-1", "labels": ["alpha"], "depends_on": [], "blocked_by": []},
				{"id": "skip", "title": "Skip", "status": "open", "parent": "parent-2", "labels": ["beta"], "depends_on": [], "blocked_by": []}
			]`, nil
		},
	}

	adapter := NewBDAdapter(client)
	resp, err := adapter.QueryBeads(ctx, QueryBeadsRequest{
		Status: "open",
		Labels: []string{"alpha"},
		Parent: "parent-1",
	})
	if err != nil {
		t.Fatalf("QueryBeads failed: %v", err)
	}
	if resp == nil || len(resp.Beads) != 1 {
		t.Fatalf("unexpected result: %+v", resp)
	}
	if resp.Beads[0].ID != "keep" {
		t.Fatalf("unexpected bead returned: %s", resp.Beads[0].ID)
	}
	if !containsString(recordedArgs, "--status") {
		t.Fatalf("status flag missing: %v", recordedArgs)
	}
	if !containsString(recordedArgs, "open") {
		t.Fatalf("status value missing: %v", recordedArgs)
	}
}

func TestConvertBead_DependenciesWithBlocksType_MapsToBlockedBy(t *testing.T) {
	t.Parallel()

	b := &bead.Bead{
		ID:     "test-bead",
		Title:  "Test",
		Status: "open",
		Dependencies: []bead.Dependency{
			{ID: "dep-1", DependencyType: "blocks"},
			{ID: "dep-2", DependencyType: "depends"},
		},
	}

	result := convertBead(b)
	if result == nil {
		t.Fatal("convertBead returned nil")
	}
	if !containsString(result.BlockedBy, "dep-1") {
		t.Fatalf("BlockedBy should contain dep-1, got %v", result.BlockedBy)
	}
	if containsString(result.BlockedBy, "dep-2") {
		t.Fatalf("BlockedBy should not contain dep-2, got %v", result.BlockedBy)
	}
	if !containsString(result.Dependents, "dep-2") {
		t.Fatalf("Dependents should contain dep-2, got %v", result.Dependents)
	}
	if containsString(result.Dependents, "dep-1") {
		t.Fatalf("Dependents should not contain dep-1, got %v", result.Dependents)
	}
	if len(result.DependsOn) != 0 {
		t.Fatalf("DependsOn should be empty, got %v", result.DependsOn)
	}
}

func TestConvertBead_RealBDFormat_MapsBlockersToBlockedBy(t *testing.T) {
	t.Parallel()

	// Simulate real bd JSON output where dependencies with dependency_type "blocks"
	// represent beads that block the current bead (prerequisites).
	b := &bead.Bead{
		ID:     "gromit-b5p6k",
		Title:  "Implement feature",
		Status: "open",
		Dependencies: []bead.Dependency{
			{ID: "gromit-ns5fp", DependencyType: "blocks", Title: "Prerequisite 1", Status: "open"},
			{ID: "gromit-ziybi", DependencyType: "blocks", Title: "Prerequisite 2", Status: "open"},
		},
	}

	result := convertBead(b)
	if result == nil {
		t.Fatal("convertBead returned nil")
	}
	if len(result.BlockedBy) != 2 {
		t.Fatalf("BlockedBy should have 2 entries, got %v", result.BlockedBy)
	}
	if !containsString(result.BlockedBy, "gromit-ns5fp") {
		t.Fatalf("BlockedBy missing gromit-ns5fp, got %v", result.BlockedBy)
	}
	if !containsString(result.BlockedBy, "gromit-ziybi") {
		t.Fatalf("BlockedBy missing gromit-ziybi, got %v", result.BlockedBy)
	}
	if len(result.Dependents) != 0 {
		t.Fatalf("Dependents should be empty for blocks-only deps, got %v", result.Dependents)
	}
}

func TestConvertBead_BlockedByMergesExplicitAndDependencies(t *testing.T) {
	t.Parallel()

	// When both b.BlockedBy and b.Dependencies contain blockers, they should merge.
	b := &bead.Bead{
		ID:     "merge-test",
		Title:  "Merge test",
		Status: "open",
		BlockedBy: []bead.Dependency{
			{ID: "explicit-blocker"},
		},
		Dependencies: []bead.Dependency{
			{ID: "dep-blocker", DependencyType: "blocks"},
		},
	}

	result := convertBead(b)
	if result == nil {
		t.Fatal("convertBead returned nil")
	}
	if !containsString(result.BlockedBy, "explicit-blocker") {
		t.Fatalf("BlockedBy missing explicit-blocker, got %v", result.BlockedBy)
	}
	if !containsString(result.BlockedBy, "dep-blocker") {
		t.Fatalf("BlockedBy missing dep-blocker, got %v", result.BlockedBy)
	}
}

func TestConvertBead_DuplicateIDsInBlockedByAndDependencies_Deduplicated(t *testing.T) {
	t.Parallel()

	// Same bead ID appears in both BlockedBy and Dependencies with type "blocks".
	// The merged BlockedBy slice should contain each ID only once.
	b := &bead.Bead{
		ID:     "dedup-test",
		Title:  "Dedup test",
		Status: "open",
		BlockedBy: []bead.Dependency{
			{ID: "shared-blocker"},
			{ID: "only-explicit"},
		},
		Dependencies: []bead.Dependency{
			{ID: "shared-blocker", DependencyType: "blocks"},
			{ID: "only-dep-blocker", DependencyType: "blocks"},
		},
	}

	result := convertBead(b)
	if result == nil {
		t.Fatal("convertBead returned nil")
	}
	if len(result.BlockedBy) != 3 {
		t.Fatalf("BlockedBy should have 3 unique entries, got %d: %v", len(result.BlockedBy), result.BlockedBy)
	}
	for _, expected := range []string{"shared-blocker", "only-explicit", "only-dep-blocker"} {
		if !containsString(result.BlockedBy, expected) {
			t.Fatalf("BlockedBy missing %s, got %v", expected, result.BlockedBy)
		}
	}
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
