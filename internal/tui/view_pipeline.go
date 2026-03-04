package tui

import (
    "fmt"
    "strings"

    "github.com/danabrams/gromit/internal/backlog"
)

// IdeaListItem adapts a backlog.Idea for pipeline tabs.
type IdeaListItem struct {
    idea *backlog.Idea
}

func (i *IdeaListItem) Title() string {
    if i == nil || i.idea == nil {
        return ""
    }
    return strings.TrimSpace(i.idea.Text)
}

func (i *IdeaListItem) Summary() string {
    if i == nil || i.idea == nil {
        return ""
    }
    parts := []string{}
    if t := strings.TrimSpace(i.idea.Type); t != "" {
        parts = append(parts, t)
    }
    if spec := strings.TrimSpace(i.idea.SpecName); spec != "" {
        parts = append(parts, fmt.Sprintf("spec=%s", spec))
    }
    if id := strings.TrimSpace(i.idea.ID); id != "" {
        parts = append(parts, id)
    }
    return strings.Join(parts, " · ")
}
