//go:build acceptance

package bead

import (
	"strings"
	"testing"
)

// TestHasOpenChildrenUsesParentFlag tests that HasOpenChildren uses --parent flag
// to filter children server-side instead of fetching all open beads.
// Expected failure: HasOpenChildren currently calls c.List() which fetches all open beads
// and filters client-side. After implementation, it should call c.run() with --parent flag.
func TestHasOpenChildrenUsesParentFlag(t *testing.T) {
	c := newIsolatedClient(t)

	// Create a parent bead
	parent, err := c.Create("Parent epic", 1, []string{}, []string{})
	if err != nil {
		t.Skipf("Cannot create parent bead: %v", err)
	}

	// Create a child bead with the parent
	_, err = c.CreateWithParent("Child task", 1, []string{}, []string{}, parent.ID)
	if err != nil {
		t.Skipf("Cannot create child bead: %v", err)
	}

	// Test that HasOpenChildren returns true for parent with open children
	hasChildren, err := c.HasOpenChildren(parent.ID)
	if err != nil {
		t.Fatalf("HasOpenChildren(%q) error = %v", parent.ID, err)
	}
	if !hasChildren {
		t.Errorf("HasOpenChildren(%q) = false, want true (parent has open child)", parent.ID)
	}
}

// TestHasOpenChildrenReturnsTrue tests that HasOpenChildren returns true when parent has open children.
// Expected failure: After implementation, this verifies the --parent flag correctly identifies
// the existence of open children with limit 1 optimization.
func TestHasOpenChildrenReturnsTrue(t *testing.T) {
	c := newIsolatedClient(t)

	// Create parent epic
	parent, err := c.Create("Epic with children", 1, []string{}, []string{})
	if err != nil {
		t.Skipf("Cannot create parent bead: %v", err)
	}

	// Create multiple children (should only need to find one)
	for i := 1; i <= 3; i++ {
		_, err = c.CreateWithParent("Child task", 1, []string{}, []string{}, parent.ID)
		if err != nil {
			t.Skipf("Cannot create child bead %d: %v", i, err)
		}
	}

	// HasOpenChildren should return true by finding at least one child
	hasChildren, err := c.HasOpenChildren(parent.ID)
	if err != nil {
		t.Fatalf("HasOpenChildren(%q) error = %v", parent.ID, err)
	}
	if !hasChildren {
		t.Errorf("HasOpenChildren(%q) = false, want true (parent has 3 open children)", parent.ID)
	}
}

// TestHasOpenChildrenReturnsFalse tests that HasOpenChildren returns false when parent has no open children.
// Expected failure: After implementation with --parent flag, this should correctly return false
// when the filtered query returns empty results.
func TestHasOpenChildrenReturnsFalse(t *testing.T) {
	c := newIsolatedClient(t)

	// Create parent without children
	parent, err := c.Create("Epic without children", 1, []string{}, []string{})
	if err != nil {
		t.Skipf("Cannot create parent bead: %v", err)
	}

	// HasOpenChildren should return false
	hasChildren, err := c.HasOpenChildren(parent.ID)
	if err != nil {
		t.Fatalf("HasOpenChildren(%q) error = %v", parent.ID, err)
	}
	if hasChildren {
		t.Errorf("HasOpenChildren(%q) = true, want false (parent has no children)", parent.ID)
	}
}

// TestHasOpenChildrenReturnsFalseForClosedChildren tests that HasOpenChildren returns false
// when parent only has closed children (not open ones).
// Expected failure: After implementation, the --status open --parent filter should correctly
// exclude closed children from the count.
func TestHasOpenChildrenReturnsFalseForClosedChildren(t *testing.T) {
	c := newIsolatedClient(t)

	// Create parent epic
	parent, err := c.Create("Epic with closed children", 1, []string{}, []string{})
	if err != nil {
		t.Skipf("Cannot create parent bead: %v", err)
	}

	// Create a child and close it
	child, err := c.CreateWithParent("Completed child", 1, []string{}, []string{}, parent.ID)
	if err != nil {
		t.Skipf("Cannot create child bead: %v", err)
	}

	err = c.Close(child.ID)
	if err != nil {
		t.Skipf("Cannot close child bead: %v", err)
	}

	// HasOpenChildren should return false because the only child is closed
	hasChildren, err := c.HasOpenChildren(parent.ID)
	if err != nil {
		t.Fatalf("HasOpenChildren(%q) error = %v", parent.ID, err)
	}
	if hasChildren {
		t.Errorf("HasOpenChildren(%q) = true, want false (only child is closed)", parent.ID)
	}
}

