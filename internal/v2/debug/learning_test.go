package debug

import (
    "strings"
    "testing"
)

func TestBuildLearningEntryFromRootCause_IncludesPatternExample(t *testing.T) {
    entry := buildLearningEntryFromRootCause(RootCauseBadBuildOutput)
    if entry == "" {
        t.Fatal("entry is empty")
    }
    if !strings.Contains(entry, "Capture build failure diagnostics") {
        t.Fatalf("missing pattern description, got %q", entry)
    }
    if !strings.Contains(entry, "Example:") {
        t.Fatalf("missing example section, got %q", entry)
    }
}
