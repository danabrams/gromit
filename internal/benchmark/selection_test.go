package benchmark

import "testing"

func TestResolveSelectedBeads_CLIOverrideTakesPrecedenceOverManifest(t *testing.T) {
	resolved, err := ResolveSelectedBeads([]string{"gromit-1", "gromit-2"}, []string{"gromit-9", "gromit-8"}, 0)
	if err != nil {
		t.Fatalf("ResolveSelectedBeads() error = %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("resolved length = %d, want 2", len(resolved))
	}
	if resolved[0] != "gromit-9" || resolved[1] != "gromit-8" {
		t.Fatalf("resolved = %v, want [gromit-9 gromit-8]", resolved)
	}
}

func TestResolveSelectedBeads_ReturnsErrorForDuplicates(t *testing.T) {
	_, err := ResolveSelectedBeads([]string{"gromit-1", "gromit-1"}, nil, 0)
	if err == nil {
		t.Fatal("ResolveSelectedBeads() error = nil, want duplicate error")
	}
}

func TestResolveSelectedBeads_ReturnsErrorForEmptySelection(t *testing.T) {
	_, err := ResolveSelectedBeads(nil, nil, 0)
	if err == nil {
		t.Fatal("ResolveSelectedBeads() error = nil, want empty selection error")
	}
}

func TestResolveSelectedBeads_ReturnsErrorWhenSelectionIsNotExactlyFive(t *testing.T) {
	_, err := ResolveSelectedBeads([]string{"gromit-1", "gromit-2", "gromit-3", "gromit-4"}, nil, 0)
	if err == nil {
		t.Fatal("ResolveSelectedBeads() error = nil, want fixed-size cohort error")
	}
}

func TestResolveSelectedBeads_TruncatesDeterministicallyWithBeadCount(t *testing.T) {
	resolved, err := ResolveSelectedBeads([]string{"gromit-1", "gromit-2", "gromit-3"}, nil, 2)
	if err != nil {
		t.Fatalf("ResolveSelectedBeads() error = %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("resolved length = %d, want 2", len(resolved))
	}
	if resolved[0] != "gromit-1" || resolved[1] != "gromit-2" {
		t.Fatalf("resolved = %v, want [gromit-1 gromit-2]", resolved)
	}
}

func TestResolveSelectedBeads_ReturnsErrorWhenBeadCountExceedsSelection(t *testing.T) {
	_, err := ResolveSelectedBeads([]string{"gromit-1", "gromit-2"}, nil, 3)
	if err == nil {
		t.Fatal("ResolveSelectedBeads() error = nil, want out-of-range bead-count error")
	}
}

func TestResolveSelectedBeads_ReturnsErrorWhenBeadCountIsNegative(t *testing.T) {
	_, err := ResolveSelectedBeads([]string{"gromit-1", "gromit-2"}, nil, -1)
	if err == nil {
		t.Fatal("ResolveSelectedBeads() error = nil, want negative bead-count error")
	}
}
