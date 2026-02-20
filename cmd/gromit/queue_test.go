package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestGroupBeadsBySpec(t *testing.T) {
	beads := []*bead.Bead{
		{ID: "a", Labels: []string{"spec:auth"}},
		{ID: "b", Labels: []string{"priority:p1"}},
		{ID: "c", Labels: []string{"spec:api"}},
		{ID: "d", Labels: []string{"spec:auth"}},
	}

	grouped := groupBeadsBySpec(beads)

	if len(grouped["auth"]) != 2 {
		t.Fatalf("grouped[auth] count = %d, want 2", len(grouped["auth"]))
	}
	if len(grouped["api"]) != 1 {
		t.Fatalf("grouped[api] count = %d, want 1", len(grouped["api"]))
	}
	if len(grouped[""]) != 1 {
		t.Fatalf("grouped[(none)] count = %d, want 1", len(grouped[""]))
	}
}

func TestOrderedSpecKeys_PutsNoneLast(t *testing.T) {
	grouped := map[string][]*bead.Bead{
		"":     []*bead.Bead{{ID: "none"}},
		"auth": []*bead.Bead{{ID: "auth"}},
		"api":  []*bead.Bead{{ID: "api"}},
	}

	keys := orderedSpecKeys(grouped)
	if len(keys) != 3 {
		t.Fatalf("len(keys) = %d, want 3", len(keys))
	}
	if keys[0] != "api" || keys[1] != "auth" || keys[2] != "" {
		t.Fatalf("keys = %v, want [api auth \"\"]", keys)
	}
}

func TestColorizeLine(t *testing.T) {
	line := "hello"
	colored := colorizeLine(line, ansiGreen, true)
	if colored != ansiGreen+line+ansiReset {
		t.Fatalf("colored = %q", colored)
	}

	plain := colorizeLine(line, ansiGreen, false)
	if plain != line {
		t.Fatalf("plain = %q, want %q", plain, line)
	}
}
