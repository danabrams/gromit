package provider

import (
    "os"
    "path/filepath"
    "testing"
)

func TestParseGeminiStreamRemoved(t *testing.T) {
	t.Parallel()

    path := filepath.Join(".", "gemini_stream.go")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("parseGeminiStream still exists; expected removal")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking %s: %v", path, err)
	}
}
