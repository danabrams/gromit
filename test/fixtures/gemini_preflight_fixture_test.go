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

func TestGeminiPermissionsNotesFixture_DocumentsPermissionChecksAndLedgerEvidence(t *testing.T) {
	fixturePath := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "permissions", "permissions-notes.md")

	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read permissions notes fixture: %v", err)
	}

	body := string(content)
	required := []string{
		"# Gemini Permissions Notes",
		"## Commands",
		"## Observations",
		"permission",
	}

	for _, token := range required {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(token)) {
			t.Fatalf("permissions notes must contain %q", token)
		}
	}

	logPath := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "commands.log")
	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read commands log fixture: %v", err)
	}

	if !strings.Contains(string(logContent), "permissions") {
		t.Fatalf("commands log must include permissions-related command entries")
	}
}

func TestGeminiWorkdirNotesFixture_DocumentsCwdChecksAndLedgerEvidence(t *testing.T) {
	fixturePath := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "workdir", "workdir-notes.md")

	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read workdir notes fixture: %v", err)
	}

	body := string(content)
	required := []string{
		"# Gemini Workdir Notes",
		"## Commands",
		"## Observations",
		"working directory",
	}

	for _, token := range required {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(token)) {
			t.Fatalf("workdir notes must contain %q", token)
		}
	}

	logPath := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "commands.log")
	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read commands log fixture: %v", err)
	}

	if !strings.Contains(string(logContent), "workdir") {
		t.Fatalf("commands log must include workdir-related command entries")
	}
}

func TestGeminiSchemaNotesFixture_DocumentsTokenCostAndModelObservations(t *testing.T) {
	fixturePath := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "schema-notes.md")

	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read schema notes fixture: %v", err)
	}

	body := string(content)
	required := []string{
		"# Gemini Schema Notes",
		"## Token and Cost Observations",
		"## Model Observations",
		"valid-model",
		"invalid-model",
	}
	for _, token := range required {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(token)) {
			t.Fatalf("schema notes must contain %q", token)
		}
	}
}
