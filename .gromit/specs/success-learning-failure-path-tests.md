---
id: success-learning-failure-path-tests
source_ideas: []
created: 2026-02-26
epic: test-quality
---

# Success-Learning Extraction Failure-Path Tests

## Problem

Success-learning extraction has no test for the failure path: when `bc.Result.Success` is false, the code should still run telemetry/logging without panicking or silently skipping. The absence of this test leaves the failure branch effectively untested.

## Approach

- Add a test that constructs a `BeadContext` (or equivalent) with `Result.Success = false` and passes it to the success-learning extraction function
- Assert that the function returns without error and that the telemetry/logging path executes (e.g., verify a log record is written or a telemetry counter is incremented, depending on what the implementation does)
- If the implementation simply returns early on failure, the test should assert that the early return path is taken cleanly (no panic, correct return values)
- Use mock or in-memory implementations for any external dependencies (logger, telemetry sink)

## Files to Change

- `internal/runner/` (whichever file contains success-learning extraction, likely `learnings.go` or `handler.go`) — add `TestSuccessLearningExtraction_FailedIteration` test in the corresponding `_test.go` file

## Acceptance Criteria

- Test constructs a `BeadContext` with `Result.Success = false` and invokes the extraction function
- Test asserts no panic and correct return value for the failure case
- If telemetry/logging runs on failure, the test verifies at least one log/telemetry event is recorded
- If the function short-circuits on failure, the test documents that behavior with an assertion
- Test passes with `go test ./internal/runner/...`
