//go:build e2e

package e2e

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
)

type ToolCallKind string

const (
	ToolCallCodex  ToolCallKind = "codex"
	ToolCallClaude ToolCallKind = "claude"
	ToolCallBD     ToolCallKind = "bd"
)

const codexFixtureEnvVar = "CODEX_FIXTURE"

func ApplyCodexFixtureEnvE2E(env []string, fixtureFile string) []string {
	return testutil.ReplaceOrAppend(append([]string{}, env...), codexFixtureEnvVar, fixtureFile)
}

func FilterE2EToolCalls(env *e2eEnv, tool ToolCallKind) ([]string, error) {
	prefix, err := ToolCallPrefix(tool)
	if err != nil {
		return nil, err
	}
	return filterE2ECalls(env, prefix)
}

func ToolCallPrefix(tool ToolCallKind) (string, error) {
	switch tool {
	case ToolCallCodex:
		return "codex", nil
	case ToolCallClaude:
		return "claude", nil
	case ToolCallBD:
		return "bd", nil
	default:
		return "", fmt.Errorf("unknown tool call kind: %q", tool)
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

func filterE2ECalls(env *e2eEnv, prefix string) ([]string, error) {
	calls, err := readE2ECallLog(env)
	if err != nil {
		return nil, err
	}

	var filtered []string
	for _, call := range calls {
		trimmed := strings.TrimSpace(call)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, prefix) {
			filtered = append(filtered, call)
		}
	}

	return filtered, nil
}

func readE2ECallLog(env *e2eEnv) ([]string, error) {
	f, err := os.Open(env.CallLog)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var calls []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "" {
			continue
		}
		calls = append(calls, trimmed)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return calls, nil
}
