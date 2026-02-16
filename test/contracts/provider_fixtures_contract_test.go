//go:build contract

package contracts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
)

func TestProviderContractFixtures_CodexTextFixturesHaveDeterministicCLIShape(t *testing.T) {
	// Expected failure: fixturecatalog.AssertCodexTextFixtureShape() does not exist yet,
	// and codex text fixtures have not been updated to the deterministic CLI transcript format.
	tests := []struct {
		name               string
		fixtureName        string
		requiredSubstrings []string
	}{
		{
			name:        "codex success text fixture",
			fixtureName: "codex_success.txt",
			requiredSubstrings: []string{
				"# provenance:",
				"# refresh:",
				"codex run --model sonnet",
				"status: success",
				"exit_code: 0",
			},
		},
		{
			name:        "codex failure text fixture",
			fixtureName: "codex_failure.txt",
			requiredSubstrings: []string{
				"# provenance:",
				"# refresh:",
				"codex run --model sonnet",
				"status: failure",
				"error_code:",
				"error_message:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Expected failure: fixturecatalog.CodexTextScenarios() does not exist yet,
			// so these subtests fail until codex text fixtures are normalized to this shape.
			path := filepath.Join(fixturesDir, tt.fixtureName)
			contentBytes, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read fixture %q: %v", tt.fixtureName, err)
			}
			content := string(contentBytes)

			for _, required := range tt.requiredSubstrings {
				if !strings.Contains(content, required) {
					t.Fatalf("fixture %q must contain %q", tt.fixtureName, required)
				}
			}

			lines := strings.Split(strings.TrimSpace(content), "\n")
			if len(lines) < 3 {
				t.Fatalf("fixture %q must include a two-line comment header plus body content", tt.fixtureName)
			}
			if !strings.HasPrefix(lines[0], "# provenance:") {
				t.Fatalf("fixture %q first line must be '# provenance:'", tt.fixtureName)
			}
			if !strings.HasPrefix(lines[1], "# refresh:") {
				t.Fatalf("fixture %q second line must be '# refresh:'", tt.fixtureName)
			}
		})
	}
}

func TestProviderContractFixtures_CodexTextFixturesUseDeterministicKeyValueTranscript(t *testing.T) {
	// Expected failure: fixturecatalog.ParseDeterministicCodexTranscript() does not exist yet,
	// and codex text fixtures are still freeform prose instead of ordered key/value transcripts.
	tests := []struct {
		name           string
		fixtureName    string
		expectedKeys   []string
		expectedStatus string
	}{
		{
			name:           "codex success uses stable key ordering",
			fixtureName:    "codex_success.txt",
			expectedKeys:   []string{"command", "status", "exit_code", "output_summary"},
			expectedStatus: "success",
		},
		{
			name:           "codex failure uses stable key ordering",
			fixtureName:    "codex_failure.txt",
			expectedKeys:   []string{"command", "status", "exit_code", "error_code", "error_message"},
			expectedStatus: "failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Expected failure: fixturecatalog.RequiredCodexTranscriptKeys() does not exist yet,
			// so fixtures fail this test until they adopt the deterministic transcript shape.
			path := filepath.Join(fixturesDir, tt.fixtureName)
			contentBytes, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read fixture %q: %v", tt.fixtureName, err)
			}

			rawLines := strings.Split(strings.TrimSpace(string(contentBytes)), "\n")
			lines := make([]string, 0, len(rawLines))
			for _, line := range rawLines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				lines = append(lines, trimmed)
			}

			minLines := 2 + len(tt.expectedKeys)
			if len(lines) != minLines {
				t.Fatalf("fixture %q must have %d non-empty lines (2 comment headers + %d key/value lines), got %d", tt.fixtureName, minLines, len(tt.expectedKeys), len(lines))
			}
			if !strings.HasPrefix(lines[0], "# provenance:") {
				t.Fatalf("fixture %q first non-empty line must start with '# provenance:'", tt.fixtureName)
			}
			if !strings.HasPrefix(lines[1], "# refresh:") {
				t.Fatalf("fixture %q second non-empty line must start with '# refresh:'", tt.fixtureName)
			}

			for i, expectedKey := range tt.expectedKeys {
				line := lines[i+2]
				parts := strings.SplitN(line, ":", 2)
				if len(parts) != 2 {
					t.Fatalf("fixture %q line %q must be key/value with ':' separator", tt.fixtureName, line)
				}

				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				if key != expectedKey {
					t.Fatalf("fixture %q key at position %d = %q, want %q", tt.fixtureName, i, key, expectedKey)
				}
				if value == "" {
					t.Fatalf("fixture %q key %q must have a non-empty value", tt.fixtureName, key)
				}
			}

			statusParts := strings.SplitN(lines[3], ":", 2)
			statusValue := strings.TrimSpace(statusParts[1])
			if statusValue != tt.expectedStatus {
				t.Fatalf("fixture %q status = %q, want %q", tt.fixtureName, statusValue, tt.expectedStatus)
			}
		})
	}
}

