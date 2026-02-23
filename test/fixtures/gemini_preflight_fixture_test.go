package fixtures_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeminiPreflightFixture_IncludesChecklistAndObservedResults(t *testing.T) {
	fixturePath := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "preflight.md")

	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read preflight fixture: %v", err)
	}

	body := string(content)
	required := []string{
		"# Gemini Preflight",
		"## Checklist",
		"## Observed Results",
		"gemini --version",
	}

	for _, token := range required {
		if !strings.Contains(body, token) {
			t.Fatalf("preflight fixture must contain %q", token)
		}
	}
}

func TestGeminiCommandsLogFixture_InitializedWithTimestampedLedgerEntries(t *testing.T) {
	fixturePath := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "commands.log")

	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read commands log fixture: %v", err)
	}

	body := string(content)
	required := []string{
		"timestamp=",
		"command=",
		"exit_code=",
	}

	for _, token := range required {
		if !strings.Contains(body, token) {
			t.Fatalf("commands log must contain %q", token)
		}
	}
}

func TestGeminiPreflightFixture_DocumentsAppendHarnessPattern(t *testing.T) {
	fixturePath := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "preflight.md")

	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read preflight fixture: %v", err)
	}

	body := string(content)
	required := []string{
		"## Capture Harness",
		"exit_code=$?",
		".gromit/plans/fixtures/gemini/commands.log",
		"command=\"",
	}

	for _, token := range required {
		if !strings.Contains(body, token) {
			t.Fatalf("preflight fixture must document harness token %q", token)
		}
	}
}
