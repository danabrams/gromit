package fixtures_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type geminiCommandLogEntry struct {
	Timestamp time.Time
	Command   string
	ExitCode  int
	Category  string
}

func parseGeminiCommandLog(t *testing.T, content []byte) []geminiCommandLogEntry {
	t.Helper()

	var entries []geminiCommandLogEntry
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		const tsPrefix = "timestamp="
		const cmdPrefix = " command=\""
		const exitPrefix = "\" exit_code="

		if !strings.HasPrefix(line, tsPrefix) {
			t.Fatalf("commands log line does not match expected ledger format: %q", line)
		}
		rest := strings.TrimPrefix(line, tsPrefix)
		cmdStart := strings.Index(rest, cmdPrefix)
		if cmdStart < 0 {
			t.Fatalf("commands log line missing command section: %q", line)
		}
		tsText := rest[:cmdStart]
		rest = rest[cmdStart+len(cmdPrefix):]

		exitStart := strings.LastIndex(rest, exitPrefix)
		if exitStart < 0 {
			t.Fatalf("commands log line missing exit_code section: %q", line)
		}
		commandText := rest[:exitStart]
		exitAndCategory := rest[exitStart+len(exitPrefix):]
		if commandText == "" {
			t.Fatalf("commands log line has empty command text: %q", line)
		}

		exitCodeText := exitAndCategory
		category := ""
		if hashIdx := strings.Index(exitAndCategory, " # "); hashIdx >= 0 {
			exitCodeText = exitAndCategory[:hashIdx]
			category = exitAndCategory[hashIdx+3:]
		}

		ts, err := time.Parse(time.RFC3339, tsText)
		if err != nil {
			t.Fatalf("commands log timestamp %q is not RFC3339: %v", tsText, err)
		}
		exitCode, err := strconv.Atoi(strings.TrimSpace(exitCodeText))
		if err != nil {
			t.Fatalf("commands log exit code %q is not an int: %v", strings.TrimSpace(exitCodeText), err)
		}

		entries = append(entries, geminiCommandLogEntry{
			Timestamp: ts,
			Command:   commandText,
			ExitCode:  exitCode,
			Category:  strings.TrimSpace(category),
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan commands log: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("commands log must include at least one ledger entry")
	}
	return entries
}

func TestGeminiMarkdownFixtures_HaveMetadataHeaders(t *testing.T) {
	fixtures := []struct {
		parts   []string
		headers []string
	}{
		{
			parts: []string{"preflight.md"},
			headers: []string{
				"# provenance:",
				"# refresh:",
				"# Gemini Preflight",
				"## Checklist",
				"## Observed Results",
				"## Capture Harness",
			},
		},
		{
			parts: []string{"permissions", "permissions-notes.md"},
			headers: []string{
				"# provenance:",
				"# refresh:",
				"# Gemini Permissions Notes",
				"## Commands",
				"## Raw Evidence",
				"## Observations",
			},
		},
		{
			parts: []string{"workdir", "workdir-notes.md"},
			headers: []string{
				"# provenance:",
				"# refresh:",
				"# Gemini Workdir Notes",
				"## Commands",
				"## Raw Evidence",
				"## Observations",
			},
		},
		{
			parts: []string{"schema-notes.md"},
			headers: []string{
				"# provenance:",
				"# refresh:",
				"# Gemini Schema Notes",
				"## Token and Cost Observations",
				"## Model Observations",
				"## Stream-JSON Schema",
				"## JSON Schema",
			},
		},
		{
			parts: []string{"errors", "exit-codes-notes.md"},
			headers: []string{
				"# provenance:",
				"# refresh:",
				"# Gemini Exit Code Notes",
				"## Trigger Attempts",
				"## Observations",
			},
		},
	}

	for _, fixture := range fixtures {
		requireMarkdownHeaders(t, geminiFixturePath(fixture.parts...), fixture.headers)
	}
}

func TestGeminiPreflightFixture_IncludesChecklistAndObservedResults(t *testing.T) {
	requireMarkdownHeaders(t, geminiFixturePath("preflight.md"), []string{
		"# provenance:",
		"# refresh:",
		"# Gemini Preflight",
		"## Checklist",
		"## Observed Results",
		"## Capture Harness",
	})
}

func TestGeminiCommandsLogFixture_InitializedWithTimestampedLedgerEntries(t *testing.T) {
	fixturePath := geminiFixturePath("commands.log")

	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("failed to read commands log fixture: %v", err)
	}

	entries := parseGeminiCommandLog(t, content)
	for _, entry := range entries {
		if entry.Command == "" {
			t.Fatal("commands log entry must include command text")
		}
		if entry.Timestamp.IsZero() {
			t.Fatal("commands log entry must include a non-zero timestamp")
		}
	}

	containsVersionCheck := false
	for _, entry := range entries {
		if entry.Command == "gemini --version" {
			containsVersionCheck = true
			break
		}
	}
	if !containsVersionCheck {
		t.Fatal("commands log should include preflight version check command")
	}
}

