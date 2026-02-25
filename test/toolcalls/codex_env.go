package toolcalls

import "github.com/danabrams/gromit/test/codexenv"

func ApplyCodexFixtureEnv(env []string, fixtureFile string) []string {
	return codexenv.ApplyFixtureEnv(env, fixtureFile)
}

func ApplyCodexFailEnv(env []string, exitCode string) []string {
	return codexenv.ApplyFailureEnv(env, exitCode)
}

func ApplyCodexDelayEnv(env []string, delay string) []string {
	return codexenv.ApplyDelayEnv(env, delay)
}
