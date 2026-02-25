//go:build contract

package contracts

import "github.com/danabrams/gromit/test/toolcalls"

func ApplyCodexFixtureEnv(env []string, fixtureFile string) []string {
	return codexTestEnvWithFixture(env, fixtureFile)
}

func ApplyCodexFailEnv(env []string, exitCode string) []string {
	return codexTestEnvWithFailureExitCode(env, exitCode)
}

func ApplyCodexDelayEnv(env []string, delay string) []string {
	return codexTestEnvWithDelay(env, delay)
}

func FilterToolCalls(env *testEnv, tool toolcalls.ToolCallKind) ([]string, error) {
	prefix, err := toolcalls.ToolCallPrefix(tool)
	if err != nil {
		return nil, err
	}
	return filterCalls(env, prefix)
}
