//go:build contract

package contracts

import (
	"os/exec"
	"path/filepath"
	"time"

	"github.com/danabrams/gromit/test/testutil"
)

const (
	codexFixtureEnvVar             = "CODEX_FIXTURE"
	codexFailureExitCodeEnvVar     = "CODEX_FAIL"
	codexDelayEnvVar               = "CODEX_DELAY"
	codexFixtureRequiredErrorToken = "CODEX_FIXTURE"
)

func codexTestEnvWithFixture(baseEnv []string, fixtureFile string) []string {
	return testutil.ReplaceOrAppend(append([]string{}, baseEnv...), codexFixtureEnvVar, fixtureFile)
}

func codexTestEnvWithFailureExitCode(baseEnv []string, exitCode string) []string {
	return testutil.ReplaceOrAppend(append([]string{}, baseEnv...), codexFailureExitCodeEnvVar, exitCode)
}

func newCodexFakeCommand(testDir string, env []string, args ...string) *exec.Cmd {
	cmd := exec.Command(filepath.Join(fakesDir, "codex"), args...)
	cmd.Dir = testDir
	cmd.Env = env
	return cmd
}

func testNowUnixMilli() int64 {
	return time.Now().UnixMilli()
}
