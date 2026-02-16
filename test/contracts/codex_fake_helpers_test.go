//go:build contract

package contracts

import "time"

const (
	codexFakeNotImplementedSentinel = "implemented"
	codexFailureExitCodeEnvVar      = "CODEX_FAIL"
	codexDelayEnvVar                = "CODEX_DELAY"
	codexFixtureRequiredErrorToken  = "CODEX_FIXTURE"
)

func testNowUnixMilli() int64 {
	return time.Now().UnixMilli()
}
