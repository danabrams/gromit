//go:build acceptance
// +build acceptance

package v2

import (
    "testing"
    "time"

    "github.com/danabrams/gromit/internal/bead"
    "github.com/danabrams/gromit/internal/events"
    v2review "github.com/danabrams/gromit/internal/v2/review"
)

func TestReviewStageInScopeFindingsCreateNextGenBeads(t *testing.T) {
    t.Parallel()

    parent := &bead.Bead{
        ID:     "parent-123",
        Labels: []string{"gen:2", "spec:demo"},
    }
    findings := []v2review.Finding{
        {
            Title:       "Fix validation gap",
            Description: "Tighten input validation for user creation",
            InScope:     true,
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

func TestReviewStageOutOfScopeFindingsEmitEvents(t *testing.T) {
    t.Parallel()

    parent := &bead.Bead{
        ID:     "parent-456",
        Labels: []string{"gen:1"},
    }

    emitter := events.NewEmitter()
    ch := emitter.Subscribe()
    t.Cleanup(func() {
        emitter.Unsubscribe(ch)
    })

    findings := []v2review.Finding{
        {
            Title:         "Document audit drift",
            Description:   "Existing documentation drift is outside current acceptance criteria",
            InScope:       false,
            AffectedFiles: []string{"README.md", "docs/audit.md"},
        },
    }

    classifier := v2review.NewClassifier(emitter)
    result := classifier.Classify(parent, findings)

    if len(result.Beads) != 0 {
        t.Fatalf("expected no beads for out-of-scope findings, got %d", len(result.Beads))
    }

    select {
    case evt := <-ch:
        reviewEvt, ok := evt.(*events.ReviewFindingEvent)
        if !ok {
            t.Fatalf("unexpected event type %T", evt)
        }
        if reviewEvt.InScope {
            t.Fatalf("expected in_scope=false, got true")
        }
        if reviewEvt.Description != findings[0].Description {
            t.Fatalf("description = %q, want %q", reviewEvt.Description, findings[0].Description)
        }
        if len(reviewEvt.AffectedFiles) != 2 || reviewEvt.AffectedFiles[0] != "README.md" {
            t.Fatalf("affected files = %v", reviewEvt.AffectedFiles)
        }
    case <-time.After(time.Second):
        t.Fatal("timed out waiting for review finding event")
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
