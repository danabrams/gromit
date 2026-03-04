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

type pipelineTabListModel interface {
	Render(width int) string
	Selected() ListItem
}

// RenderBacklogTab renders the backlog tab using the provided list model.
func RenderBacklogTab(_ *Store, listModel pipelineTabListModel, width int, inDetailView bool) string {
	return renderPipelineTab("Backlog", listModel, width, inDetailView)
}

// RenderSpecsTab renders the specs tab using the provided list model.
func RenderSpecsTab(_ *Store, listModel pipelineTabListModel, width int, inDetailView bool) string {
	return renderPipelineTab("Specs", listModel, width, inDetailView)
}

// RenderPlansTab renders the plans tab using the provided list model.
func RenderPlansTab(_ *Store, listModel pipelineTabListModel, width int, inDetailView bool) string {
	return renderPipelineTab("Plans", listModel, width, inDetailView)
}

// RenderQueueTab renders the queue tab using the provided list model.
func RenderQueueTab(_ *Store, listModel pipelineTabListModel, width int, inDetailView bool) string {
	return renderPipelineTab("Queue", listModel, width, inDetailView)
}

func renderPipelineTab(title string, listModel pipelineTabListModel, width int, inDetailView bool) string {
	if inDetailView {
		return renderPipelineDetail(title, listModel, width)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("=== %s ===\n", title))

	if listModel == nil {
		b.WriteString("No items\n")
		return b.String()
	}

	raw := listModel.Render(width)
	if strings.TrimSpace(raw) == "" {
		b.WriteString("No items\n")
		return b.String()
	}

	b.WriteString(raw)
	if !strings.HasSuffix(raw, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

func renderPipelineDetail(title string, listModel pipelineTabListModel, width int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("=== %s Detail ===\n", title))
	if listModel == nil {
		b.WriteString("No items\n")
		return b.String()
	}
	selected := listModel.Selected()
	if selected == nil {
		b.WriteString("No items\n")
		return b.String()
	}
	b.WriteString(fmt.Sprintf("Title: %s\n", selected.Title()))
	b.WriteString(fmt.Sprintf("Summary: %s\n", selected.Summary()))
	if rendered := listModel.Render(width); strings.TrimSpace(rendered) != "" {
		b.WriteString("Content:\n")
		b.WriteString(rendered)
		if !strings.HasSuffix(rendered, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String()
}
