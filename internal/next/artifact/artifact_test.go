package artifact

import (
	"os"
	"path/filepath"
	"testing"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestJSONStore_WriteAndRead(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore()

	type sample struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	src := sample{Name: "test", Count: 42}
	if err := store.Write(dir, "sample", src); err != nil {
		t.Fatalf("Write: %v", err)
	}

	var dest sample
	if err := store.Read(dir, "sample", &dest); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if dest.Name != "test" || dest.Count != 42 {
		t.Errorf("Read = %+v, want {test 42}", dest)
	}
}

func TestJSONStore_Exists(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore()

	if store.Exists(dir, "missing") {
		t.Error("Exists should return false for missing artifact")
	}

	if err := store.Write(dir, "present", map[string]string{"a": "b"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !store.Exists(dir, "present") {
		t.Error("Exists should return true after Write")
	}
}

func TestJSONStore_WritesCorrectPath(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore()

	store.Write(dir, "architecture", map[string]string{})

	expected := filepath.Join(dir, "architecture.json")
	if !fileExists(expected) {
		t.Errorf("expected file at %s", expected)
	}
}
