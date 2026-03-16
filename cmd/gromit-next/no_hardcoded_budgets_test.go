package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestNoHardcodedClaudeTimeout(t *testing.T) {
	// Find all non-test Go files in this package
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	// Match claude.NewClient calls with a numeric literal as the last argument
	// e.g., claude.NewClient("claude", ..., 300) or claude.NewClient("claude", ..., 900)
	re := regexp.MustCompile(`claude\.NewClient\([^)]*,\s*\d+\s*\)`)

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		matches := re.FindAllString(string(data), -1)
		for _, m := range matches {
			t.Errorf("%s: hardcoded timeout in Claude client: %s — use policy.Budgets.MaxTaskDurationSeconds instead", e.Name(), m)
		}
	}
}
