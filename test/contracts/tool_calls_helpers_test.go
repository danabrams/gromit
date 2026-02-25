//go:build contract

package contracts

import "fmt"

type ToolCallKind string

const (
	ToolCallCodex  ToolCallKind = "codex"
	ToolCallClaude ToolCallKind = "claude"
	ToolCallBD     ToolCallKind = "bd"
)

func ApplyCodexFixtureEnv(env []string, fixtureFile string) []string {
	return codexTestEnvWithFixture(env, fixtureFile)
}

func ApplyCodexFailEnv(env []string, exitCode string) []string {
	return codexTestEnvWithFailureExitCode(env, exitCode)
}

func FilterToolCalls(env *testEnv, tool ToolCallKind) ([]string, error) {
	prefix, err := toolCallPrefix(tool)
	if err != nil {
		return nil, err
	}
	return filterCalls(env, prefix)
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
