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

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
