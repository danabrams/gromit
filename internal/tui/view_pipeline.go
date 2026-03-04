package tui

import (
    "fmt"
    "path/filepath"
    "strings"

    "github.com/danabrams/gromit/internal/backlog"
    "github.com/danabrams/gromit/internal/bead"
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

// SpecListItem adapts a spec path for pipeline tabs.
type SpecListItem struct {
    path string
}

func (s *SpecListItem) Title() string {
    if s == nil {
        return ""
    }
    return filepath.Base(s.path)
}

func (s *SpecListItem) Summary() string {
    if s == nil {
        return ""
    }
    return strings.TrimSpace(s.path)
}

// PlanListItem adapts a plan path for pipeline tabs.
type PlanListItem struct {
    path string
}

func (p *PlanListItem) Title() string {
    if p == nil {
        return ""
    }
    return filepath.Base(p.path)
}

func (p *PlanListItem) Summary() string {
    if p == nil {
        return ""
    }
    return strings.TrimSpace(p.path)
}

// BeadListItem adapts a bead.Bead for pipeline tabs.
type BeadListItem struct {
    bead *bead.Bead
}

func (b *BeadListItem) Title() string {
    if b == nil || b.bead == nil {
        return ""
    }
    return strings.TrimSpace(b.bead.Title)
}

func (b *BeadListItem) Summary() string {
    if b == nil || b.bead == nil {
        return ""
    }
    parts := []string{}
    if id := strings.TrimSpace(b.bead.ID); id != "" {
        parts = append(parts, id)
    }
    if status := strings.TrimSpace(b.bead.Status); status != "" {
        parts = append(parts, fmt.Sprintf("status=%s", status))
    }
    parts = append(parts, fmt.Sprintf("priority=%d", b.bead.Priority))
    return strings.Join(parts, " · ")
}
