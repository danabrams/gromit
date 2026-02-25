//go:build e2e

package e2e

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/test/testutil"
	"github.com/danabrams/gromit/test/toolcalls"
)

type ToolCallKind = toolcalls.ToolCallKind

const (
	ToolCallCodex  = toolcalls.ToolCallCodex
	ToolCallClaude = toolcalls.ToolCallClaude
	ToolCallBD     = toolcalls.ToolCallBD
)

const codexFixtureEnvVar = "CODEX_FIXTURE"

var E2EToolCallPrefixMap = toolcalls.ToolCallPrefixMap

func ApplyCodexFixtureEnvE2E(env []string, fixtureFile string) []string {
	fixtureValue := fixtureFile
	if absFixture, err := filepath.Abs(fixtureFile); err == nil {
		fixtureValue = absFixture
	}
	return testutil.ReplaceOrAppend(append([]string{}, env...), codexFixtureEnvVar, fixtureValue)
}

func FilterE2EToolCalls(env *e2eEnv, tool ToolCallKind) ([]string, error) {
	return toolcalls.FilterToolCalls(env.CallLog, tool)
}

func ToolCallPrefix(tool ToolCallKind) (string, error) {
	return toolcalls.ToolCallPrefix(tool)
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
