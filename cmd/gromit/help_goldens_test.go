package main

import (
	"os"
	"strings"
	"testing"
)

func loadGolden(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file %q: %v", path, err)
	}
	return strings.TrimSuffix(string(data), "\n")
}

func assertHelpMatchesGolden(t *testing.T, path string, actual string) {
	t.Helper()
	expected := loadGolden(t, path)
	if actual != expected {
		t.Fatalf("command help differs from golden %q\nexpected:\n%s\n\ngot:\n%s", path, expected, actual)
	}
}
