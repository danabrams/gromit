# Scenario Test Writer Resilience Design

**Date:** 2026-03-17
**Status:** Approved

## Problem

Run `run-94196d329c7dae1d` (spec `0003a-infrastructure-failure-detection`) blocked because the LLM scenario test writer returned a response without the `===TEST_FILE_PATH===` marker. The strict parser returned an error, which immediately broke out of the retry loop — no retries were attempted, the run blocked.

Two root causes:

1. **No fuzzy fallback:** The parser is strict; if the LLM uses markdown fences instead of `===` markers, it fails immediately.
2. **Parse errors bypass retries:** The retry loop in `write_scenario_tests.go` was designed for compilation failures. Parse failures from `WriteScenarioTest` break out of the loop with no retry.

## Design

### Mitigation 1 — Fuzzy fallback parser (`llm_scenario_test_writer.go`)

`parseScenarioTestResponse` gains a fallback path that fires only when strict marker parsing fails.

**Fallback algorithm:**
1. Find the first ` ```go ` fence in the response.
2. Extract the content between the opening and closing fence as the test file content.
3. Extract the file path from the line immediately before the fence (if it ends in `.go`) or from a `// path/to/file_test.go` comment at the top of the fence body.
4. If both path and content are extracted, return them (success).
5. If either is missing, return the original strict-parse error.

The strict parse always runs first. The fallback is only attempted when markers are absent.

### Mitigation 2 — Retry parse failures (`write_scenario_tests.go`)

The existing retry loop passes `compileErrors` back to the LLM on failed compilation attempts. Parse errors receive the same treatment:

- When `WriteScenarioTest` returns an error containing `"parse scenario test response:"`, treat it as a retryable format error rather than a fatal break.
- Pass the error string as `compileErrors` for the next attempt (the existing prompt section "Prior Compilation Errors / fix these in your next attempt" covers format errors too).
- Max retries unchanged (2 retries, 3 total attempts).
- All other errors (e.g., `invoke llm:`, `write test file:`) remain fatal and break immediately.

## Scope

Two files, ~40 lines changed:

- `internal/next/contract/llm_scenario_test_writer.go` — fuzzy fallback in `parseScenarioTestResponse`
- `internal/next/specloop/stages/write_scenario_tests.go` — retry parse errors in the retry loop

No interface changes. No new types.

## Testing

- Unit tests for `parseScenarioTestResponse` covering: strict markers (existing), markdown fence with path-before-fence, markdown fence with path-comment inside fence, missing both markers and fence (returns original error).
- Unit test for the retry loop in `write_scenario_tests_test.go`: parse error on first attempt, success on second attempt.
