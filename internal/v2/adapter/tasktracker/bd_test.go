package tasktracker

import (
	"context"
	"fmt"
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

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