func TestProviderContractFixtures_ExistWithProvenance(t *testing.T) {
	// Expected failure: fixturecatalog.LoadCanonicalProviderFixtures() does not exist yet,
	// and canonical Codex/Claude fixture snapshots have not been added under test/fixtures.
	required := []string{
		"codex_success.txt",
		"codex_failure.txt",
		"codex_stream_success.jsonl",
		"codex_stream_failure.jsonl",
		"claude_stream_success.jsonl",
	}

	for _, name := range required {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(fixturesDir, name)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("expected canonical fixture %q to exist: %v", name, err)
			}

			if strings.TrimSpace(string(content)) == "" {
				t.Fatalf("expected fixture %q to contain realistic payload content", name)
			}

			lower := strings.ToLower(string(content))
			if !strings.Contains(lower, "provenance") {
				t.Fatalf("expected fixture %q to include a provenance comment for refresh workflow", name)
			}
		})
	}
}

func TestProviderContractFixtures_NamingIsScenarioDriven(t *testing.T) {
	pattern := regexp.MustCompile(`^(codex|claude)(?:_stream)?_(success|failure)\.(txt|jsonl)$`)
	required := []string{
		"codex_success.txt",
		"codex_failure.txt",
		"codex_stream_success.jsonl",
		"codex_stream_failure.jsonl",
		"claude_stream_success.jsonl",
	}

	for _, name := range required {
		if !pattern.MatchString(name) {
			t.Fatalf("required fixture %q does not match provider[_stream]_(success|failure).(txt|jsonl)", name)
		}
	}
}

