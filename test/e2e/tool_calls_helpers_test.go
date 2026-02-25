//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
)

func TestApplyCodexFixtureEnvE2E_ProvidesAbsolutePath(t *testing.T) {
	relFixture := filepath.Join("..", "fixtures", "codex_success.txt")
	absFixture, err := filepath.Abs(relFixture)
	if err != nil {
		t.Fatalf("failed to resolve absolute path for %s: %v", relFixture, err)
	}

	baseEnv := []string{"PATH=/tmp/bin", codexFixtureEnvVar + "=/old/path"}
	updatedEnv := ApplyCodexFixtureEnvE2E(baseEnv, relFixture)

	var gotFixture string
	for _, entry := range updatedEnv {
		if strings.HasPrefix(entry, codexFixtureEnvVar+"=") {
			gotFixture = strings.TrimPrefix(entry, codexFixtureEnvVar+"=")
			break
		}
	}

	if gotFixture != absFixture {
		t.Fatalf("CODEX_FIXTURE = %q, want %q", gotFixture, absFixture)
	}

	if baseEnv[1] != codexFixtureEnvVar+"=/old/path" {
		t.Fatalf("ApplyCodexFixtureEnvE2E mutated original env: got %v", baseEnv)
	}
}

func TestToolCallPrefixMapping(t *testing.T) {
	cases := map[ToolCallKind]string{
		ToolCallCodex:  "codex",
		ToolCallClaude: "claude",
		ToolCallBD:     "bd",
	}

	for kind, want := range cases {
		prefix, err := ToolCallPrefix(kind)
		if err != nil {
			t.Fatalf("ToolCallPrefix(%q) returned error: %v", kind, err)
		}
		if prefix != want {
			t.Fatalf("ToolCallPrefix(%q) = %q, want %q", kind, prefix, want)
		}
	}

	if _, err := ToolCallPrefix(ToolCallKind("unknown")); err == nil {
		t.Fatal("expected error for unknown ToolCallKind")
	}
}

func TestE2EToolCallPrefixMapExported(t *testing.T) {
	if E2EToolCallPrefixMap == nil {
		t.Fatal("E2EToolCallPrefixMap is nil")
	}

	if len(E2EToolCallPrefixMap) != 3 {
		t.Fatalf("expected 3 entries in E2EToolCallPrefixMap, got %d", len(E2EToolCallPrefixMap))
	}

	cases := map[ToolCallKind]string{
		ToolCallCodex:  "codex",
		ToolCallClaude: "claude",
		ToolCallBD:     "bd",
	}

	for kind, expected := range cases {
		prefix, ok := E2EToolCallPrefixMap[kind]
		if !ok {
			t.Fatalf("E2EToolCallPrefixMap does not contain %q", kind)
		}
		if prefix != expected {
			t.Fatalf("E2EToolCallPrefixMap[%q] = %q, want %q", kind, prefix, expected)
		}
	}
}

func TestReadE2ECallLog_TrimsWhitespace(t *testing.T) {
	env := setupE2E(t)

	callLog := "   codex run --model sonnet\n" +
		"\n" +
		"\tclaude -p --model sonnet\n"
	if err := os.WriteFile(env.CallLog, []byte(callLog), 0644); err != nil {
		t.Fatalf("failed to write call log: %v", err)
	}

	calls, err := readE2ECallLog(env)
	if err != nil {
		t.Fatalf("readE2ECallLog returned error: %v", err)
	}

	want := []string{"codex run --model sonnet", "claude -p --model sonnet"}
	if len(calls) != len(want) {
		t.Fatalf("expected %d entries, got %d (%v)", len(want), len(calls), calls)
	}
	for i, line := range want {
		if calls[i] != line {
			t.Fatalf("call %d = %q, want %q", i, calls[i], line)
		}
	}
}

func TestWriteE2ECallLog_WritesLines(t *testing.T) {
	env := setupE2E(t)

	lines := []string{
		"bd ready --json --limit 10",
		"codex run --model sonnet",
		"claude -p --model sonnet",
	}
	if err := writeE2ECallLog(env, lines...); err != nil {
		t.Fatalf("writeE2ECallLog returned error: %v", err)
	}

	data, err := os.ReadFile(env.CallLog)
	if err != nil {
		t.Fatalf("failed to read call log: %v", err)
	}

	want := strings.Join(lines, "\n") + "\n"
	if string(data) != want {
		t.Fatalf("call log = %q, want %q", string(data), want)
	}
}
