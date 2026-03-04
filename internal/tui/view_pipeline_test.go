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

var pipelineTabRenderers = []struct {
	name     string
	header   string
	renderer pipelineTabRenderer
}{
	{"backlog", "Backlog", RenderBacklogTab},
	{"specs", "Specs", RenderSpecsTab},
	{"plans", "Plans", RenderPlansTab},
	{"queue", "Queue", RenderQueueTab},
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

func TestRenderPipelineTabsIncludesListAndDetailScenarios(t *testing.T) {
	listModel := &testPipelineTabModel{
		rendered: "item detail line 1\nitem detail line 2",
		selected: &mockListItem{title: "Focused idea", summary: "Full idea description"},
	}

	t.Run("list view", func(t *testing.T) {
		for _, tc := range pipelineTabRenderers {
			t.Run(tc.name, func(t *testing.T) {
				got := tc.renderer(nil, listModel, 40, false)
				header := "=== " + tc.header + " ===\n"
				if !strings.HasPrefix(got, header) {
					t.Fatalf("list view for %s tab missing header, got %q", tc.header, got)
				}
				if !strings.Contains(got, listModel.rendered) {
					t.Fatalf("expected rendered list for %s tab, got %q", tc.header, got)
				}
				if !strings.HasSuffix(got, "\n") {
					t.Fatalf("expected newline-terminated output for %s tab, got %q", tc.header, got)
				}
			})
		}
	})

	t.Run("detail view", func(t *testing.T) {
		for _, tc := range pipelineTabRenderers {
			t.Run(tc.name, func(t *testing.T) {
				got := tc.renderer(nil, listModel, 80, true)
				detailHeader := "=== " + tc.header + " Detail ==="
				if !strings.Contains(got, detailHeader) {
					t.Fatalf("detail view for %s tab missing header, got %q", tc.header, got)
				}
				if !strings.Contains(got, "Title: "+listModel.selected.Title()) {
					t.Fatalf("detail view for %s tab missing title, got %q", tc.header, got)
				}
				if !strings.Contains(got, "Summary: "+listModel.selected.Summary()) {
					t.Fatalf("detail view for %s tab missing summary, got %q", tc.header, got)
				}
				if !strings.Contains(got, "Content:") {
					t.Fatalf("detail view for %s tab missing content label, got %q", tc.header, got)
				}
				if !strings.Contains(got, listModel.rendered) {
					t.Fatalf("detail view for %s tab missing rendered content, got %q", tc.header, got)
				}
			})
		}
	})

	t.Run("empty lists", func(t *testing.T) {
		for _, tc := range pipelineTabRenderers {
			t.Run(tc.name+" list", func(t *testing.T) {
				got := tc.renderer(nil, nil, 80, false)
				if !strings.Contains(got, "No items") {
					t.Fatalf("expected 'No items' for %s list, got %q", tc.header, got)
				}
			})
			t.Run(tc.name+" detail", func(t *testing.T) {
				model := &testPipelineTabModel{}
				got := tc.renderer(nil, model, 80, true)
				if !strings.Contains(got, "No items") {
					t.Fatalf("expected 'No items' for %s detail, got %q", tc.header, got)
				}
			})
		}
	})
}

func TestRenderPipelineDetailAlwaysDisplaysContentHeading(t *testing.T) {
	listModel := &testPipelineTabModel{
		rendered: "",
		selected: &mockListItem{title: "Empty idea", summary: "No extra content"},
	}

	got := RenderBacklogTab(nil, listModel, 40, true)

	if !strings.Contains(got, "Content:") {
		t.Fatalf("expected detail view to include Content heading, got %q", got)
	}
}
