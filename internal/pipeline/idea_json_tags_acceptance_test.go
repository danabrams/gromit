//go:build acceptance

package pipeline_test

import (
	"encoding/json"
	"testing"

	"github.com/danabrams/gromit/internal/pipeline"
)

func TestIdeaJSONTags_SnakeCase(t *testing.T) {
	// Expected failure: pipeline.Idea lacks json tags and pipeline.IdeaJSONKeys does not exist yet

	idea := pipeline.Idea{
		ID:       "idea-1",
		Text:     "Add a new cache layer",
		Type:     "feature",
		Context:  "High traffic endpoints",
		Status:   "refined",
		SpecName: "cache-layer",
	}

	data, err := json.Marshal(idea)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	assertJSONKeys(t, got, pipeline.IdeaJSONKeys)

	if _, ok := got["ID"]; ok {
		t.Errorf("json contains Go field name %q, want snake_case keys only", "ID")
	}
	if _, ok := got["SpecName"]; ok {
		t.Errorf("json contains Go field name %q, want snake_case keys only", "SpecName")
	}
}

func assertJSONKeys(t *testing.T, got map[string]any, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("json key count = %d, want %d", len(got), len(want))
	}

	for _, key := range want {
		if _, ok := got[key]; !ok {
			t.Fatalf("json missing key %q", key)
		}
	}
}
