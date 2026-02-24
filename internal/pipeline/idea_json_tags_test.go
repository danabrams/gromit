package pipeline

import (
	"reflect"
	"testing"
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

	if len(IdeaJSONKeys) != len(want) {
		t.Fatalf("IdeaJSONKeys len = %d, want %d", len(IdeaJSONKeys), len(want))
	}
}

func TestIdeaCreatedAtJSONTag(t *testing.T) {
	t.Parallel()

	ideaType := reflect.TypeOf(Idea{})
	sf, ok := ideaType.FieldByName("CreatedAt")
	if !ok {
		t.Fatalf("missing field %q on Idea", "CreatedAt")
	}

	if got := sf.Tag.Get("json"); got != "created_at" {
		t.Fatalf("Idea.CreatedAt json tag = %q, want %q", got, "created_at")
	}
}

func TestIdeaOptionalFieldJSONTagsUseOmitEmpty(t *testing.T) {
	t.Parallel()

	ideaType := reflect.TypeOf(Idea{})

	statusField, ok := ideaType.FieldByName("Status")
	if !ok {
		t.Fatalf("missing field %q on Idea", "Status")
	}
	if got := statusField.Tag.Get("json"); got != "status,omitempty" {
		t.Fatalf("Idea.Status json tag = %q, want %q", got, "status,omitempty")
	}

	specField, ok := ideaType.FieldByName("SpecName")
	if !ok {
		t.Fatalf("missing field %q on Idea", "SpecName")
	}
	if got := specField.Tag.Get("json"); got != "spec_name,omitempty" {
		t.Fatalf("Idea.SpecName json tag = %q, want %q", got, "spec_name,omitempty")
	}
}
