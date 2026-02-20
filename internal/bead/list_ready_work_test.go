package bead

import "testing"

func TestListReadyWork_UsesReadyCommand(t *testing.T) {
	c := &Client{
		RunFn: func(args ...string) (string, error) {
			want := []string{"ready", "--json", "--sort", "priority", "--limit", "0"}
			if len(args) != len(want) {
				t.Fatalf("run args len = %d, want %d (%v)", len(args), len(want), args)
			}
			for i := range want {
				if args[i] != want[i] {
					t.Fatalf("run args[%d] = %q, want %q", i, args[i], want[i])
				}
			}
			return `[{"id":"task-1","title":"Ready task","priority":1,"issue_type":"task","status":"open"}]`, nil
		},
	}

	ready, err := c.ListReadyWork()
	if err != nil {
		t.Fatalf("ListReadyWork() error = %v", err)
	}
	if len(ready) != 1 {
		t.Fatalf("len(ready) = %d, want 1", len(ready))
	}
	if ready[0].ID != "task-1" {
		t.Fatalf("ready[0].ID = %q, want task-1", ready[0].ID)
	}
}