func TestGeminiPreflightFixture_DocumentsCaptureHarnessCodeBlock(t *testing.T) {
	path := geminiFixturePath("preflight.md")

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read preflight fixture: %v", err)
	}

	body := string(content)
	if !strings.Contains(body, "```bash") {
		t.Fatalf("preflight fixture must include a bash code block in the Capture Harness section")
	}
}

func TestGeminiPermissionsNotesFixture_DocumentsPermissionChecksAndLedgerEvidence(t *testing.T) {
	path := geminiFixturePath("permissions", "permissions-notes.md")
	requireMarkdownHeaders(t, path, []string{
		"# provenance:",
		"# refresh:",
		"# Gemini Permissions Notes",
		"## Commands",
		"## Raw Evidence",
		"## Observations",
	})

	logPath := geminiFixturePath("commands.log")
	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read commands log fixture: %v", err)
	}

	entries := parseGeminiCommandLog(t, logContent)
	found := false
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Category), "permission") {
			found = true
			break
		}
		if strings.Contains(strings.ToLower(entry.Command), "permission") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("commands log must include permissions-related command entries")
	}
}

func TestGeminiWorkdirNotesFixture_DocumentsCwdChecksAndLedgerEvidence(t *testing.T) {
	path := geminiFixturePath("workdir", "workdir-notes.md")
	requireMarkdownHeaders(t, path, []string{
		"# provenance:",
		"# refresh:",
		"# Gemini Workdir Notes",
		"## Commands",
		"## Raw Evidence",
		"## Observations",
	})

	logPath := geminiFixturePath("commands.log")
	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read commands log fixture: %v", err)
	}

	entries := parseGeminiCommandLog(t, logContent)
	found := false
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Category), "workdir") {
			found = true
			break
		}
		if strings.Contains(strings.ToLower(entry.Command), "workdir") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("commands log must include workdir-related command entries")
	}
}

func TestGeminiSchemaNotesFixture_DocumentsTokenCostAndModelObservations(t *testing.T) {
	fixturePath := geminiFixturePath("schema-notes.md")

	requireMarkdownHeaders(t, fixturePath, []string{
		"# provenance:",
		"# refresh:",
		"# Gemini Schema Notes",
		"## Token and Cost Observations",
		"## Model Observations",
	})
}

func TestGeminiErrorsFixtures_CaptureCategorizedStderrSamples(t *testing.T) {
	dir := geminiFixturePath("errors")

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
	dir := geminiFixturePath("models")
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
	path := geminiFixturePath("errors", "exit-codes-notes.md")
	requireMarkdownHeaders(t, path, []string{
		"# provenance:",
		"# refresh:",
		"# Gemini Exit Code Notes",
		"## Trigger Attempts",
		"## Observations",
	})

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read exit code notes fixture: %v", err)
	}

	body := strings.ToLower(string(content))
	required := []string{
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
	logPath := geminiFixturePath("commands.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read commands log fixture: %v", err)
	}

	entries := parseGeminiCommandLog(t, content)
	categories := map[string]int{}
	for _, entry := range entries {
		if entry.Category != "" {
			categories[entry.Category]++
		}
	}
	for _, category := range []string{"model", "token-cost", "exit-error"} {
		if categories[category] == 0 {
			t.Fatalf("commands log must include categorized entries for %q", category)
		}
	}
}

