---
id: claude-timeout-classification-tests
source_ideas: []
created: 2026-02-26
epic: test-quality
---

# Claude Client Timeout Classification Tests

## Problem

Timeout classification for invocation timeout, stall timeout, and bead timeout in the Claude client has no deterministic test coverage. Existing tests are either skipped permanently due to POSIX shell dependencies or rely on timing, making it impossible to reliably assert which timeout type is returned under each deadline scenario.

## Approach

- Extend the existing fake-binary test pattern (used elsewhere in `internal/claude/` or `internal/provider/`) to simulate each timeout scenario
- For **invocation timeout**: use a fake binary that sleeps past the invocation deadline; assert the returned error is classified as `InvocationTimeout`
- For **stall timeout**: use a fake binary that writes some output then hangs; assert classification as `StallTimeout`
- For **bead timeout**: use a context deadline that expires mid-run; assert classification as `BeadTimeout`
- Each test must pass deterministically under `go test -count=5` with no POSIX shell dependency (`/bin/sh -c` style strings are fine for the fake binary; the test itself must not depend on shell behavior for its assertions)
- Remove or replace any existing `t.Skip` placeholders covering these cases

## Files to Change

- `internal/claude/claude_test.go` or `internal/provider/claude_test.go` — add `TestClaudeClient_InvocationTimeoutClassification`, `TestClaudeClient_StallTimeoutClassification`, `TestClaudeClient_BeadTimeoutClassification`

## Acceptance Criteria

- Three deterministic tests, one per timeout type, using the fake-binary pattern
- Each test asserts the correct timeout classification in the returned error
- Tests pass with `go test -count=5` without flakiness
- No `t.Skip` calls in the new tests
- No POSIX shell dependency required for test assertions
