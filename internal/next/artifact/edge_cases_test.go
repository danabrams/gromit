package artifact

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJSONStore_ReadCorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore()

	// Write corrupted JSON
	os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{invalid json!!!"), 0o644)

	var dest map[string]any
	err := store.Read(dir, "broken", &dest)
	if err == nil {
		t.Error("expected error for corrupted JSON artifact")
	}
}

func TestJSONStore_ReadMissing(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore()

	var dest map[string]any
	err := store.Read(dir, "nonexistent", &dest)
	if err == nil {
		t.Error("expected error for missing artifact")
	}
}
