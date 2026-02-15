package runner

import (
	"strings"
	"testing"
)

func TestSummarizeATDDProviderOutput(t *testing.T) {
	if got := summarizeATDDProviderOutput("   "); got != "no provider output" {
		t.Fatalf("expected empty output sentinel, got %q", got)
	}

	if got := summarizeATDDProviderOutput("  failure details  "); got != "failure details" {
		t.Fatalf("expected trimmed output, got %q", got)
	}

	long := strings.Repeat("x", 1700)
	got := summarizeATDDProviderOutput(long)
	if !strings.Contains(got, "...[truncated]...") {
		t.Fatalf("expected truncated marker, got %q", got)
	}
	if len(got) >= len(long) {
		t.Fatalf("expected summarized output shorter than input, got len=%d input=%d", len(got), len(long))
	}
}
