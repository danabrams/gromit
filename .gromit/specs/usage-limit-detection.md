---
id: usage-limit-detection
source_ideas: [idea-1770817453360]
created: 2026-02-11
epic: provider-ecosystem
---

# Usage-Limit Error Detection for CLI Providers

## Specification

The multi-provider routing system (`multi-provider-routing` spec) requires each provider to implement `IsUsageLimitError(result *Result, err error) bool`. This function inspects the exit code, stderr, and stdout of a CLI invocation to determine whether the failure was caused by a usage/rate limit — as opposed to a code error, timeout, or other transient failure.

This spec defines:
1. The known error patterns for Claude CLI and Codex CLI when usage limits are hit
2. A structured test plan for empirically capturing any unknown patterns
3. The detection logic that `ClaudeProvider` and `CodexProvider` will use

### What We Already Know (Claude CLI)

From `internal/logger/stream.go`, Claude CLI's `--output-format stream-json` emits error events during streaming:

```json
{"type": "error", "subtype": "rate_limit"}
{"type": "error", "subtype": "rate_limited"}
{"type": "error", "subtype": "overloaded"}
```

These are **in-stream** events — they occur when the API rate-limits mid-conversation. The stream may recover (the CLI retries internally) or the invocation may ultimately fail. What we don't know is the **terminal** behavior:

- What exit code does Claude CLI return when the Max plan's usage cap is fully exhausted?
- What text appears on stderr when it can't start an invocation at all?
- Does stdout contain any structured error, or is it empty?
- Does the CLI distinguish between temporary rate limits (429 retryable) and hard usage caps (plan exhausted)?

### What We Need to Discover

For each provider CLI, capture the following under usage-limit conditions:

| Signal | Claude CLI | Codex CLI |
|--------|-----------|-----------|
| Exit code | ? | ? |
| stderr text pattern | ? | ? |
| stdout text pattern | ? | ? |
| stream-json error event | `rate_limit` / `rate_limited` / `overloaded` (known) | N/A or ? |
| Distinguishes soft vs hard limit? | ? | ? |

### Detection Logic

`IsUsageLimitError` should return `true` when ANY of these conditions match:

1. **Exit code match** — specific non-zero exit codes associated with usage limits
2. **Stderr pattern match** — case-insensitive substring match on stderr for known error phrases (e.g., "usage limit", "rate limit", "quota exceeded", "capacity", "overloaded")
3. **Stdout pattern match** — same substring matching on stdout (some CLIs emit errors on stdout)
4. **In-stream signal** — if `StreamStats.RateLimitHits > 0` and the invocation failed (non-zero exit), treat as usage-limit error

The function should be conservative: false negatives (missing a limit error) cause a failed retry; false positives (misclassifying a code error as a limit) cause unnecessary provider switching. False negatives are worse because they waste an escalation attempt.

### Test Plan for Empirical Capture

To fill in the unknowns, run the following experiments:

**Claude CLI (Max plan):**
1. Wait for or simulate usage cap exhaustion on a Max plan
2. Run `claude -p --model opus "echo hello" 2>stderr.txt >stdout.txt; echo "exit: $?"`
3. Run the same with `--output-format stream-json` to see if a terminal error event is emitted
4. Try with different models (opus, sonnet, haiku) — limits may differ
5. Record: exit code, full stderr, full stdout, any stream-json events

**Codex CLI:**
1. Wait for or simulate usage cap exhaustion
2. Run `codex --prompt /tmp/hello.txt 2>stderr.txt >stdout.txt; echo "exit: $?"`
3. Try with different models (o3, gpt-4o, gpt-4o-mini)
4. Record: exit code, full stderr, full stdout

**Interim detection (before empirical data):**
Use a broad heuristic as a starting implementation:
- Non-zero exit code AND stderr/stdout contains any of: `"usage limit"`, `"rate limit"`, `"quota"`, `"exceeded"`, `"capacity"`, `"overloaded"`, `"too many requests"`, `"429"`
- OR: `StreamStats.RateLimitHits > 0` AND invocation failed

This heuristic can be tightened once empirical patterns are captured.

## Acceptance Criteria

- `IsUsageLimitError` function exists for both `ClaudeProvider` and `CodexProvider` (or as a shared utility with provider-specific pattern lists)
- Claude detection matches known in-stream patterns (`rate_limit`, `rate_limited`, `overloaded`) plus any exit-code/stderr patterns discovered empirically
- Codex detection matches Codex-specific patterns discovered empirically
- A broad heuristic fallback catches unknown limit patterns by matching common limit-related keywords in stderr/stdout
- False positives are minimized — normal code errors (exit code 1 with test failures) must NOT trigger limit detection
- Empirical test results are documented in this spec (updated after capture) so future providers can follow the same pattern

## Decisions

1. **Conservative heuristic first, tighten later.** Since we can't reliably trigger usage limits on demand, start with a broad keyword-matching heuristic. False positives (treating a code error as a limit) are less harmful than false negatives (wasting retries on an exhausted provider). The heuristic will be refined once empirical data is captured.

2. **Combine exit code + output text, not just one signal.** Exit codes alone may be ambiguous (e.g., exit code 1 could mean many things). Requiring both a non-zero exit AND limit-related text in stderr/stdout reduces false positives.

3. **Reuse in-stream detection for StreamRun.** For invocations that use `StreamRun` with stream-json, the existing `StreamStats.RateLimitHits` counter provides a strong signal. If rate limit events were observed AND the invocation ultimately failed, that's a usage-limit error.

4. **Provider-specific pattern lists, shared matching logic.** The `IsUsageLimitError` implementation should use a shared pattern-matching utility that takes a list of known patterns. Each provider supplies its own pattern list. This keeps the matching logic DRY while allowing provider-specific patterns.

## Research & Context

### Current State

- `internal/claude/claude.go` — `Run()` and `StreamRun()` capture exit codes and stderr but don't inspect them for usage-limit patterns. Failed invocations return `Result{Success: false, ExitCode: N}`.
- `internal/logger/stream.go:362-369` — `isRateLimitEvent()` detects in-stream rate limit events by matching subtypes `"overloaded"`, `"rate_limit"`, `"rate_limited"`. This is used for counting/logging only.
- `internal/runner/runner.go:742-827` — `executeWithRetry()` handles invocation failures but has no usage-limit-specific path. It retries, escalates, or stops based on generic failure analysis.
- `internal/runner/process.go:185` — `RateLimitHits` from stream stats are recorded in the iteration result but not acted upon.

### Multi-Provider Routing Dependency

The `multi-provider-routing` spec defines the `Provider` interface at `.gromit/specs/multi-provider-routing.md:127-134`. `IsUsageLimitError` is called after each failed invocation to decide whether to mark the provider as unavailable and fall back to another provider (lines 194-199).

### Known Claude CLI Error Subtypes

From stream-json observation and the codebase:
- `"overloaded"` — API is overloaded (temporary)
- `"rate_limit"` — rate limited (temporary, 429-style)
- `"rate_limited"` — variant spelling of the same

These may or may not correspond to the terminal exit behavior when the Max plan cap is fully exhausted (as opposed to a momentary 429).
