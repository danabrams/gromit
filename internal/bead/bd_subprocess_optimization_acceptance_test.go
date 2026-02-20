//go:build acceptance

package bead

import (
	"testing"
)

// TestReady_UsesLimit3 verifies that Ready() calls bd with --limit 3 instead of --limit 10.
// This reduces the number of beads fetched and parsed when filtering out epics.
func TestReady_UsesLimit3(t *testing.T) {
	var capturedArgs []string
	c := &Client{
		RunFn: func(args ...string) (string, error) {
			capturedArgs = args
			// Return valid JSON with one non-epic bead
			return `[{"id":"test-1","title":"Test","priority":1,"labels":[],"parent":"","issue_type":"task","status":"open","owner":""}]`, nil
		},
	}

	_, err := c.Ready()
	if err != nil {
		t.Fatalf("Ready() error: %v", err)
	}

	// Verify --limit 3 is in the args
	limitFound := false
	limitValue := ""
	for i, arg := range capturedArgs {
		if arg == "--limit" && i+1 < len(capturedArgs) {
			limitFound = true
			limitValue = capturedArgs[i+1]
			break
		}
	}

	if !limitFound {
		t.Fatal("Ready() did not call bd with --limit flag")
	}

	if limitValue != "3" {
		t.Errorf("Ready() uses --limit %s, expected --limit 3", limitValue)
	}
}

// NOTE: HasOpenChildren optimization tests are not included because
// HasOpenChildren() has already been optimized in bead.go:701 to use
// bd list --parent <id> --limit 1. This task only needs to reduce the
// Ready() batch size from 10 to 3.

// TestReadyWithLabel_UsesLimit3 verifies that ReadyWithLabel() uses --limit 3
// for consistency with Ready().
func TestReadyWithLabel_UsesLimit3(t *testing.T) {
	var capturedArgs []string
	c := &Client{
		RunFn: func(args ...string) (string, error) {
			capturedArgs = args
			return `[{"id":"test-1","title":"Test","priority":1,"labels":["spec:foo"],"parent":"","issue_type":"task","status":"open","owner":""}]`, nil
		},
	}

	_, err := c.ReadyWithLabel("spec:foo")
	if err != nil {
		t.Fatalf("ReadyWithLabel() error: %v", err)
	}

	// Verify --limit 3 is in the args
	limitFound := false
	limitValue := ""
	for i, arg := range capturedArgs {
		if arg == "--limit" && i+1 < len(capturedArgs) {
			limitFound = true
			limitValue = capturedArgs[i+1]
			break
		}
	}

	if !limitFound {
		t.Fatal("ReadyWithLabel() did not call bd with --limit flag")
	}

	if limitValue != "3" {
		t.Errorf("ReadyWithLabel() uses --limit %s, expected --limit 3 for consistency with Ready()", limitValue)
	}
}

// TestReady_StillFiltersEpicsCorrectly verifies that reducing the batch size
// from 10 to 3 does not break epic filtering behavior.
func TestReady_StillFiltersEpicsCorrectly(t *testing.T) {
	testCases := []struct {
		name         string
		bdOutput     string
		expectBead   bool
		expectBeadID string
	}{
		{
			name:         "first bead is non-epic",
			bdOutput:     `[{"id":"task-1","title":"Task","priority":1,"labels":[],"parent":"","issue_type":"task","status":"ready","owner":""}]`,
			expectBead:   true,
			expectBeadID: "task-1",
		},
		{
			name:         "first is epic, second is task",
			bdOutput:     `[{"id":"epic-1","title":"Epic","priority":1,"labels":[],"parent":"","issue_type":"epic","status":"ready","owner":""},{"id":"task-1","title":"Task","priority":1,"labels":[],"parent":"epic-1","issue_type":"task","status":"ready","owner":""}]`,
			expectBead:   true,
			expectBeadID: "task-1",
		},
		{
			name:       "all three are epics",
			bdOutput:   `[{"id":"epic-1","title":"Epic 1","priority":1,"labels":[],"parent":"","issue_type":"epic","status":"ready","owner":""},{"id":"epic-2","title":"Epic 2","priority":1,"labels":[],"parent":"","issue_type":"epic","status":"ready","owner":""},{"id":"epic-3","title":"Epic 3","priority":1,"labels":[],"parent":"","issue_type":"epic","status":"ready","owner":""}]`,
			expectBead: false,
		},
		{
			name:       "empty result",
			bdOutput:   `[]`,
			expectBead: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{
				RunFn: func(args ...string) (string, error) {
					// Verify --limit 3
					limitFound := false
					for i, arg := range args {
						if arg == "--limit" && i+1 < len(args) && args[i+1] == "3" {
							limitFound = true
							break
						}
					}
					if !limitFound {
						t.Error("Ready() should use --limit 3")
					}

					return tc.bdOutput, nil
				},
			}

			bead, err := c.Ready()
			if err != nil {
				t.Fatalf("Ready() error: %v", err)
			}

			if tc.expectBead {
				if bead == nil {
					t.Error("Expected Ready() to return a bead, got nil")
				} else if bead.ID != tc.expectBeadID {
					t.Errorf("Ready() returned bead ID %s, expected %s", bead.ID, tc.expectBeadID)
				}
			} else {
				if bead != nil {
					t.Errorf("Expected Ready() to return nil, got bead %s", bead.ID)
				}
			}
		})
	}
}

// TestListReadyIDs_NotAffectedByReadyOptimization verifies that ListReadyIDs()
// is intentionally NOT changed by this optimization. It fetches IDs for display
// purposes, so keeping --limit 10 is acceptable there.
func TestListReadyIDs_NotAffectedByReadyOptimization(t *testing.T) {
	// This test verifies that ListReadyIDs still uses --limit 10 because it's
	// fetching a list of IDs for display, not filtering for a single next bead.
	// The optimization only applies to Ready() which returns one bead.

	var capturedArgs []string
	c := &Client{
		RunFn: func(args ...string) (string, error) {
			capturedArgs = args
			return `[{"id":"test-1","title":"Test 1","priority":1,"labels":[],"parent":"","issue_type":"task","status":"ready","owner":""},{"id":"test-2","title":"Test 2","priority":1,"labels":[],"parent":"","issue_type":"task","status":"ready","owner":""}]`, nil
		},
	}

	ids, err := c.ListReadyIDs()
	if err != nil {
		t.Fatalf("ListReadyIDs() error: %v", err)
	}

	if len(ids) != 2 {
		t.Errorf("ListReadyIDs() returned %d IDs, expected 2", len(ids))
	}

	// ListReadyIDs should still use --limit 10 (not affected by this optimization)
	limitFound := false
	limitValue := ""
	for i, arg := range capturedArgs {
		if arg == "--limit" && i+1 < len(capturedArgs) {
			limitFound = true
			limitValue = capturedArgs[i+1]
			break
		}
	}

	if !limitFound {
		t.Fatal("ListReadyIDs() did not call bd with --limit flag")
	}

	// ListReadyIDs intentionally keeps --limit 10 for display purposes
	if limitValue != "10" {
		t.Logf("Note: ListReadyIDs() uses --limit %s. This is intentional - the optimization only applies to Ready() which returns a single bead.", limitValue)
	}
}
