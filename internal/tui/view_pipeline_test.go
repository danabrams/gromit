package tui

import (
    "testing"

    "github.com/danabrams/gromit/internal/backlog"
)

func TestIdeaListItemTitleSummary(t *testing.T) {
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
    if got, want := item.Summary(), "feature · spec=specs/pipeline-view.md · idea-123"; got != want {
        t.Fatalf("Summary() = %q, want %q", got, want)
    }
}
