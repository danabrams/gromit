//go:build contract

package contracts

import (
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
