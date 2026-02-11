---
created: 2026-02-11T00:00:00Z
decomposed: true
decomposed_at: "2026-02-11T09:13:58-05:00"
id: usage-limit-detection
source_spec: usage-limit-detection
---

# Usage-Limit Error Detection Implementation Plan

**Goal:** Detect usage/rate-limit errors from CLI providers so the runner can skip useless escalation and (later) fall back to alternative providers.

**Architecture:** Standalone `internal/usagelimit` package with shared pattern-matching logic and provider-specific keyword lists, wired into the runner's retry loop to short-circuit escalation when a plan-wide usage cap is hit.

**Tech Stack:** Go, table-driven tests

**Spec:** `.gromit/specs/usage-limit-detection.md`

---

## Architecture

**Overview:**
Create a standalone `internal/usagelimit` package with a pure-function `Check()` that implements the spec's two-path heuristic. Wire it into `executeWithRetry()` so usage-limit failures skip analysis and escalation. This package will later be consumed by `ClaudeProvider`/`CodexProvider` from the `multi-provider-routing` spec.

**Key Components:**

1. **`internal/usagelimit/detect.go`** — Core detection logic. `Signals` struct holds invocation outcomes (exit code, output text, rate limit hits). `Patterns` struct holds provider-specific keyword lists. `Check()` implements the heuristic. `ClaudePatterns()` and `CodexPatterns()` supply per-provider keywords.

2. **Runner integration** — In `executeWithRetry()`, after a failed invocation, check `usagelimit.Check()` before `analyzeAndHandleFailure()`. Short-circuits retry/escalation when limit detected.

3. **Logger integration** — `UsageLimited bool` added to `IterationResult` and `IterationLog` for observability.

**Detection Heuristic (from spec):**
- Path 1: Non-zero exit code AND case-insensitive match on any of: `"usage limit"`, `"rate limit"`, `"quota"`, `"exceeded"`, `"capacity"`, `"overloaded"`, `"too many requests"`, `"429"`
- Path 2: `RateLimitHits > 0` AND non-zero exit code (in-stream rate limit events observed and invocation ultimately failed)

**Integration Points:**
- `executeWithRetry()` in `internal/runner/runner.go` — new check between invocation result and failure analysis
- `IterationResult` — new `UsageLimited bool` field
- `IterationLog` — new `UsageLimited bool` field
- `writeIterationLog()` — propagates field from result to log

**Data Flow:**
```
Failed invocation → Signals{ExitCode, Output, RateLimitHits}
                  → usagelimit.Check(signals, ClaudePatterns())
                  → true: set UsageLimited, return error (no retry/escalate)
                  → false: proceed to analyzeAndHandleFailure() as before
```

**Files to Create:**
- `internal/usagelimit/detect.go` — Detection logic, patterns, check function
- `internal/usagelimit/detect_test.go` — Comprehensive unit tests

**Files to Modify:**
- `internal/runner/runner.go` — Usage-limit check in `executeWithRetry()`, `UsageLimited` field on `IterationResult`
- `internal/logger/logger.go` — `UsageLimited` field on `IterationLog`

**Tradeoffs:**
- Standalone package over inline: reusable by future provider implementations, easier to test
- Not modifying StreamRun to capture stderr: RateLimitHits + exit code is sufficient signal for streaming invocations; can add stderr capture later if empirical data warrants it
- Short-circuiting escalation: all models share plan limits, so escalating wastes time; false positives are less harmful than false negatives per spec

## Test Strategy

**Unit Tests** (`internal/usagelimit/detect_test.go`):
Table-driven tests for `Check()` covering:
- Each heuristic keyword with non-zero exit → true
- Case insensitivity ("Rate Limit", "RATE LIMIT") → true
- RateLimitHits > 0 with non-zero exit → true
- Zero exit code with keyword in output → false
- Non-zero exit with no keyword and 0 hits → false
- False positive prevention: "FAIL: TestUserLogin" with exit 1 → false
- False positive prevention: "build failed" with exit 1 → false
- Combined signals (keyword + hits) → true
- Empty output, zero hits, non-zero exit → false

**Mocking Strategy:**
- No mocking needed — `Check()` is a pure function taking value types
- Runner integration uses existing mock infrastructure

**Coverage Goals:**
- Every keyword tested
- Both detection paths tested independently and together
- Common false positive scenarios verified

## Implementation Tasks

### Task 1: Create core detection logic

**Files:**
- Create: `internal/usagelimit/detect.go`

**What to Do:**
Define `Signals` struct with `ExitCode int`, `Output string`, `RateLimitHits int`. Define `Patterns` struct with `Keywords []string`. Implement `Check(signals Signals, patterns Patterns) bool` with two detection paths: (1) non-zero exit AND case-insensitive keyword substring match in Output, (2) non-zero exit AND RateLimitHits > 0. Add `ClaudePatterns()` returning keywords from spec. Add `CodexPatterns()` returning same keyword set (to be refined with empirical data).

**Acceptance Criteria:**
- `Check` returns true when exit code is non-zero AND output contains a keyword (case-insensitive)
- `Check` returns true when exit code is non-zero AND RateLimitHits > 0
- `Check` returns false when exit code is 0 regardless of other signals

**Dependencies:** None

### Task 2: Add comprehensive unit tests

**Files:**
- Create: `internal/usagelimit/detect_test.go`

**What to Do:**
Table-driven tests with `t.Run` subtests covering all heuristic keywords, both detection paths, false positive prevention (test failures, build errors, lint output), case insensitivity, edge cases (empty output, zero hits), and combined signals. Follow existing test patterns in the codebase.

**Acceptance Criteria:**
- Every keyword in the heuristic list has a passing test case
- False positive cases (normal test/build/lint failures) return false
- Both detection paths (keyword match, RateLimitHits) tested independently

**Dependencies:** Task 1

### Task 3: Wire into runner and logger

**Files:**
- Modify: `internal/runner/runner.go` — add `UsageLimited bool` to `IterationResult`, add usage-limit check in `executeWithRetry()`
- Modify: `internal/logger/logger.go` — add `UsageLimited bool` to `IterationLog`

**What to Do:**
In `executeWithRetry()`, after checking `claudeResult.Success` is false and before `analyzeAndHandleFailure()`, construct `usagelimit.Signals` from the result and stream stats, call `usagelimit.Check()` with `ClaudePatterns()`. If true, set `bc.result.UsageLimited = true`, set `bc.result.Error` to a descriptive message, log a warning, and return false. Add `UsageLimited bool` to `IterationResult` and `IterationLog`. In `writeIterationLog()`, propagate the field.

**Acceptance Criteria:**
- Usage-limit failures skip `analyzeAndHandleFailure` and escalation in `executeWithRetry`
- `UsageLimited` field appears in JSONL iteration logs when triggered
- Normal failures continue through existing analysis/escalation path unchanged

**Dependencies:** Task 1

---

## Notes

- The heuristic keyword list is intentionally broad per spec decision 1. It will be tightened once empirical data from the test plan in the spec is captured.
- The `CodexPatterns()` function currently returns the same generic keyword list as Claude. It will be updated with Codex-specific patterns after the Codex CLI spike (gromit-zyc8) completes.
- StreamRun stderr is not captured — RateLimitHits from stream events provides equivalent signal for streaming invocations. If empirical testing reveals hard caps that emit errors only to stderr before streaming starts, `StreamRun` can be modified to capture stderr via `io.MultiWriter`.
- This package is designed to be consumed by the future `Provider` interface's `IsUsageLimitError()` method. The `Check()` function signature is deliberately provider-agnostic.
