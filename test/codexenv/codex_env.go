package codexenv

import (
	"path/filepath"

	"github.com/danabrams/gromit/test/testutil"
)

const (
	// FixtureEnvVar is the environment variable used to configure CODEX fixtures.
	FixtureEnvVar = "CODEX_FIXTURE"
	// FailureEnvVar is the environment variable used to trigger CODEX failures.
	FailureEnvVar = "CODEX_FAIL"
	// DelayEnvVar controls CODEX latency in seconds or duration strings.
	DelayEnvVar = "CODEX_DELAY"
)

// ApplyFixtureEnv sets CODEX_FIXTURE to the normalized fixture path.
func ApplyFixtureEnv(env []string, fixtureFile string) []string {
	return applyCodexEnv(env, FixtureEnvVar, normalizeFixtureValue(fixtureFile))
}

// ApplyFailureEnv sets CODEX_FAIL to the provided exit code string.
func ApplyFailureEnv(env []string, exitCode string) []string {
	return applyCodexEnv(env, FailureEnvVar, exitCode)
}

// ApplyDelayEnv sets CODEX_DELAY to the provided delay string.
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
