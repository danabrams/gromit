//go:build e2e

package e2e

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/test/testutil"
)

type ToolCallKind string

const (
	ToolCallCodex  ToolCallKind = "codex"
	ToolCallClaude ToolCallKind = "claude"
	ToolCallBD     ToolCallKind = "bd"
)

const codexFixtureEnvVar = "CODEX_FIXTURE"

var E2EToolCallPrefixMap = map[ToolCallKind]string{
	ToolCallCodex:  "codex",
	ToolCallClaude: "claude",
	ToolCallBD:     "bd",
}

func ApplyCodexFixtureEnvE2E(env []string, fixtureFile string) []string {
	fixtureValue := fixtureFile
	if absFixture, err := filepath.Abs(fixtureFile); err == nil {
		fixtureValue = absFixture
	}
	return testutil.ReplaceOrAppend(append([]string{}, env...), codexFixtureEnvVar, fixtureValue)
}

func FilterE2EToolCalls(env *e2eEnv, tool ToolCallKind) ([]string, error) {
	prefix, err := ToolCallPrefix(tool)
	if err != nil {
		return nil, err
	}
	return filterE2ECalls(env, prefix)
}

func ToolCallPrefix(tool ToolCallKind) (string, error) {
	prefix, ok := E2EToolCallPrefixMap[tool]
	if !ok {
		return "", fmt.Errorf("unknown tool call kind: %q", tool)
	}
	return prefix, nil
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

func writeE2ECallLog(env *e2eEnv, lines ...string) error {
	callLog := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(env.CallLog, []byte(callLog), 0644)
}
