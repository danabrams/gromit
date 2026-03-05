//go:build acceptance
// +build acceptance

package v2

import (
    "testing"

    "github.com/danabrams/gromit/internal/bead"
    v2review "github.com/danabrams/gromit/internal/v2/review"
)

func TestReviewStageInScopeFindingsCreateNextGenBeads(t *testing.T) {
    t.Parallel()

    parent := &bead.Bead{
        ID: "parent-123",
        Labels: []string{"gen:2", "spec:demo"},
    }
    findings := []v2review.Finding{
        {
            Title: "Fix validation gap",
            Description: "Tighten input validation for user creation",
            InScope: true,
        },
    }

    classifier := v2review.NewClassifier(nil)
    result := classifier.Classify(parent, findings)

    if len(result.Beads) != 1 {
        t.Fatalf("expected 1 in-scope bead, got %d", len(result.Beads))
    }

    beadLabels := result.Beads[0].Labels
    if !hasLabel(beadLabels, "gen:3") {
        t.Fatalf("expected bead to be tagged with gen:3, got %v", beadLabels)
    }
}

func hasLabel(labels []string, target string) bool {
    for _, label := range labels {
        if label == target {
            return true
        }
    }
    return false
}
