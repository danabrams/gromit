package bead

import (
	"context"
	"testing"
)

func TestReady_SelectsOldestWithinPriority(t *testing.T) {
	t.Parallel()

	c := &Client{
		RunFn: func(args ...string) (string, error) {
			return `[
				{"id":"newer","title":"Newer","priority":1,"created_at":"2026-02-27T12:00:00Z","issue_type":"task","status":"open"},
				{"id":"older","title":"Older","priority":1,"created_at":"2026-02-27T08:00:00Z","issue_type":"task","status":"open"}
			]`, nil
		},
	}

	got, err := c.Ready(context.Background())
	if err != nil {
		t.Fatalf("Ready() error = %v", err)
	}
	if got == nil {
		t.Fatal("Ready() returned nil bead")
	}
	if got.ID != "older" {
		t.Fatalf("Ready() picked %q, want %q", got.ID, "older")
	}
}

func TestReadyExcluding_SelectsOldestWithinPriority(t *testing.T) {
	t.Parallel()

	c := &Client{
		RunFn: func(args ...string) (string, error) {
			return `[
				{"id":"newer","title":"Newer","priority":2,"created_at":"2026-02-27T12:00:00Z","issue_type":"task","status":"open"},
				{"id":"older","title":"Older","priority":2,"created_at":"2026-02-27T08:00:00Z","issue_type":"task","status":"open"}
			]`, nil
		},
	}

	got, err := c.ReadyExcluding(context.Background(), map[string]bool{"skip-me": true})
	if err != nil {
		t.Fatalf("ReadyExcluding() error = %v", err)
	}
	if got == nil {
		t.Fatal("ReadyExcluding() returned nil bead")
	}
	if got.ID != "older" {
		t.Fatalf("ReadyExcluding() picked %q, want %q", got.ID, "older")
	}
}

func TestReady_PreservesPriorityBeforeCreatedAt(t *testing.T) {
	t.Parallel()

	c := &Client{
		RunFn: func(args ...string) (string, error) {
			return `[
				{"id":"older-p2","title":"Older P2","priority":2,"created_at":"2026-02-26T08:00:00Z","issue_type":"task","status":"open"},
				{"id":"newer-p1","title":"Newer P1","priority":1,"created_at":"2026-02-27T12:00:00Z","issue_type":"task","status":"open"}
			]`, nil
		},
	}

	got, err := c.Ready(context.Background())
	if err != nil {
		t.Fatalf("Ready() error = %v", err)
	}
	if got == nil {
		t.Fatal("Ready() returned nil bead")
	}
	if got.ID != "newer-p1" {
		t.Fatalf("Ready() picked %q, want %q", got.ID, "newer-p1")
	}
}
