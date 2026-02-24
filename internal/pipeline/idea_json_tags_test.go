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
		{name: "Status", tag: "status"},
		{name: "SpecName", tag: "spec_name"},
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
