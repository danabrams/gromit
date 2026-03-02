package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedFileContainsNewPipeline(t *testing.T) {
	path := filepath.Join("shared.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
	if !strings.Contains(string(data), "func newPipeline(") {
		t.Fatalf("shared.go does not declare newPipeline")
	}
}
