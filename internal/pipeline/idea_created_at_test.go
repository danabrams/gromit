package pipeline

import (
	"reflect"
	"testing"
)

func TestIdeaHasCreatedAtField(t *testing.T) {
	t.Parallel()

	ideaType := reflect.TypeOf(Idea{})
	field, ok := ideaType.FieldByName("CreatedAt")
	if !ok {
		t.Fatalf("Idea is missing CreatedAt field")
	}

	if got := field.Tag.Get("json"); got != "created_at" {
		t.Fatalf("Idea.CreatedAt json tag = %q, want %q", got, "created_at")
	}
}