func TestProviderContractFixtures_CodexStreamFixturesUseLifecycleAndErrorShapes(t *testing.T) {
	// Expected failure: fixturecatalog.AssertCodexJSONLStreamLifecycle() does not exist yet,
	// and codex stream fixtures do not yet encode the required start/delta/end ordering and explicit error termination.
	tests := []struct {
		name                        string
		fixtureName                 string
		requireLifecycleStart       bool
		requireTerminalErrorEvent   bool
		requireTerminalResultStatus bool
	}{
		{
			name:                  "codex stream success has start delta end lifecycle",
			fixtureName:           "codex_stream_success.jsonl",
			requireLifecycleStart: true,
		},
		{
			name:                      "codex stream failure ends with explicit error event",
			fixtureName:               "codex_stream_failure.jsonl",
			requireLifecycleStart:     true,
			requireTerminalErrorEvent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Expected failure: fixturecatalog.ParseCodexJSONLEvents() does not exist yet,
			// so fixture validation fails until codex stream fixtures adopt canonical JSONL event ordering.
			path := filepath.Join(fixturesDir, tt.fixtureName)
			contentBytes, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read fixture %q: %v", tt.fixtureName, err)
			}

			commentLines, events := parseJSONLFixture(t, string(contentBytes))
			if len(commentLines) < 2 {
				t.Fatalf("fixture %q must start with two comment lines (# provenance: and # refresh:)", tt.fixtureName)
			}
			if !strings.HasPrefix(commentLines[0], "# provenance:") {
				t.Fatalf("fixture %q first comment must start with '# provenance:'", tt.fixtureName)
			}
			if !strings.HasPrefix(commentLines[1], "# refresh:") {
				t.Fatalf("fixture %q second comment must start with '# refresh:'", tt.fixtureName)
			}

			if len(events) < 3 {
				t.Fatalf("fixture %q must contain at least 3 JSON events (start, delta, end/error)", tt.fixtureName)
			}

			if tt.requireLifecycleStart {
				firstType := eventType(events[0])
				if !isStartEventType(firstType) {
					t.Fatalf("fixture %q first event type = %q, want a start event type", tt.fixtureName, firstType)
				}

				hasDelta := false
				for _, event := range events[1 : len(events)-1] {
					if isDeltaEventType(eventType(event)) {
						hasDelta = true
						break
					}
				}
				if !hasDelta {
					t.Fatalf("fixture %q must include at least one delta/message event between start and terminal events", tt.fixtureName)
				}
			}

			last := events[len(events)-1]
			lastType := eventType(last)
			if tt.requireTerminalErrorEvent {
				if !isErrorEventType(lastType) {
					t.Fatalf("fixture %q terminal event type = %q, want explicit error event type", tt.fixtureName, lastType)
				}
				if !hasNonEmptyString(last, "error") {
					t.Fatalf("fixture %q terminal error event must include non-empty 'error' field", tt.fixtureName)
				}
			} else {
				if !isEndEventType(lastType) {
					t.Fatalf("fixture %q terminal event type = %q, want end/result event type", tt.fixtureName, lastType)
				}
				if hasNestedString(last, "result", "status") != "success" {
					t.Fatalf("fixture %q terminal result status must be success", tt.fixtureName)
				}
			}
		})
	}
}

func parseJSONLFixture(t *testing.T, content string) ([]string, []map[string]any) {
	t.Helper()

	rawLines := strings.Split(strings.TrimSpace(content), "\n")
	commentLines := make([]string, 0, 2)
	events := make([]map[string]any, 0, len(rawLines))

	for _, raw := range rawLines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			commentLines = append(commentLines, line)
			continue
		}

		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("invalid JSONL event line %q: %v", line, err)
		}
		events = append(events, event)
	}

	return commentLines, events
}

func eventType(event map[string]any) string {
	v, ok := event["type"]
	if !ok {
		return ""
	}
	typeStr, _ := v.(string)
	return typeStr
}

func isStartEventType(typ string) bool {
	switch typ {
	case "start", "stream_start", "thread.started", "response.started":
		return true
	default:
		return false
	}
}

func isDeltaEventType(typ string) bool {
	switch typ {
	case "delta", "assistant", "message.delta", "response.output_text.delta":
		return true
	default:
		return false
	}
}

func isEndEventType(typ string) bool {
	switch typ {
	case "end", "stream_end", "result", "response.completed":
		return true
	default:
		return false
	}
}

func isErrorEventType(typ string) bool {
	switch typ {
	case "error", "stream_error", "response.error":
		return true
	default:
		return false
	}
}

func hasNonEmptyString(event map[string]any, key string) bool {
	v, ok := event[key]
	if !ok {
		return false
	}
	s, ok := v.(string)
	return ok && strings.TrimSpace(s) != ""
}

func hasNestedString(event map[string]any, parent string, child string) string {
	v, ok := event[parent]
	if !ok {
		return ""
	}
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	childValue, _ := m[child].(string)
	return childValue
}

func newClaudeFixtureCommand(testDir string, env []string, args ...string) *exec.Cmd {
	cmd := exec.Command(filepath.Join(fakesDir, "claude"), args...)
	cmd.Dir = testDir
	cmd.Env = env
	return cmd
}

