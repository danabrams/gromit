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
