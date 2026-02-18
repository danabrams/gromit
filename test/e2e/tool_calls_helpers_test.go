//go:build e2e

package e2e

import (
	"bufio"
	"fmt"
	"os"
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

func ApplyCodexFixtureEnvE2E(env []string, fixtureFile string) []string {
	return testutil.ReplaceOrAppend(append([]string{}, env...), codexFixtureEnvVar, fixtureFile)
}

func FilterE2EToolCalls(env *e2eEnv, tool ToolCallKind) ([]string, error) {
	prefix, err := toolCallPrefix(tool)
	if err != nil {
		return nil, err
	}
	return filterE2ECalls(env, prefix)
}

func toolCallPrefix(tool ToolCallKind) (string, error) {
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

func filterE2ECalls(env *e2eEnv, prefix string) ([]string, error) {
	calls, err := readE2ECallLog(env)
	if err != nil {
		return nil, err
	}

	var filtered []string
	for _, call := range calls {
		if strings.HasPrefix(call, prefix) {
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
		calls = append(calls, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return calls, nil
}
