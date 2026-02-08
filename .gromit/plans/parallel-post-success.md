---
created: 2026-02-08T00:00:00Z
decomposed: true
decomposed_at: "2026-02-08T08:04:19-05:00"
id: parallel-post-success
source_spec: parallel-post-success
---

# Parallel Post-Success Stage Execution Implementation Plan

**Goal:** Run success learning extraction and light code review concurrently after validation passes, reducing wall-clock time from `sum(learning, review)` to `max(learning, review)`.

**Architecture:** Use `errgroup.Group` to launch both stages as goroutines when both are enabled. Learning extraction swallows errors (returns nil). Review propagates re-validation errors via errgroup. When only one stage is enabled, it runs inline without goroutine overhead.

**Tech Stack:** Go, `golang.org/x/sync/errgroup`

**Spec:** `.gromit/specs/parallel-post-success.md`

---

## Architecture

**Overview:**
Replace the sequential post-success block in `runValidation()` with a conditional concurrent fork using `errgroup.Group`. Both stages already operate on disjoint state and use thread-safe logging via `syncWriter`.

**Key Components:**
1. **`runValidation()` refactor** (`internal/runner/process.go`): The post-success block (lines 865-912) becomes a three-way branch: both enabled → errgroup, one enabled → inline, neither → skip.

**Integration Points:**
- Modifies the post-success section of `runValidation()` in `internal/runner/process.go`
- Adds `golang.org/x/sync/errgroup` as a new dependency
- No new files created
- No changes to `extractSuccessLearning` or `runLightReview` signatures or internals

**Data Flow:**
1. Validation passes → check which stages are enabled
2. **Both enabled:** Create `errgroup.Group`. Launch learning extraction in one goroutine (errors swallowed internally, returns nil). Launch review in another goroutine (captures `reviewResult` and returns re-validation error if any). `g.Wait()` blocks until both complete; returns first non-nil error (the review re-validation error).
3. **One enabled:** Run inline, exactly as today — no goroutine overhead.
4. **Neither enabled:** Skip both, as today.

**Files to Modify:**
- `internal/runner/process.go` — refactor post-success block to use errgroup
- `internal/runner/process_test.go` — add concurrent execution tests
- `go.mod` / `go.sum` — add `golang.org/x/sync` dependency

**Tradeoffs:**
- **`errgroup.Group` vs `sync.WaitGroup`**: Chose errgroup because the review goroutine needs to propagate re-validation errors. WaitGroup doesn't handle errors.
- **No config flag**: Parallelism is strictly better when both are enabled. Individual stages already have their own enable/disable toggles.
- **Interleaved output accepted**: Both stages use `r.log()` → `syncWriter` (mutex-protected). Each stage's log lines are distinguishable by content.

## Test Strategy

**Unit Tests:**
- Verify concurrent execution via timing (both stages get artificial delays; concurrent should complete in ~max time, not sum)
- Verify learning failure doesn't block review and vice versa
- Verify review re-validation error propagates through errgroup
- Verify single-stage-enabled cases run without goroutine overhead (inline)

**Key Test Cases:**
1. Both enabled, both succeed — concurrent execution confirmed by timing
2. Both enabled, learning fails — review still completes, no error returned
3. Both enabled, review re-validation fails — error propagates to caller, learning still completes
4. Only learning enabled — runs inline, review skipped
5. Only review enabled — runs inline, learning skipped

**Mocking Strategy:**
- Use existing `mockClaudeClient` with `RunFn` that injects `time.Sleep` delays to measure concurrency
- Use existing `mockPromptRenderer` with `RenderLearnFn` / `RenderReviewFn` for controlling behavior
- Use existing `learnings.NewFile(t.TempDir())` pattern for learning file setup

**Test Organization:**
- All tests in `internal/runner/process_test.go`
- Follow existing naming pattern: `TestRunValidation_*` or `TestPostSuccess_*`

## Implementation Tasks

### Task 1: Add errgroup dependency and refactor post-success block

**Files:**
- Modify: `internal/runner/process.go`
- Modify: `go.mod` / `go.sum` (via `go get golang.org/x/sync`)

