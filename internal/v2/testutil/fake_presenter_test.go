package testutil

import (
    "context"
    "testing"

    "github.com/danabrams/gromit/internal/v2/presentation"
)

func TestFakePresenterRecordsSummary(t *testing.T) {
    t.Parallel()

    fake := NewFakePresenter()
    summary := presentation.PresentationSummary{
        SpecName: "example",
        Success:  true,
    }

    if err := fake.PresentSummary(context.Background(), "spec-abc", summary); err != nil {
        t.Fatalf("PresentSummary failed: %v", err)
    }

    if len(fake.Calls) != 1 {
        t.Fatalf("expected 1 call, got %d", len(fake.Calls))
    }

    call := fake.Calls[0]
    if call.SpecID != "spec-abc" {
        t.Fatalf("unexpected spec ID: %q", call.SpecID)
    }
    if call.Summary.SpecName != summary.SpecName {
        t.Fatalf("unexpected summary: %+v", call.Summary)
    }
}
