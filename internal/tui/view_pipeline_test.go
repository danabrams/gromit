package tui

import (
    "strings"
    "testing"

    "github.com/danabrams/gromit/internal/backlog"
    "github.com/danabrams/gromit/internal/bead"
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

func TestSpecListItemTitleSummary(t *testing.T) {
    path := "specs/awesome-feature.md"
    item := &SpecListItem{path: path}

    if got, want := item.Title(), "awesome-feature.md"; got != want {
        t.Fatalf("Title() = %q, want %q", got, want)
    }
    if got, want := item.Summary(), path; got != want {
        t.Fatalf("Summary() = %q, want %q", got, want)
    }
}

func TestPlanListItemTitleSummary(t *testing.T) {
    path := "plans/awesome-feature.plan"
    item := &PlanListItem{path: path}

    if got, want := item.Title(), "awesome-feature.plan"; got != want {
        t.Fatalf("Title() = %q, want %q", got, want)
    }
    if got, want := item.Summary(), path; got != want {
        t.Fatalf("Summary() = %q, want %q", got, want)
    }
}

func TestBeadListItemTitleSummary(t *testing.T) {
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
}

func TestRenderBacklogTabUsesListModelOutput(t *testing.T) {
    model := &testPipelineTabModel{
        rendered: "idea-1 · feature\nidea-2 · bug\n",
    }

    got := RenderBacklogTab(nil, model, 60, false)
    if !strings.Contains(got, "=== Backlog ===") {
        t.Fatalf("expected header, got %q", got)
    }
    if !strings.Contains(got, model.rendered) {
        t.Fatalf("expected list content, got %q", got)
    }
}

type testPipelineTabModel struct {
	rendered string
	selected ListItem
}

func (m *testPipelineTabModel) Render(width int) string {
    return m.rendered
}

func (m *testPipelineTabModel) Selected() ListItem {
	return m.selected
}

type pipelineTabRenderer func(*Store, pipelineTabListModel, int, bool) string

type mockListItem struct {
	title   string
	summary string
}

func (m *mockListItem) Title() string {
	return m.title
}

func (m *mockListItem) Summary() string {
	return m.summary
}

func TestRenderPipelineTabsDetailViewShowsTitleAndSummary(t *testing.T) {
	listModel := &testPipelineTabModel{
		selected: &mockListItem{title: "Focused idea", summary: "Full idea description"},
	}
	renderers := []struct {
		name     string
		header   string
		renderer pipelineTabRenderer
	}{
		{"backlog", "Backlog", RenderBacklogTab},
		{"specs", "Specs", RenderSpecsTab},
		{"plans", "Plans", RenderPlansTab},
		{"queue", "Queue", RenderQueueTab},
	}

	for _, tc := range renderers {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.renderer(nil, listModel, 80, true)
			if !strings.Contains(got, "=== "+tc.header+" Detail ===") {
				t.Fatalf("expected detail header for %s tab, got %q", tc.header, got)
			}
			if !strings.Contains(got, listModel.selected.Title()) {
				t.Fatalf("expected title in detail view, got %q", got)
			}
			if !strings.Contains(got, listModel.selected.Summary()) {
				t.Fatalf("expected summary in detail view, got %q", got)
			}
		})
	}
}
