package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGeminiHelpersRemoved(t *testing.T) {
	t.Parallel()

	path := filepath.Join(".", "gemini_helpers.go")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("gemini_helpers still exists; expected removal")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking %s: %v", path, err)
	}
}