func TestGeminiSchemaNotesFixture_ReferencesModelArtifactsAndTokenCostEvidence(t *testing.T) {
	path := geminiFixturePath("schema-notes.md")
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
	dir := geminiFixturePath("prompt-delivery")
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

func TestGeminiCommandsLogFixture_HasProvenanceAndRefreshHeaders(t *testing.T) {
	logPath := geminiFixturePath("commands.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read commands log fixture: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) < 2 {
		t.Fatal("commands log fixture must include at least two header lines")
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[0]), "# provenance:") {
		t.Fatalf("first commands log line = %q, want # provenance header", strings.TrimSpace(lines[0]))
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[1]), "# refresh:") {
		t.Fatalf("second commands log line = %q, want # refresh header", strings.TrimSpace(lines[1]))
	}
}

func TestGeminiStreamJSONSuccessFixture_ExistsWithJSONLRecords(t *testing.T) {
	path := geminiFixturePath("stream-json-success.jsonl")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read stream-json fixture: %v", err)
	}

	events := parseJSONLEvents(t, string(content))
	if len(events) < 2 {
		t.Fatalf("stream-json fixture must contain at least 2 jsonl records")
	}
	seenTypes := map[string]bool{}
	var finalRecord map[string]any
	for i, record := range events {
		typeValue, ok := record["type"].(string)
		if !ok || typeValue == "" {
			t.Fatalf("line %d must include non-empty string field %q", i+1, "type")
		}
		seenTypes[typeValue] = true
		finalRecord = record
	}
	if !seenTypes["init"] || !seenTypes["result"] {
		t.Fatal("stream-json fixture must include both init and result records")
	}
	if usage, ok := finalRecord["usage"].(map[string]any); ok {
		if _, ok := usage["input_tokens"].(float64); !ok {
			t.Fatal("final stream-json usage must include numeric input_tokens")
		}
		if _, ok := usage["output_tokens"].(float64); !ok {
			t.Fatal("final stream-json usage must include numeric output_tokens")
		}
	} else if stats, ok := finalRecord["stats"].(map[string]any); ok {
		if _, ok := stats["input_tokens"].(float64); !ok {
			t.Fatal("final stream-json stats must include numeric input_tokens")
		}
		if _, ok := stats["output_tokens"].(float64); !ok {
			t.Fatal("final stream-json stats must include numeric output_tokens")
		}
	} else {
		t.Fatal("final stream-json record must include usage or stats object")
	}
	if cost, ok := finalRecord["cost"].(map[string]any); ok {
		if _, ok := cost["total"].(float64); !ok {
			t.Fatal("final stream-json cost must include numeric total")
		}
	}

	commentLines := extractCommentLines(string(content))
	if len(commentLines) < 2 {
		t.Fatalf("stream-json fixture has %d comment lines, want at least 2", len(commentLines))
	}
	if !strings.HasPrefix(commentLines[0], "# provenance:") {
		t.Fatalf("first comment line = %q, want # provenance header", commentLines[0])
	}
	if !strings.HasPrefix(commentLines[1], "# refresh:") {
		t.Fatalf("second comment line = %q, want # refresh header", commentLines[1])
	}
}

