package pipeline

import (
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
)

func TestIdeaJSONTags(t *testing.T) {
	t.Parallel()

	type fieldTag struct {
		name string
		tag  string
	}

	want := []fieldTag{
		{name: "ID", tag: "id"},
		{name: "Text", tag: "text"},
		{name: "Type", tag: "type"},
		{name: "Context", tag: "context"},
		{name: "CreatedAt", tag: "created_at"},
		{name: "Status", tag: "status,omitempty"},
		{name: "SpecName", tag: "spec_name,omitempty"},
	}

	ideaType := reflect.TypeOf(Idea{})
	for _, field := range want {
		sf, ok := ideaType.FieldByName(field.name)
		if !ok {
			t.Fatalf("missing field %q on Idea", field.name)
		}

		if got := sf.Tag.Get("json"); got != field.tag {
			t.Fatalf("Idea.%s json tag = %q, want %q", field.name, got, field.tag)
		}
	}
}

func TestIdeaTypeMatchesBacklogCanonicalType(t *testing.T) {
	t.Parallel()

	pipelineIdeaType := reflect.TypeOf(Idea{})
	backlogIdeaType := reflect.TypeOf(backlog.Idea{})
	if pipelineIdeaType != backlogIdeaType {
		t.Fatalf("pipeline.Idea type = %v, want canonical backlog.Idea type = %v", pipelineIdeaType, backlogIdeaType)
	}

	statusField, ok := pipelineIdeaType.FieldByName("Status")
	if !ok {
		t.Fatal("missing Status field")
	}
	if got := statusField.Tag.Get("json"); got != "status,omitempty" {
		t.Fatalf("Idea.Status json tag = %q, want %q", got, "status,omitempty")
	}

	specNameField, ok := pipelineIdeaType.FieldByName("SpecName")
	if !ok {
		t.Fatal("missing SpecName field")
	}
	if got := specNameField.Tag.Get("json"); got != "spec_name,omitempty" {
		t.Fatalf("Idea.SpecName json tag = %q, want %q", got, "spec_name,omitempty")
	}
}
