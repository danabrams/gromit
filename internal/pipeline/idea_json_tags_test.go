package pipeline

import (
	"encoding/json"
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

func TestIdeaJSON_OmitsEmptyOptionalFields(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(Idea{
		ID:   "idea-1",
		Text: "test",
		Type: "feature",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if _, ok := got["status"]; ok {
		t.Fatal("status should be omitted when empty")
	}
	if _, ok := got["spec_name"]; ok {
		t.Fatal("spec_name should be omitted when empty")
	}
}
