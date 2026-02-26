---
id: parallel-post-success
source_ideas: []
created: 2026-02-08
epic: developer-experience
---

# Parallel Post-Success Stage Execution

## Specification

After validation passes in a Gromit iteration, two independent stages currently run sequentially: success learning extraction (haiku call, ~5-10s) and light code review (sonnet/opus call, longer). These stages read different inputs and write to different outputs, so there is no reason to serialize them.

When both `loop.learn_from_success` and `review.enabled` are true, the two stages run concurrently as separate goroutines. The iteration waits for both to complete before proceeding. When only one is enabled, it runs inline as today — no goroutine overhead for a single task.

The review's internal sub-step (re-validation if fixes were applied) remains sequential within the review goroutine. This doesn't affect learning extraction.

Both stages announce themselves at the start of execution so the user can see they're running concurrently. Console output may interleave between stages — this is acceptable since each stage emits only 1-3 discrete log lines with distinguishable content.

Error handling matches today's semantics: learning extraction failures are silently ignored, review failures log a warning. Neither stage's failure cancels the other. If the review's re-validation fails (review fixes broke validation), that error propagates up as it does today.

There is no config flag for parallelism. It is the default behavior when both stages are enabled.

## Acceptance Criteria

- When both `learn_from_success` and `review.enabled` are true, `extractSuccessLearning` and `runLightReview` execute concurrently after validation passes.
- Wall-clock time for the post-success phase is approximately `max(learning, review)` rather than `sum(learning, review)`.
- A failure in learning extraction does not prevent or delay the review, and vice versa.
- Review re-validation (when fixes are applied) still works correctly within its goroutine and its error still propagates to the caller.
- When only one stage is enabled, it runs without goroutine overhead.
- Existing tests continue to pass; new test confirms concurrent execution.

## Decisions

1. **No config flag.** Parallelism is strictly better when both stages are enabled. Individual stages already have their own enable/disable toggles (`loop.learn_from_success`, `review.enabled`). A third toggle would be configuration bloat with no use case for opting out.

2. **Interleaved output is acceptable.** Both stages use `r.log()` backed by the mutex-protected `syncWriter`. Output is thread-safe. Each stage's log lines are distinguishable by content (e.g. "Success learning extracted: ..." vs "Review: ..."), so interleaving doesn't cause confusion.

3. **Scoped to success learning + light review only.** This spec does not cover other parallelization candidates (precheck + scope check, post-success housekeeping). Those can be evaluated separately if this pattern proves effective.

4. **Use `sync.WaitGroup` or `errgroup.Group` for synchronization.** Standard Go concurrency primitives. `errgroup` is preferred since the review goroutine needs to propagate re-validation errors.

## Research & Context

### Current State

The post-success flow lives in `runValidation()` in `internal/runner/process.go` (lines 865-912). After `r.log("Validation passed")`, the two stages execute sequentially:

```go
r.extractSuccessLearning(ctx, bc)          // haiku call, 30s timeout

if r.cfg.Review.Enabled {
    reviewResult, err := r.runLightReview(...)  // sonnet/opus call
    // ... re-validate if fixes applied
    // ... create beads/backlog from findings
    // ... log review result
}
```

### Concurrency Infrastructure Already In Place

- `syncWriter` (`internal/runner/syncwriter.go`) — mutex-protected `io.Writer` for console output, already used by all `r.log()` calls.
- `StreamStats` (`internal/runner/stream.go`) — thread-safe stats with `sync.Mutex`.
- Goroutines already used for heartbeat monitoring, stdin piping, and startup monitoring.

### Thread Safety Considerations

- `r.log()` / `r.output`: Safe (mutex via `syncWriter`).
- `LEARNINGS.md` writes: Only written by `extractSuccessLearning`. Review does not write learnings. No conflict.
- `bc.result`: Only mutated by the review goroutine (appending re-validation output). Learning extraction doesn't touch it. No conflict.
- `bd` CLI calls (bead/backlog creation from review): Subprocess-based, independent of learning extraction.
- `r.beadStats`: Not accessed by either post-success stage. No conflict.