func TestProviderContractFixtures_AreConsumedByFakeCLI(t *testing.T) {
	// Expected failure: fixturecatalog.PathForProviderScenario() helper does not exist yet,
	// and contract tests still rely on inline fixture payloads instead of canonical snapshots.
	env := setupTestEnv(t)

	tests := []struct {
		name         string
		cmdArgs      []string
		envVarName   string
		fixtureName  string
		assertOutput func(t *testing.T, output string, fixture string)
	}{
		{
			name:        "codex plain success fixture",
			cmdArgs:     []string{"run", "--model", "sonnet"},
			envVarName:  codexFixtureEnvVar,
			fixtureName: "codex_success.txt",
			assertOutput: func(t *testing.T, output string, fixture string) {
				t.Helper()
				if strings.TrimSpace(output) != strings.TrimSpace(fixture) {
					t.Fatalf("codex plain output must match canonical success fixture")
				}
			},
		},
		{
			name:        "codex plain failure fixture",
			cmdArgs:     []string{"run", "--model", "sonnet"},
			envVarName:  codexFixtureEnvVar,
			fixtureName: "codex_failure.txt",
			assertOutput: func(t *testing.T, output string, fixture string) {
				t.Helper()
				if strings.TrimSpace(output) != strings.TrimSpace(fixture) {
					t.Fatalf("codex plain output must match canonical failure fixture")
				}
			},
		},
		{
			name:        "codex stream success fixture",
			cmdArgs:     []string{"run", "--jsonl", "--model", "sonnet"},
			envVarName:  codexFixtureEnvVar,
			fixtureName: "codex_stream_success.jsonl",
			assertOutput: func(t *testing.T, output string, fixture string) {
				t.Helper()
				if !strings.Contains(output, `"type":"assistant"`) || !strings.Contains(output, `"type":"result"`) {
					t.Fatalf("codex stream output should emit assistant and result JSON events")
				}
				if !strings.Contains(strings.ToLower(output), "provenance") {
					t.Fatalf("codex stream output should include canonical fixture payload content")
				}
			},
		},
		{
			name:        "codex stream failure fixture",
			cmdArgs:     []string{"run", "--jsonl", "--model", "sonnet"},
			envVarName:  codexFixtureEnvVar,
			fixtureName: "codex_stream_failure.jsonl",
			assertOutput: func(t *testing.T, output string, fixture string) {
				t.Helper()
				if !strings.Contains(output, `"type":"assistant"`) || !strings.Contains(output, `"type":"result"`) {
					t.Fatalf("codex stream output should emit assistant and result JSON events")
				}
				if !strings.Contains(strings.ToLower(output), "provenance") {
					t.Fatalf("codex stream output should include canonical fixture payload content")
				}
			},
		},
		{
			name:        "claude stream success fixture",
			cmdArgs:     []string{"stream-json", "--model", "sonnet"},
			envVarName:  "CLAUDE_FIXTURE",
			fixtureName: "claude_stream_success.jsonl",
			assertOutput: func(t *testing.T, output string, fixture string) {
				t.Helper()
				if !strings.Contains(output, `"type":"assistant"`) || !strings.Contains(output, `"type":"result"`) {
					t.Fatalf("claude stream output should emit assistant and result JSON events")
				}
				if !strings.Contains(strings.ToLower(output), "provenance") {
					t.Fatalf("claude stream output should include canonical fixture payload content")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixturePath := filepath.Join(fixturesDir, tt.fixtureName)
			fixtureContentBytes, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("failed to read canonical fixture %q: %v", tt.fixtureName, err)
			}

			testEnv := append([]string{}, env.Env...)
			testEnv = testutil.ReplaceOrAppend(testEnv, tt.envVarName, fixturePath)

			var output []byte
			if strings.HasPrefix(tt.name, "claude") {
				cmd := newClaudeFixtureCommand(env.Dir, testEnv, tt.cmdArgs...)
				cmd.Stdin = strings.NewReader("acceptance prompt input\n")
				output, err = cmd.CombinedOutput()
			} else {
				cmd := newCodexFakeCommand(env.Dir, testEnv, tt.cmdArgs...)
				cmd.Stdin = strings.NewReader("acceptance prompt input\n")
				output, err = cmd.CombinedOutput()
			}
			if err != nil {
				t.Fatalf("fake provider command failed for fixture %q: %v\noutput: %s", tt.fixtureName, err, string(output))
			}

			tt.assertOutput(t, string(output), string(fixtureContentBytes))
		})
	}
}
