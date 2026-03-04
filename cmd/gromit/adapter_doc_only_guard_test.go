package main

import (
	"os"
	"strings"
	"testing"
)

func TestAdapterDocOnlyTestsAreLimited(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading cmd/gromit: %v", err)
	}

	var docOnly []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "adapter_") || !strings.HasSuffix(name, "_test.go") {
			continue
		}
		contents, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if isDocOnlyAdapterTest(string(contents)) {
			docOnly = append(docOnly, name)
		}
	}

	allowed := map[string]struct{}{
		"adapter_interface_contracts_test.go": {},
	}

	var disallowed []string
	for _, sample := range docOnly {
		if _, ok := allowed[sample]; !ok {
			disallowed = append(disallowed, sample)
		}
	}

	if len(disallowed) > 0 {
		t.Fatalf("found %d doc-only adapter tests that must be deleted: %v", len(disallowed), disallowed)
	}
}

func isDocOnlyAdapterTest(content string) bool {
	if !strings.Contains(content, "t.Log") && !strings.Contains(content, "t.Logf") {
		return false
	}

	assertionMarkers := []string{"t.Fatalf", "t.Fatalf(", "t.Errorf", "t.Errorf(", "t.Fatal", "t.Fatal(", "t.Error", "t.Error(", "t.Fail", "t.Fail(", "t.FailNow", "t.FailNow(", "t.Skip", "t.Skip(", "t.Skipf", "t.Skipf("}
	for _, marker := range assertionMarkers {
		if strings.Contains(content, marker) {
			return false
		}
	}

	return true
}
