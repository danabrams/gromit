package main

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/scope"
)

// TestBuildBeadFilterFromSpecFlag verifies that we can build a bead ID filter
// from a --spec flag value using scope.ResolveSpec
func TestBuildBeadFilterFromSpecFlag(t *testing.T) {
	specFlag := "init-wizard"

	// Resolve spec to labels
	labels := scope.ResolveSpec(specFlag)

	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(labels))
	}

	expectedLabel := "spec:init-wizard"
	if labels[0] != expectedLabel {
		t.Fatalf("expected label %q, got %q", expectedLabel, labels[0])
	}

	// TODO: Next step would be to call bead.ListWithLabel(label) to get bead IDs
	// and build filter map[string]bool from those IDs
}

// TestBuildBeadFilter_EmptyLabelReturnsNilFilter verifies that when no labels
// are provided, buildBeadFilter returns nil (meaning no filtering)
func TestBuildBeadFilter_EmptyLabelReturnsNilFilter(t *testing.T) {
	ctx := context.Background()
	filter, err := buildBeadFilter(ctx, []string{})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if filter != nil {
		t.Fatalf("expected nil filter for empty labels, got %v", filter)
	}
}

// buildBeadFilter resolves labels to bead IDs and returns a filter map.
// If labels is empty, returns nil (no filtering).
func buildBeadFilter(ctx context.Context, labels []string) (map[string]bool, error) {
	if len(labels) == 0 {
		return nil, nil
	}

	client, err := bead.NewClient()
	if err != nil {
		return nil, err
	}

	filter := make(map[string]bool)
	for _, label := range labels {
		beads, err := client.ListWithLabel(label)
		if err != nil {
			return nil, err
		}

		for _, b := range beads {
			filter[b.ID] = true
		}
	}

	return filter, nil
}
