package codexenv

import (
	"path/filepath"

	"github.com/danabrams/gromit/test/testutil"
)

const (
	FixtureEnvVar = "CODEX_FIXTURE"
	FailureEnvVar = "CODEX_FAIL"
	DelayEnvVar   = "CODEX_DELAY"
)

func ApplyFixtureEnv(env []string, fixtureFile string) []string {
	return applyCodexEnv(env, FixtureEnvVar, normalizeFixtureValue(fixtureFile))
}

func ApplyFailureEnv(env []string, exitCode string) []string {
	return applyCodexEnv(env, FailureEnvVar, exitCode)
}

func ApplyDelayEnv(env []string, delay string) []string {
	return applyCodexEnv(env, DelayEnvVar, delay)
}

func applyCodexEnv(env []string, key, value string) []string {
	cloned := append([]string{}, env...)
	return testutil.ReplaceOrAppend(cloned, key, value)
}

func normalizeFixtureValue(fixture string) string {
	if fixture == "" {
		return fixture
	}
	if abs, err := filepath.Abs(fixture); err == nil {
		return abs
	}
	return fixture
}
