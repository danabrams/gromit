package tui

import (
    "testing"

    "github.com/danabrams/gromit/internal/backlog"
    "github.com/danabrams/gromit/internal/bead"
)

func TestIdeaListItemTitleSummaryIdentifier(t *testing.T) {
    idea := &backlog.Idea{
        ID:       "idea-123",
        Text:     "Implement the pipeline view",
        Type:     "feature",
        SpecName: "specs/pipeline-view.md",
    }
    item := &IdeaListItem{idea: idea}

    if got, want := item.Title(), idea.Text; got != want {
        t.Fatalf("Title() = %q, want %q", got, want)
    }

    wantSummary := "feature · spec=specs/pipeline-view.md · idea-123"
    if got := item.Summary(); got != wantSummary {
        t.Fatalf("Summary() = %q, want %q", got, wantSummary)
    }

    if got, want := item.Identifier(), idea.ID; got != want {
        t.Fatalf("Identifier() = %q, want %q", got, want)
    }
}

func TestSpecListItemTitleSummaryIdentifier(t *testing.T) {
    path := "specs/awesome-feature.md"
    item := &SpecListItem{path: path}

    if got, want := item.Title(), "awesome-feature.md"; got != want {
        t.Fatalf("Title() = %q, want %q", got, want)
    }
    if got, want := item.Summary(), path; got != want {
        t.Fatalf("Summary() = %q, want %q", got, want)
    }
    if got, want := item.Identifier(), path; got != want {
        t.Fatalf("Identifier() = %q, want %q", got, want)
    }
}

func TestPlanListItemTitleSummaryIdentifier(t *testing.T) {
    path := "plans/awesome-feature.plan"
    item := &PlanListItem{path: path}

    if got, want := item.Title(), "awesome-feature.plan"; got != want {
        t.Fatalf("Title() = %q, want %q", got, want)
    }
    if got, want := item.Summary(), path; got != want {
        t.Fatalf("Summary() = %q, want %q", got, want)
    }
    if got, want := item.Identifier(), path; got != want {
        t.Fatalf("Identifier() = %q, want %q", got, want)
    }
}

func TestBeadListItemTitleSummaryIdentifier(t *testing.T) {
    b := &bead.Bead{
        ID:       "B-1",
        Title:    "Execute pipeline",
        Status:   "open",
        Priority: 1,
    }
    item := &BeadListItem{bead: b}

    if got, want := item.Title(), b.Title; got != want {
        t.Fatalf("Title() = %q, want %q", got, want)
    }
    if got, want := item.Summary(), "B-1 · status=open · priority=1"; got != want {
        t.Fatalf("Summary() = %q, want %q", got, want)
    }
    if got, want := item.Identifier(), b.ID; got != want {
        t.Fatalf("Identifier() = %q, want %q", got, want)
    }
}
