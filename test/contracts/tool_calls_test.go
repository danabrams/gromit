//go:build contract

package contracts

import (
	"os"
	"strings"
	"testing"
)

func TestFilterToolCalls_Codex(t *testing.T) {
	env := setupTestEnv(t)

	callLog := "claude -p --model sonnet\n" +
		"codex run --model sonnet\n"
	if err := os.WriteFile(env.CallLog, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	calls, err := FilterToolCalls(env, ToolCallCodex)
	if err != nil {
		t.Fatalf("FilterToolCalls returned error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 codex call, got %d (%v)", len(calls), calls)
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

func TestFilterToolCalls_UnknownKind(t *testing.T) {
	env := setupTestEnv(t)
	_, err := FilterToolCalls(env, ToolCallKind("unknown"))
	if err == nil {
		t.Fatal("expected an error for unknown tool kind")
	}
	if !strings.Contains(err.Error(), "unknown tool call kind") {
		t.Fatalf("unexpected error: %v", err)
	}
}
