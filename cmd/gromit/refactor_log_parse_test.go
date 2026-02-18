package main

import (
	"strings"
	"testing"
)

func TestParseTestLog_DetectsFailingTest(t *testing.T) {
	input := `=== RUN   TestFoo
--- FAIL: TestFoo (0.01s)
    foo_test.go:12: got 1, want 2
FAIL
FAIL	github.com/danabrams/gromit/internal/foo	0.01s
`
	result := parseTestLog(strings.NewReader(input))

	if len(result.Failures) != 1 {
		t.Fatalf("want 1 failure, got %d", len(result.Failures))
	}
	got := result.Failures[0]
	if got.Package != "github.com/danabrams/gromit/internal/foo" {
		t.Errorf("package = %q, want %q", got.Package, "github.com/danabrams/gromit/internal/foo")
	}
	if got.Test != "TestFoo" {
		t.Errorf("test = %q, want %q", got.Test, "TestFoo")
	}
}
