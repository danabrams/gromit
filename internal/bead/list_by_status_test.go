package bead

import (
	"strings"
	"testing"
)

func TestListByStatus_EmptyStatus(t *testing.T) {
	c := &Client{}
	_, err := c.ListByStatus("")
	if err == nil {
		t.Fatal("ListByStatus() expected error for empty status")
	}
	if !strings.Contains(err.Error(), "status cannot be empty") {
		t.Fatalf("ListByStatus() error = %v", err)
	}
}

func TestListByStatus_ParsesResults(t *testing.T) {
	c := &Client{
		RunFn: func(args ...string) (string, error) {
			want := []string{"list", "--json", "--status", "in_progress", "--sort", "priority", "--limit", "0"}
			if len(args) != len(want) {
				t.Fatalf("run args len = %d, want %d (%v)", len(args), len(want), args)
			}
			for i := range want {
				if args[i] != want[i] {
					t.Fatalf("run args[%d] = %q, want %q", i, args[i], want[i])
				}
			}
			return `[{"id":"task-1","title":"In Progress","priority":1,"issue_type":"task","status":"in_progress"}]`, nil
		},
	}

	beads, err := c.ListByStatus("in_progress")
	if err != nil {
		t.Fatalf("ListByStatus() error = %v", err)
	}
	if len(beads) != 1 {
		t.Fatalf("len(beads) = %d, want 1", len(beads))
	}
	if beads[0].ID != "task-1" {
		t.Fatalf("beads[0].ID = %q, want task-1", beads[0].ID)
	}
}
