package bead

import (
	"strings"
	"testing"
)

func TestListReadyNilClient(t *testing.T) {
	var c *Client
	_, err := c.ListReady()
	if err == nil {
		t.Fatal("ListReady() expected error on nil client")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Fatalf("ListReady() error = %v, want mention of nil", err)
	}
}

func TestListReadyParsesResults(t *testing.T) {
	c := &Client{
		RunFn: func(args ...string) (string, error) {
			want := []string{"list", "--json", "--status", "ready", "--sort", "priority", "--limit", "0"}
			if len(args) != len(want) {
				t.Fatalf("run args len = %d, want %d (%v)", len(args), len(want), args)
			}
			for i := range want {
				if args[i] != want[i] {
					t.Fatalf("run args[%d] = %q, want %q", i, args[i], want[i])
				}
			}
			return `[{"id":"task-1","title":"Ready task","priority":1,"issue_type":"task","status":"ready"}]`, nil
		},
	}

	beads, err := c.ListReady()
	if err != nil {
		t.Fatalf("ListReady() error = %v", err)
	}
	if len(beads) != 1 {
		t.Fatalf("len(beads) = %d, want 1", len(beads))
	}
	if beads[0].ID != "task-1" {
		t.Fatalf("beads[0].ID = %q, want task-1", beads[0].ID)
	}
}

func TestListReadyEmptyOutput(t *testing.T) {
	c := &Client{
		RunFn: func(args ...string) (string, error) {
			return "[]", nil
		},
	}

	beads, err := c.ListReady()
	if err != nil {
		t.Fatalf("ListReady() error = %v", err)
	}
	if beads == nil {
		t.Fatal("ListReady() returned nil slice, want empty slice")
	}
	if len(beads) != 0 {
		t.Fatalf("len(beads) = %d, want 0", len(beads))
	}
}
