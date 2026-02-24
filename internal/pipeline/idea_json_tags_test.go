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
}

<<<<<<< HEAD
func TestIdeaOptionalJSONTagsUseOmitEmpty(t *testing.T) {
	t.Parallel()

	ideaType := reflect.TypeOf(Idea{})
	for _, tt := range []struct {
		field string
		want  string
	}{
		{field: "Status", want: "status,omitempty"},
		{field: "SpecName", want: "spec_name,omitempty"},
	} {
		sf, ok := ideaType.FieldByName(tt.field)
		if !ok {
			t.Fatalf("missing field %q on Idea", tt.field)
		}
		if got := sf.Tag.Get("json"); got != tt.want {
			t.Fatalf("Idea.%s json tag = %q, want %q", tt.field, got, tt.want)
		}
	}
}

func TestIdeaJSONKeys(t *testing.T) {
	t.Parallel()

	want := []string{"id", "text", "type", "context", "created_at", "status", "spec_name"}
	if !reflect.DeepEqual(IdeaJSONKeys, want) {
		t.Fatalf("IdeaJSONKeys = %#v, want %#v", IdeaJSONKeys, want)
	}
}
