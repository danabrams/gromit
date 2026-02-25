package toolcalls

import "strings"

const (
	codexFixtureEnvVar = "CODEX_FIXTURE"
	codexFailEnvVar    = "CODEX_FAIL"
	codexDelayEnvVar   = "CODEX_DELAY"
)

// ApplyCodexFixtureEnv applies the CODEX_FIXTURE environment variable to the given environment.
func ApplyCodexFixtureEnv(env []string, fixtureFile string) []string {
	return replaceOrAppend(append([]string{}, env...), codexFixtureEnvVar, fixtureFile)
}

// ApplyCodexFailEnv applies the CODEX_FAIL environment variable to the given environment.
func ApplyCodexFailEnv(env []string, exitCode string) []string {
	return replaceOrAppend(append([]string{}, env...), codexFailEnvVar, exitCode)
}

// ApplyCodexDelayEnv applies the CODEX_DELAY environment variable to the given environment.
func ApplyCodexDelayEnv(env []string, delay string) []string {
	return replaceOrAppend(append([]string{}, env...), codexDelayEnvVar, delay)
}

// replaceOrAppend replaces an environment variable if it exists, or appends it if it doesn't.
func replaceOrAppend(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}
