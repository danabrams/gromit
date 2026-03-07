package dep

import (
	"reflect"
	"testing"
)

func TestParseSpecMetadata_DependsOnKey(t *testing.T) {
	fm := map[string]interface{}{
		"id":         "spec-a",
		"accepted":   true,
		"depends_on": []interface{}{"spec-b", "spec-c"},
	}
	meta := parseSpecMetadata("fallback-id", fm)
	if meta.ID != "spec-a" {
		t.Errorf("expected ID spec-a, got %s", meta.ID)
	}
	if !meta.Accepted {
		t.Error("expected Accepted true")
	}
	want := []string{"spec-b", "spec-c"}
	if !reflect.DeepEqual(meta.DependsOn, want) {
		t.Errorf("DependsOn = %v, want %v", meta.DependsOn, want)
	}
}

func TestParseSpecMetadata_DependenciesKey(t *testing.T) {
	fm := map[string]interface{}{
		"id":           "spec-a",
		"accepted":     true,
		"dependencies": []interface{}{"spec-x", "spec-y"},
	}
	meta := parseSpecMetadata("fallback-id", fm)
	want := []string{"spec-x", "spec-y"}
	if !reflect.DeepEqual(meta.DependsOn, want) {
		t.Errorf("DependsOn = %v, want %v", meta.DependsOn, want)
	}
}

func TestParseSpecMetadata_BothKeysMerged(t *testing.T) {
	fm := map[string]interface{}{
		"id":           "spec-a",
		"accepted":     true,
		"depends_on":   []interface{}{"spec-b"},
		"dependencies": []interface{}{"spec-c", "spec-d"},
	}
	meta := parseSpecMetadata("fallback-id", fm)
	want := []string{"spec-b", "spec-c", "spec-d"}
	if !reflect.DeepEqual(meta.DependsOn, want) {
		t.Errorf("DependsOn = %v, want %v", meta.DependsOn, want)
	}
}

func TestParseSpecMetadata_NeitherKey(t *testing.T) {
	fm := map[string]interface{}{
		"id":       "spec-a",
		"accepted": true,
	}
	meta := parseSpecMetadata("fallback-id", fm)
	if len(meta.DependsOn) != 0 {
		t.Errorf("expected empty DependsOn, got %v", meta.DependsOn)
	}
}
