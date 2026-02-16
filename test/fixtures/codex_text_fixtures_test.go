package fixtures_test

import (
	"strconv"
	"strings"
	"testing"
)

func TestCodexSuccessTextFixtureDeterministicTranscript(t *testing.T) {
	content := readFixtureFile(t, "codex_success.txt")
	lines := nonEmptyLines(content)

	if len(lines) != 6 {
		t.Fatalf("codex_success.txt has %d non-empty lines, want 6", len(lines))
	}
	if !strings.HasPrefix(lines[0], "# provenance:") {
		t.Fatalf("first non-empty line = %q, want '# provenance:' header", lines[0])
	}
	if !strings.HasPrefix(lines[1], "# refresh:") {
		t.Fatalf("second non-empty line = %q, want '# refresh:' header", lines[1])
	}

	keyValues := parseKeyValueLines(t, "codex_success.txt", lines[2:])
	expectedKeys := []string{"command", "status", "exit_code", "output_summary"}
	for i, key := range expectedKeys {
		if keyValues[i].key != key {
			t.Fatalf("key at index %d = %q, want %q", i, keyValues[i].key, key)
		}
	}

	if keyValues[1].value != "success" {
		t.Fatalf("status = %q, want success", keyValues[1].value)
	}
}

func TestCodexTextFixturesUseDeterministicKeyValueTranscript(t *testing.T) {
	// Expected failure: fixturecatalog.ParseDeterministicCodexKV() does not exist yet,
	// and codex text fixtures are still freeform text instead of deterministic key/value transcripts.
	tests := []struct {
		name           string
		fixtureName    string
		expectedStatus string
		expectedKeys   []string
	}{
		{
			name:           "success fixture has required deterministic keys in order",
			fixtureName:    "codex_success.txt",
			expectedStatus: "success",
			expectedKeys:   []string{"command", "status", "exit_code", "output_summary"},
		},
		{
			name:           "failure fixture has required deterministic keys in order",
			fixtureName:    "codex_failure.txt",
			expectedStatus: "failure",
			expectedKeys:   []string{"command", "status", "exit_code", "error_code", "error_message"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Expected failure: fixturecatalog.RequiredCodexTranscriptKeys() does not exist yet,
			// so fixtures fail until they adopt the deterministic transcript shape.
			content := readFixtureFile(t, tt.fixtureName)
			lines := nonEmptyLines(content)

			expectedLineCount := 2 + len(tt.expectedKeys)
			if len(lines) != expectedLineCount {
				t.Fatalf("fixture %q has %d non-empty lines, want %d", tt.fixtureName, len(lines), expectedLineCount)
			}

			if !strings.HasPrefix(lines[0], "# provenance:") {
				t.Fatalf("first non-empty line = %q, want '# provenance:' header", lines[0])
			}
			if !strings.HasPrefix(lines[1], "# refresh:") {
				t.Fatalf("second non-empty line = %q, want '# refresh:' header", lines[1])
			}

			keyValues := parseKeyValueLines(t, tt.fixtureName, lines[2:])
			for i, key := range tt.expectedKeys {
				if keyValues[i].key != key {
					t.Fatalf("fixture %q key at index %d = %q, want %q", tt.fixtureName, i, keyValues[i].key, key)
				}
				if keyValues[i].value == "" {
					t.Fatalf("fixture %q key %q must have a non-empty value", tt.fixtureName, key)
				}
			}

			if keyValues[1].value != tt.expectedStatus {
				t.Fatalf("fixture %q status = %q, want %q", tt.fixtureName, keyValues[1].value, tt.expectedStatus)
			}
		})
	}
}

func TestCodexTextFixturesHaveRequiredCommandAndExitFields(t *testing.T) {
	// Expected failure: fixturecatalog.ValidateCodexTextFixtureFields() does not exist yet,
	// and codex text fixtures do not consistently provide deterministic command/exit/error fields.
	tests := []struct {
		name                     string
		fixtureName              string
		expectedExitCode         int
		expectFailureErrorFields bool
	}{
		{
			name:             "success fixture contains command and zero exit code",
			fixtureName:      "codex_success.txt",
			expectedExitCode: 0,
		},
		{
			name:                     "failure fixture contains command and explicit error fields",
			fixtureName:              "codex_failure.txt",
			expectedExitCode:         1,
			expectFailureErrorFields: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Expected failure: fixturecatalog.LookupCodexTranscriptField() does not exist yet,
			// and current fixtures are not structured as deterministic field maps.
			content := readFixtureFile(t, tt.fixtureName)
			keyValues := parseKeyValueLines(t, tt.fixtureName, nonEmptyLines(content)[2:])
			fields := make(map[string]string, len(keyValues))
			for _, kv := range keyValues {
				fields[kv.key] = kv.value
			}

			command := fields["command"]
			if !strings.HasPrefix(command, "codex run --model sonnet") {
				t.Fatalf("fixture %q command = %q, want codex CLI transcript command", tt.fixtureName, command)
			}

			exitCode, err := strconv.Atoi(fields["exit_code"])
			if err != nil {
				t.Fatalf("fixture %q exit_code must be numeric: %v", tt.fixtureName, err)
			}
			if exitCode != tt.expectedExitCode {
				t.Fatalf("fixture %q exit_code = %d, want %d", tt.fixtureName, exitCode, tt.expectedExitCode)
			}

			if tt.expectFailureErrorFields {
				if strings.TrimSpace(fields["error_code"]) == "" {
					t.Fatalf("fixture %q must include non-empty error_code", tt.fixtureName)
				}
				if strings.TrimSpace(fields["error_message"]) == "" {
					t.Fatalf("fixture %q must include non-empty error_message", tt.fixtureName)
				}
			}
		})
	}
}

type keyValueLine struct {
	key   string
	value string
}

func nonEmptyLines(content string) []string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return filtered
}

func parseKeyValueLines(t *testing.T, fixtureName string, lines []string) []keyValueLine {
	t.Helper()
	parsed := make([]keyValueLine, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			t.Fatalf("fixture %q line %q must be key/value with ':' separator", fixtureName, line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		parsed = append(parsed, keyValueLine{key: key, value: value})
	}
	return parsed
}