// TestHasOpenChildrenIgnoresUnrelatedBeads tests that HasOpenChildren only counts
// children of the specified parent, not other open beads in the system.
// Expected failure: After implementation with --parent flag, the server-side filtering
// should ensure only the specified parent's children are considered.
func TestHasOpenChildrenIgnoresUnrelatedBeads(t *testing.T) {
	c := newIsolatedClient(t)

	// Create first parent without children
	parent1, err := c.Create("Parent 1 without children", 1, []string{}, []string{})
	if err != nil {
		t.Skipf("Cannot create parent 1: %v", err)
	}

	// Create second parent with children
	parent2, err := c.Create("Parent 2 with children", 1, []string{}, []string{})
	if err != nil {
		t.Skipf("Cannot create parent 2: %v", err)
	}

	// Create children for parent2 only
	_, err = c.CreateWithParent("Child of parent 2", 1, []string{}, []string{}, parent2.ID)
	if err != nil {
		t.Skipf("Cannot create child for parent 2: %v", err)
	}

	// Parent1 should report no children despite other open beads existing
	hasChildren, err := c.HasOpenChildren(parent1.ID)
	if err != nil {
		t.Fatalf("HasOpenChildren(%q) error = %v", parent1.ID, err)
	}
	if hasChildren {
		t.Errorf("HasOpenChildren(%q) = true, want false (parent1 has no children, even though parent2 does)", parent1.ID)
	}

	// Parent2 should report having children
	hasChildren, err = c.HasOpenChildren(parent2.ID)
	if err != nil {
		t.Fatalf("HasOpenChildren(%q) error = %v", parent2.ID, err)
	}
	if !hasChildren {
		t.Errorf("HasOpenChildren(%q) = false, want true (parent2 has children)", parent2.ID)
	}
}

// TestHasOpenChildrenValidatesParentID tests that HasOpenChildren validates the parent ID
// before making the bd subprocess call.
// Expected failure: Validation should remain unchanged; this test ensures validation
// still works after the --parent flag implementation.
func TestHasOpenChildrenValidatesParentID(t *testing.T) {
	c, _ := NewClient()

	tests := []struct {
		name     string
		parentID string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "invalid parent ID with semicolon",
			parentID: "epic; rm -rf /",
			wantErr:  true,
			errMsg:   "invalid parent ID",
		},
		{
			name:     "invalid parent ID with spaces",
			parentID: "epic 123",
			wantErr:  true,
			errMsg:   "invalid parent ID",
		},
		{
			name:     "parent ID too long",
			parentID: strings.Repeat("a", maxIDLength+1),
			wantErr:  true,
			errMsg:   "invalid parent ID",
		},
		{
			name:     "empty parent ID",
			parentID: "",
			wantErr:  true,
			errMsg:   "invalid parent ID",
		},
		{
			name:     "command injection attempt",
			parentID: "epic$(whoami)",
			wantErr:  true,
			errMsg:   "invalid parent ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.HasOpenChildren(tt.parentID)
			if err == nil {
				t.Errorf("HasOpenChildren(%q) expected error but got nil", tt.parentID)
				return
			}

			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("HasOpenChildren(%q) should fail with %q, got: %v", tt.parentID, tt.errMsg, err)
			}
		})
	}
}

// TestHasOpenChildrenNilClient tests that HasOpenChildren returns error on nil client.
// Expected failure: Nil client check should remain unchanged after --parent flag implementation.
func TestHasOpenChildrenNilClient(t *testing.T) {
	var c *Client
	_, err := c.HasOpenChildren("valid-id")
	if err == nil {
		t.Error("HasOpenChildren() on nil client expected error but got nil")
		return
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("HasOpenChildren() on nil client should mention nil, got: %v", err)
	}
}
