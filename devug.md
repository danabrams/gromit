# gromit-eimf Debug Continuation Log (Codex Path + Retry Diagnostics)

## 2026-02-15

### User request for this cycle
- Continue debugging.
- Fix Codex path behavior directly (do not route around Codex).
- Add retry/diagnostics and record all work in this file.

### Changes implemented

1. **Codex path hardening (`internal/provider/codex.go`)**
- Added environment preparation in both `Run()` and `StreamRun()`.
- `CODEX_HOME` behavior now:
  - if set and missing: create directory automatically
  - if set under temp dir (e.g. `/tmp/...`): rewrite to workspace local `./.codex-home`
  - inject effective value into command env for invocation

2. **Codex transient failure retry (`internal/provider/codex.go`)**
- Added one automatic retry for non-zero-exit Codex failures when stderr/stdout match transient infra patterns:
  - e.g. `stream disconnected`, connection reset/refused/timeout, 429/503, broken pipe

3. **Actionable invocation diagnostics (`internal/provider/codex.go`)**
- Added `Diagnostics string` to `provider.Result` (`internal/provider/provider.go`).
- Codex provider now records diagnostic metadata on result:
  - `codex_args=...`
  - effective `codex_home=...`
  - `stderr_head=` first 2KB
  - `stderr_tail=` last 2KB

4. **Surface diagnostics in ATDD failure path (`internal/runner/callbacks.go`)**
- ATDD invocation failure now includes `diagnostics=...` in error text.
- Output summarizer now keeps both start and end of long strings so tail errors remain visible.

### Tests added/updated

- `internal/provider/codex_test.go`
  - `TestCodexProviderRun_CreatesMissingCODEXHOME`
  - `TestCodexProviderRun_RetriesTransientFailureOnce`
  - `TestCodexProviderRun_FailureIncludesDiagnostics`

- `internal/runner/callbacks_test.go`
  - updated truncation expectation for new head+tail summarization marker

### Verification commands and outcomes

- `go test ./internal/provider -run 'TestCodexProviderRun_(RetriesTransientFailureOnce|FailureIncludesDiagnostics|CreatesMissingCODEXHOME|CapturesStdout|CapturesStderr)' -count=1` -> PASS
- `go test ./internal/runner -run 'TestMethodologyExec_InvokeFn_FailureIncludesProviderStderr|TestSummarizeATDDProviderOutput' -count=1` -> PASS
- `make build` -> PASS

### End-to-end forced run reproductions (after patch)

#### Run 1
- Command:
  - `env GOCACHE=/tmp/gocache-gromit CODEX_HOME=/tmp/codex-home-missing-e2e ./gromit run -c /tmp/gromit-eimf-unstick2.yaml -n 1`
- Log:
  - `.gromit/logs/run-20260215-162416.jsonl`
- Result:
  - failed in ATDD retry (Codex exit 1)
  - diagnostics now present and show:
    - `codex_args=exec --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check --color never --model gpt-5.3-codex -`
    - `codex_home=/home/dabrams/gromit/.codex-home`

#### Run 2 (after summarizer update)
- Command:
  - `env GOCACHE=/tmp/gocache-gromit CODEX_HOME=/tmp/codex-home-missing-e2e ./gromit run -c /tmp/gromit-eimf-unstick2.yaml -n 1`
- Log:
  - `.gromit/logs/run-20260215-163013.jsonl`
- Result:
  - failed in ATDD retry (Codex exit 1)
  - now clearly reveals core transport error in surfaced output:
    - `Reconnecting... 1/5 ...`
    - `... stream disconnected before completion: error sending request for url (https://api.openai.com/v1/responses)`

### Current root cause signal
- Path/bootstrap class is fixed (no more missing/invalid `CODEX_HOME` failures).
- Remaining blocker is Codex transport instability during ATDD retry:
  - repeated stream disconnects to `https://api.openai.com/v1/responses`
  - retries exhausted.

### Next recommended step
- Add provider fallback in ATDD retry when Codex fails with transient transport signatures:
  - mark Codex unavailable for cooldown
  - retry ATDD invocation through alternate provider (Claude) before failing bead.
