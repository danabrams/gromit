package pipeline

import (
	"reflect"
	"testing"
	"time"
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

func TestIdeaSchemaParity_OptionalFieldsAndCreatedAt(t *testing.T) {
	t.Parallel()

	ideaType := reflect.TypeOf(Idea{})

	createdAt, ok := ideaType.FieldByName("CreatedAt")
	if !ok {
		t.Fatalf("missing field %q on Idea", "CreatedAt")
	}
	if createdAt.Type != reflect.TypeOf(time.Time{}) {
		t.Fatalf("Idea.CreatedAt type = %v, want %v", createdAt.Type, reflect.TypeOf(time.Time{}))
	}
	if got := createdAt.Tag.Get("json"); got != "created_at" {
		t.Fatalf("Idea.CreatedAt json tag = %q, want %q", got, "created_at")
	}

	status, ok := ideaType.FieldByName("Status")
	if !ok {
		t.Fatalf("missing field %q on Idea", "Status")
	}
	if got := status.Tag.Get("json"); got != "status,omitempty" {
		t.Fatalf("Idea.Status json tag = %q, want %q", got, "status,omitempty")
	}

	specName, ok := ideaType.FieldByName("SpecName")
	if !ok {
		t.Fatalf("missing field %q on Idea", "SpecName")
	}
	if got := specName.Tag.Get("json"); got != "spec_name,omitempty" {
		t.Fatalf("Idea.SpecName json tag = %q, want %q", got, "spec_name,omitempty")
	}
}