**What to Do:**
1. Add `golang.org/x/sync/errgroup` dependency via `go get`
2. In `runValidation()`, after `r.log("Validation passed")` (line 866), replace the sequential post-success block (lines 868-910) with:
   - Check if both `r.cfg.Loop.ShouldLearnFromSuccess()` and `r.cfg.Review.Enabled` are true
   - **Both enabled:** Create `errgroup.Group`. Launch `extractSuccessLearning` in one goroutine (always returns nil — learning errors are already swallowed internally). Launch the full review block (runLightReview + re-validation + applyReviewResult + writeReviewLog) in another goroutine, returning re-validation errors. Call `g.Wait()` and return its error.
   - **Only learning enabled:** Call `r.extractSuccessLearning(ctx, bc)` inline (as today)
   - **Only review enabled:** Run the review block inline (as today)
   - **Neither:** Skip (as today — implicit via the conditionals)
3. The review goroutine must capture `reviewResult`, `beadsCreated`, `backlogCreated`, and `reviewDuration` in local variables within the goroutine closure, then handle re-validation, applyReviewResult, and writeReviewLog within the same goroutine.

**Acceptance Criteria:**
- When both stages are enabled, they execute concurrently via errgroup
- When only one stage is enabled, it runs inline without goroutine overhead
- Review re-validation errors propagate correctly through errgroup to the caller

**Dependencies:** None

**Notes:**
- `extractSuccessLearning` already handles all its own errors internally (silent swallow). The goroutine wrapper just calls it and returns nil.
- The review block's local variables (`reviewResult`, `reviewStart`, etc.) must be declared inside the goroutine closure to avoid data races.
- `r.log()`, `r.applyReviewResult()`, and `r.writeReviewLog()` are all thread-safe (log via syncWriter, others are subprocess-based or write to logger).

### Task 2: Add unit tests for concurrent execution

**Files:**
- Modify: `internal/runner/process_test.go`

**What to Do:**
Add test functions that exercise the parallel post-success flow:

1. **`TestPostSuccess_BothEnabled_ConcurrentExecution`**: Both `learn_from_success` and `review.enabled` true. Mock Claude's `Run` (learning) and `runLightReview` path to each sleep 100ms. Assert total duration is ~100-150ms (concurrent), not ~200ms+ (sequential).

2. **`TestPostSuccess_LearningFailure_ReviewStillCompletes`**: Learning extraction's Claude call returns an error. Review still completes successfully. No error returned from runValidation.

3. **`TestPostSuccess_ReviewRevalidationError_Propagates`**: Review applies fixes, re-validation fails. Assert runValidation returns the re-validation error. Learning extraction still completes (verify via log output or learnings file).

4. **`TestPostSuccess_OnlyLearningEnabled`**: `review.enabled = false`. Verify learning runs, review doesn't. No errgroup used (verify via timing — should be fast since only one stage).

5. **`TestPostSuccess_OnlyReviewEnabled`**: `learn_from_success = false`. Verify review runs, learning doesn't.

**Acceptance Criteria:**
- All 5 test cases pass
- Timing-based test confirms concurrent execution (wall-clock ≈ max, not sum)
- Error propagation test confirms re-validation errors reach the caller

**Dependencies:** Task 1

**Notes:**
- Use the existing mock infrastructure (`mockClaudeClient`, `mockPromptRenderer`, `mockBeadClient`) from `interfaces_test.go`
- For timing tests, use `time.Sleep` in mock Claude calls and assert duration with reasonable tolerance (e.g., < 180ms for 100ms parallel tasks)
- The tests exercise `runValidation` indirectly through `processBead` or can test the post-success section by calling `runValidation` directly if it's accessible (it's a method on `Runner`, same package)

---

## Notes

- `golang.org/x/sync/errgroup` is a well-established Go module, widely used for exactly this pattern. It's the standard library recommendation for "run N goroutines, collect errors."
- The `extractSuccessLearning` method already swallows all errors internally (lines 276-347 in process.go), so wrapping it in a goroutine that returns nil is safe and preserves existing semantics.
- No changes are needed to `syncWriter`, `extractSuccessLearning`, or `runLightReview` — only the orchestration code in `runValidation` changes.
- The spec explicitly states interleaved console output is acceptable, so no buffering or output serialization is needed.
