---
id: session-epilogue
source_ideas: []
created: 2026-02-18
---

# Session Epilogue: Auto-Review and Retro After N Iterations

## Specification

After a configurable number of build iterations, gromit automatically runs a three-phase epilogue: fix broken tests, run a thorough code review (creating beads and backlog items), and run a retrospective that auto-applies its proposals. The entire epilogue is non-interactive and fully automated.

A new `session` config block controls this behavior. When `session.iterations` is set, the bead loop stops after that many successful iterations and `finishRun()` executes the epilogue before session completion (git push/sync).

### 1. Configuration

New `session` block in `gromit.yaml`:

```yaml
session:
  iterations: 10              # build iterations before epilogue (0 = disabled)
  test_command: "go test -vet=off -count=1 ./..."  # full test suite for epilogue
  max_fix_retries: 3          # Claude invocations to fix failing tests
  fix_tier: sonnet            # provider tier for fix attempts
  review: true                # run non-interactive thorough review
  retro: true                 # run non-interactive retro with auto-apply
```

`session.iterations` is separate from `loop.max_iterations`. The loop config controls the raw bead loop ceiling; the session config adds the epilogue after the loop ends. When `session.iterations > 0`, it acts as the iteration limit for the bead loop (overriding `loop.max_iterations` for stopping purposes). When `session.iterations` is 0 or absent, behavior is unchanged.

### 2. Test-Fix Phase

After the bead loop completes:

1. Run `session.test_command` via shell exec.
2. If all tests pass, log success and proceed to review.
3. If tests fail:
   a. Capture stdout+stderr (test failure output).
   b. Build a prompt with the test output, project CLAUDE.md, and RULES.md asking the provider to fix the failures.
   c. Invoke the provider at `session.fix_tier`.
   d. Re-run `session.test_command`.
   e. Repeat up to `session.max_fix_retries` times.
4. If tests still fail after all retries, log the residual failures and create beads for them. Do not block the review and retro phases.

The fix invocation reuses the existing provider infrastructure with a focused test-failure prompt rather than a bead's acceptance criteria.

### 3. Review Phase

Run a non-interactive thorough review:

1. Get the git diff since the last review commit (same as existing `reviewer.RunThorough`).
2. Render the thorough review prompt and invoke the provider at high tier.
3. Parse the `ReviewResult` from the output.
4. Apply the result: create beads for issues found, create backlog items for lower-priority findings.
5. Log the review results.

This reuses the existing `reviewpkg.Reviewer` infrastructure. The epilogue calls the reviewer in the same way that periodic thorough reviews work during the loop, but unconditionally (not gated by iteration frequency).

### 4. Retro Phase

Run a non-interactive retrospective with auto-apply:

1. Create a `retro.Retro` instance with the runner's provider.
2. Call `retro.Run(ctx, nil)` to run the full analysis (no bead filter — all beads).
3. Call `retro.ParseProposals(result.Analysis)` to extract structured proposals.
4. Auto-apply each proposal type:
   - **Consolidations**: Merge the specified learnings in LEARNINGS.md into a single consolidated learning.
   - **Promotions**: Append the proposed rule text to the appropriate section in RULES.md, then archive the source learning.
   - **Archives**: Remove the specified learning from LEARNINGS.md (move to archived section or delete).
   - **Rule changes**: Find the current rule text in RULES.md and replace with the proposed text.
5. Save LEARNINGS.md and RULES.md.
6. Create beads for any action items in the analysis that aren't covered by the proposals.
7. Record the retro in state (same as the CLI `gromit retro` command).
8. Log the retro results.

### 5. Integration with `finishRun()`

Current `finishRun()` flow, with the epilogue inserted:

```
finishRun(ctx, st):
  1. Log completion
  2. Final full validation (existing)
  3. Update global stats
  4. Set clean exit in state
  5. [NEW] runSessionEpilogue(ctx, st)  — if session.iterations > 0
  6. Check retro suggestion (skipped if epilogue ran retro)
  7. Session completion (git push/sync)
```

The epilogue runs after stats update and clean-exit marking but before session completion, so all epilogue artifacts (new beads, updated RULES.md, updated LEARNINGS.md) get committed and pushed in the final session completion step.

## Acceptance Criteria

- `session.iterations` in gromit.yaml controls the number of build iterations before the epilogue runs.
- When `session.iterations` is 0 or absent, behavior is identical to today.
- After the configured iterations, the test-fix phase runs `session.test_command` and attempts to fix failures up to `session.max_fix_retries` times.
- After the test-fix phase, a non-interactive thorough review runs and creates beads/backlog items.
- After the review, a non-interactive retro runs with auto-apply: consolidations, promotions, archives, and rule changes are applied to LEARNINGS.md and RULES.md.
- All epilogue artifacts (beads, file changes) are committed and pushed in session completion.
- `go test ./internal/runner/... -count=1` passes.
- `go test ./internal/retro/... -count=1` passes.

## Decisions

1. **Epilogue in `finishRun()`, not a wrapper command.** Keeps everything in-process with access to the runner's provider, beads client, state, and logger. Avoids the overhead and context loss of shelling out to separate CLI commands.

2. **`session.iterations` separate from `loop.max_iterations`.** The loop config is a safety ceiling; the session config is a deliberate workflow cadence. They serve different purposes even though both stop the loop.

3. **Test-fix before review.** The review should see clean code. Reviewing code with known test failures produces noise. Fix first, then review the final state.

4. **Non-interactive for all phases.** The epilogue is designed for fully autonomous operation. Interactive sessions belong in `gromit review` and `gromit retro` CLI commands, not in the automated loop.

5. **Auto-apply retro proposals.** The retro's structured proposals (consolidations, promotions, archives, rule changes) are applied directly rather than creating beads. This keeps the knowledge base current without requiring a separate interactive session. The proposals are already structured and validated by the LLM analysis.

6. **Don't block review/retro on test failures.** If tests can't be fully fixed, create beads for the remaining failures and proceed. The review and retro still provide value on the work that was done.

7. **Separate test command from validation commands.** The epilogue test command may be broader than the fast/full validation commands used during the loop. The loop optimizes for speed; the epilogue can afford a thorough test suite.

## Research & Context

### Current State

`finishRun()` lives in `internal/runner/run_init.go:169-193`. It runs final validation, updates stats, saves state, checks retro suggestion, and runs session completion.

The thorough review infrastructure lives in `internal/runner/reviewpkg/reviewer.go`. `RunThorough()` handles diff computation, prompt rendering, provider invocation, result parsing, and bead/backlog creation.

The retro infrastructure lives in `internal/retro/retro.go`. `Run()` handles learnings loading, LLM filtering, stats aggregation, prompt rendering, and analysis. `ParseProposals()` extracts structured proposals. `LaunchClaudeCode()` handles the interactive session (not used here).

### New Code

- `internal/runner/epilogue.go` — `runSessionEpilogue()`, `runTestFixLoop()`, `runEpilogueReview()`, `runEpilogueRetro()` methods on Runner
- `internal/config/config.go` — `SessionConfig` struct added to `Config`
- `internal/retro/apply.go` — auto-apply logic for proposals (apply consolidations, promotions, archives, rule changes to files)
- `internal/learnings/file.go` — `Consolidate()` and `Archive()` methods if not already present

### Modified Code

- `internal/runner/run_init.go` — `finishRun()` calls `runSessionEpilogue()`; `shouldStopLoop()` respects `session.iterations`
- `gromit.yaml` — new `session` block
