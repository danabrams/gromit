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

func TestGeminiErrorsFixtures_CaptureCategorizedStderrSamples(t *testing.T) {
	dir := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "errors")

	required := []string{
		"command-missing.stderr.txt",
		"exit-1.stderr.txt",
		"exit-42.stderr.txt",
		"exit-53.stderr.txt",
	}

	for _, name := range required {
		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read categorized stderr fixture %q: %v", name, err)
		}
		if strings.TrimSpace(string(content)) == "" {
			t.Fatalf("categorized stderr fixture %q must be non-empty", name)
		}
	}
}

func TestGeminiModelFixtures_CaptureValidAndInvalidModelRawArtifacts(t *testing.T) {
	dir := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "models")
	required := []string{
		"valid-model.stdout.txt",
		"valid-model.stderr.txt",
		"invalid-model.stdout.txt",
		"invalid-model.stderr.txt",
	}

	for _, name := range required {
		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read model artifact %q: %v", name, err)
		}
		if name == "valid-model.stderr.txt" || name == "invalid-model.stderr.txt" {
			if strings.TrimSpace(string(content)) == "" {
				t.Fatalf("stderr model artifact %q must be non-empty", name)
			}
		}
	}
}

func TestGeminiExitCodeNotesFixture_DocumentsConcreteTriggerAttempts(t *testing.T) {
	path := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "errors", "exit-codes-notes.md")

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read exit code notes fixture: %v", err)
	}

	body := strings.ToLower(string(content))
	required := []string{
		"# gemini exit code notes",
		"trigger attempts",
		"exit code 0",
		"exit code 1",
		"exit code 42",
		"exit code 53",
		"command=",
	}
	for _, token := range required {
		if !strings.Contains(body, token) {
			t.Fatalf("exit code notes must contain %q", token)
		}
	}
}

func TestGeminiCommandsLogFixture_IncludesModelTokenCostAndExitErrorEntries(t *testing.T) {
	logPath := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "commands.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read commands log fixture: %v", err)
	}

	body := strings.ToLower(string(content))
	required := []string{
		"# model",
		"# token-cost",
		"# exit-error",
	}
	for _, token := range required {
		if !strings.Contains(body, token) {
			t.Fatalf("commands log must include categorized entry %q", token)
		}
	}
}

func TestGeminiSchemaNotesFixture_ReferencesModelArtifactsAndTokenCostEvidence(t *testing.T) {
	path := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "schema-notes.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read schema notes fixture: %v", err)
	}

	body := strings.ToLower(string(content))
	required := []string{
		"models/valid-model.stderr.txt",
		"models/invalid-model.stderr.txt",
		"commands.log",
		"input_tokens",
		"output_tokens",
		"cost",
	}
	for _, token := range required {
		if !strings.Contains(body, token) {
			t.Fatalf("schema notes must reference %q", token)
		}
	}
}

func TestGeminiPromptDeliveryFixtures_CaptureRawArtifactsPerPromptMode(t *testing.T) {
	dir := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "prompt-delivery")
	required := []string{
		"inline-small.stdout.txt",
		"inline-small.stderr.txt",
		"inline-large.stdout.txt",
		"inline-large.stderr.txt",
		"stdin-pipe.stdout.txt",
		"stdin-pipe.stderr.txt",
		"prompt-file-ref.stdout.txt",
		"prompt-file-ref.stderr.txt",
	}

	for _, name := range required {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected prompt-delivery artifact %q: %v", name, err)
		}
	}
}

func TestGeminiStreamJSONSuccessFixture_ExistsWithJSONLRecords(t *testing.T) {
	path := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "stream-json-success.jsonl")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read stream-json fixture: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) < 2 {
		t.Fatalf("stream-json fixture must contain at least 2 jsonl records")
	}
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
			t.Fatalf("line %d must be a JSON object record", i+1)
		}
	}
}

func TestGeminiJSONSuccessFixture_ExistsWithUsageAndCostFields(t *testing.T) {
	path := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "json-success.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read json-success fixture: %v", err)
	}

	body := strings.ToLower(string(content))
	required := []string{
		"\"output\"",
		"\"usage\"",
		"\"input_tokens\"",
		"\"output_tokens\"",
		"\"cost\"",
	}
	for _, token := range required {
		if !strings.Contains(body, token) {
			t.Fatalf("json-success fixture must contain %q", token)
		}
	}
}

func TestGeminiSchemaNotesFixture_DocumentsPromptModesAndSchemaExtraction(t *testing.T) {
	path := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "schema-notes.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read schema notes fixture: %v", err)
	}

	body := strings.ToLower(string(content))
	required := []string{
		"prompt mode comparison",
		"inline -p",
		"stdin pipe",
		"@file",
		"stream-json schema",
		"json schema",
		"message_start",
		"usage.input_tokens",
	}
	for _, token := range required {
		if !strings.Contains(body, token) {
			t.Fatalf("schema notes must contain %q", token)
		}
	}
}

func TestGeminiCommandsLogFixture_IncludesPromptDeliveryAndFixtureGenerationEntries(t *testing.T) {
	logPath := filepath.Join("..", "..", ".gromit", "plans", "fixtures", "gemini", "commands.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read commands log fixture: %v", err)
	}

	body := strings.ToLower(string(content))
	required := []string{
		"# prompt-delivery",
		"gemini -p",
		"@.gromit/plans/fixtures/gemini/prompt-delivery/prompt-file-input.txt",
		"stream-json-success.jsonl",
		"json-success.json",
	}
	for _, token := range required {
		if !strings.Contains(body, token) {
			t.Fatalf("commands log must include %q", token)
		}
	}
}
