package debug

import (
	"os"
	"path/filepath"
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

func TestPersistLearnablePatternEntry_AppendsToSpecLearnings(t *testing.T) {
	tmpDir := t.TempDir()
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	initial := "# LEARNINGS\n\n## Confirmed Learnings\n\n"
	if err := os.WriteFile(learningsPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("setting up learnings file: %v", err)
	}

	entry, err := PersistLearnablePatternEntry(tmpDir, RootCauseBadBuildOutput, "", "", "")
	if err != nil {
		t.Fatalf("PersistLearnablePatternEntry failed: %v", err)
	}
	if entry == "" {
		t.Fatal("expected entry, got empty string")
	}
	if !strings.Contains(entry, "Capture build failure diagnostics") {
		t.Fatalf("unexpected entry content: %q", entry)
	}
	if !strings.Contains(entry, "*Autonomous: true*") {
		t.Fatalf("entry missing autonomous marker: %q", entry)
	}

	updated, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("reading LEARNINGS.md: %v", err)
	}
	if !strings.Contains(string(updated), "Capture build failure diagnostics") {
		t.Fatalf("LEARNINGS.md not updated, got:\n%s", string(updated))
	}
}

func TestBuildLearningEntryFromDiagnostics_DetectsSyntaxErrorPattern(t *testing.T) {
	entry := buildLearningEntryFromDiagnostics(RootCauseBadBuildOutput, "syntax error: unexpected )", "syntax error: unexpected )", "build")
	if entry == "" {
		t.Fatal("expected entry, got empty string")
	}
	if !strings.Contains(strings.ToLower(entry), "syntax error") {
		t.Fatalf("expected entry to describe syntax error, got %q", entry)
	}
	if !strings.Contains(entry, "Example:") {
		t.Fatalf("expected entry to include example guidance, got %q", entry)
	}
}