func TestGeminiJSONSuccessFixture_ExistsWithUsageAndCostFields(t *testing.T) {
	path := geminiFixturePath("json-success.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read json-success fixture: %v", err)
	}

	type usageBlock struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	}
	type costBlock struct {
		Currency string  `json:"currency"`
		Total    float64 `json:"total"`
	}
	type geminiJSONFixture struct {
		Output       string     `json:"output"`
		Usage        usageBlock `json:"usage"`
		Cost         costBlock  `json:"cost"`
		Model        string     `json:"model"`
		FinishReason string     `json:"finish_reason"`
	}

	var fixture geminiJSONFixture
	if err := json.Unmarshal(content, &fixture); err != nil {
		t.Fatalf("json-success fixture must be valid JSON object: %v", err)
	}
	if fixture.Output == "" {
		t.Fatal("json-success fixture must include non-empty output")
	}
	if fixture.Usage.InputTokens <= 0 || fixture.Usage.OutputTokens <= 0 {
		t.Fatalf("json-success fixture must include positive token counts, got input=%d output=%d", fixture.Usage.InputTokens, fixture.Usage.OutputTokens)
	}
	if fixture.Cost.Total < 0 {
		t.Fatalf("json-success fixture must include non-negative total cost, got total=%f", fixture.Cost.Total)
	}
	if fixture.Model == "" || fixture.FinishReason == "" {
		t.Fatal("json-success fixture must include model and finish_reason")
	}
}

func TestGeminiSchemaNotesFixture_DocumentsPromptModesAndSchemaExtraction(t *testing.T) {
	path := geminiFixturePath("schema-notes.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read schema notes fixture: %v", err)
	}

	body := strings.ToLower(string(content))
	requireMarkdownHeaders(t, path, []string{
		"# Gemini Schema Notes",
		"## Prompt Mode Comparison",
		"## Stream-JSON Schema",
		"## JSON Schema",
	})
	for _, token := range []string{"message", "stats.input_tokens", "stats.output_tokens"} {
		if !strings.Contains(body, token) {
			t.Fatalf("schema notes must include schema marker %q", token)
		}
	}
}

func TestGeminiFixtures_DoNotReferenceLegacyPlansFixturePath(t *testing.T) {
	paths := []string{
		geminiFixturePath("preflight.md"),
		geminiFixturePath("commands.log"),
		geminiFixturePath("workdir", "workdir-notes.md"),
		geminiFixturePath("prompt-delivery", "prompt-file-ref.stdout.txt"),
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read fixture %q: %v", path, err)
		}
		if strings.Contains(string(content), ".gromit/plans/fixtures/gemini") {
			t.Fatalf("fixture %q must not reference legacy .gromit/plans/fixtures/gemini path", path)
		}
	}
}

func TestGeminiCommandsLogFixture_IncludesPromptDeliveryAndFixtureGenerationEntries(t *testing.T) {
	logPath := geminiFixturePath("commands.log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read commands log fixture: %v", err)
	}

	entries := parseGeminiCommandLog(t, content)
	promptDeliveryCount := 0
	seenFixtureWrite := map[string]bool{
		"stream-json-success.jsonl": false,
		"json-success.json":         false,
	}

	for _, entry := range entries {
		switch entry.Category {
		case "prompt-delivery":
			promptDeliveryCount++
		case "fixture-generation":
			for name := range seenFixtureWrite {
				if strings.Contains(entry.Command, name) {
					seenFixtureWrite[name] = true
				}
			}
		}
	}
	if promptDeliveryCount < 4 {
		t.Fatalf("commands log should include all prompt-delivery capture commands, got %d", promptDeliveryCount)
	}
	for name, seen := range seenFixtureWrite {
		if !seen {
			t.Fatalf("commands log must include fixture-generation command for %q", name)
		}
	}
}

func requireMarkdownHeaders(t *testing.T, path string, headers []string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %q: %v", path, err)
	}

	lines := strings.Split(string(content), "\n")
	found := make(map[string]bool, len(headers))
	type req struct {
		raw        string
		normalized string
	}
	reqs := make([]req, len(headers))
	for i, header := range headers {
		reqs[i] = req{
			raw:        header,
			normalized: strings.ToLower(header),
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, r := range reqs {
			if strings.HasPrefix(lower, r.normalized) {
				found[r.raw] = true
			}
		}
	}

	for _, r := range reqs {
		if !found[r.raw] {
			t.Fatalf("fixture %q missing required header %q", path, r.raw)
		}
	}
}
