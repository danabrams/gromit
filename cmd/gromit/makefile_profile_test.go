package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakefile_TestProfileTarget(t *testing.T) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}

	makefilePath := filepath.Join(projectRoot, "Makefile")
	data, err := os.ReadFile(makefilePath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", makefilePath, err)
	}

	content := string(data)
	if !strings.Contains(content, "test-profile:") {
		t.Fatalf("Makefile missing test-profile target")
	}
	if !strings.Contains(content, "go test -json ./internal/runner -count=1 | jq -r") {
		t.Fatalf("Makefile test-profile target missing go test -json pipeline")
	}
	if !strings.Contains(content, "sort -rn") {
		t.Fatalf("Makefile test-profile target missing descending sort")
	}
}
