//go:build contract

package contracts

import (
	"os"
	"strings"
	"testing"

	"github.com/danabrams/gromit/test/toolcalls"
)

func TestFilterToolCalls_Codex(t *testing.T) {
	env := setupTestEnv(t)

	callLog := "claude -p --model sonnet\n" +
		"codex run --model sonnet\n"
	if err := os.WriteFile(env.CallLog, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	calls, err := FilterToolCalls(env, toolcalls.ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterToolCalls returned error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 codex call, got %d (%v)", len(calls), calls)
	}
}

func TestFilterToolCalls_IgnoresLeadingWhitespace(t *testing.T) {
	env := setupTestEnv(t)

	callLog := "claude -p --model sonnet\n" +
		"\tcodex run --model sonnet\n"
	if err := os.WriteFile(env.CallLog, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	calls, err := FilterToolCalls(env, toolcalls.ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterToolCalls returned error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 codex call, got %d (%v)", len(calls), calls)
	}
	if calls[0] != "codex run --model sonnet" {
		t.Fatalf("expected trimmed call, got %q", calls[0])
	}
}

func TestApplyCodexFixtureEnv(t *testing.T) {
	const fixture = "/tmp/codex-fixture.txt"

	env := []string{"PATH=/tmp/bin", "HOME=/tmp/home"}
	got := ApplyCodexFixtureEnv(env, fixture)

	found := false
	for _, v := range got {
		if v == codexFixtureEnvVar+"="+fixture {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("%s was not set to %q in env: %v", codexFixtureEnvVar, fixture, got)
	}
}

func TestApplyCodexFailEnv(t *testing.T) {
	const exitCode = "42"

	env := []string{"PATH=/tmp/bin", "HOME=/tmp/home"}
	got := ApplyCodexFailEnv(env, exitCode)

	found := false
	for _, v := range got {
		if v == codexFailureExitCodeEnvVar+"="+exitCode {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("%s was not set to %q in env: %v", codexFailureExitCodeEnvVar, exitCode, got)
	}
}

func TestApplyCodexDelayEnv(t *testing.T) {
	const delay = "5s"

	env := []string{"PATH=/tmp/bin", "HOME=/tmp/home"}
	got := ApplyCodexDelayEnv(env, delay)

	found := false
	for _, v := range got {
		if v == codexDelayEnvVar+"="+delay {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("%s was not set to %q in env: %v", codexDelayEnvVar, delay, got)
	}
}

func TestApplyCodexEnv_ComposesMultipleHelpers(t *testing.T) {
	const fixture = "/tmp/codex-fixture.txt"
	const exitCode = "42"
	const delay = "5s"

	env := []string{"PATH=/tmp/bin", "HOME=/tmp/home"}

	// Apply all three helpers in sequence
	env = ApplyCodexFixtureEnv(env, fixture)
	env = ApplyCodexFailEnv(env, exitCode)
	env = ApplyCodexDelayEnv(env, delay)

	// Verify all three are set
	foundFixture := false
	foundFailure := false
	foundDelay := false

	for _, v := range env {
		if v == codexFixtureEnvVar+"="+fixture {
			foundFixture = true
		}
		if v == codexFailureExitCodeEnvVar+"="+exitCode {
			foundFailure = true
		}
		if v == codexDelayEnvVar+"="+delay {
			foundDelay = true
		}
	}

	if !foundFixture {
		t.Fatalf("%s was not set to %q in env: %v", codexFixtureEnvVar, fixture, env)
	}
	if !foundFailure {
		t.Fatalf("%s was not set to %q in env: %v", codexFailureExitCodeEnvVar, exitCode, env)
	}
	if !foundDelay {
		t.Fatalf("%s was not set to %q in env: %v", codexDelayEnvVar, delay, env)
	}
}

func TestFilterToolCalls_UnknownKind(t *testing.T) {
	env := setupTestEnv(t)
	_, err := FilterToolCalls(env, toolcalls.ToolCallKind("unknown"))
	if err == nil {
		t.Fatal("expected an error for unknown tool kind")
	}
	if !strings.Contains(err.Error(), "unknown tool call kind") {
		t.Fatalf("unexpected error: %v", err)
	}
}
