---
created: 2026-02-18T00:00:00Z
decomposed: true
decomposed_at: "2026-02-18T01:50:51Z"
id: session-epilogue
source_spec: session-epilogue
---

# Session Epilogue Implementation Plan

**Goal:** After a configurable number of build iterations, automatically run a three-phase epilogue: fix broken tests, run thorough code review, and run retrospective with auto-apply — all non-interactive.

**Architecture:** A new `session` config block gates the epilogue. `finishRun()` calls `runSessionEpilogue()` which sequences the three phases. Test-fix uses shell exec + provider fix invocations. Review reuses existing `reviewer.RunThorough()`. Retro reuses existing `retro.Run()` + `ParseProposals()` with new `ApplyProposals()` auto-apply logic.

**Tech Stack:** Go (shell exec, provider routing, template rendering, file manipulation)

**Spec:** `.gromit/specs/session-epilogue.md`

---

## Architecture

**Overview:**
Add a `session` config block controlling iteration count and epilogue behavior. When `session.iterations > 0`, the bead loop stops after that many successful iterations and `finishRun()` runs the epilogue before session completion. The epilogue sequences three phases: test-fix, review, retro.

**Key Components:**
1. **`SessionConfig`** (`internal/config/config.go`): Config struct with `iterations`, `test_command`, `max_fix_retries`, `fix_tier`, `review` (`*bool`), `retro` (`*bool`). Wired into `Config`, `SetDefaults()`, and `NormalizeNilFields()`.
2. **`runSessionEpilogue()`** (`internal/runner/epilogue.go`): Orchestrator method on `*Runner`. Sequences test-fix → review → retro. Gated on `cfg.Session.Iterations > 0`.
3. **`runTestFixLoop()`** (`internal/runner/epilogue.go`): Runs `session.test_command` via `r.runCmd()`. On failure, renders fix prompt, invokes provider via router at `fix_tier`, re-runs tests. Repeats up to `max_fix_retries`. Creates beads for residual failures.
4. **`runEpilogueReview()`** (`internal/runner/epilogue.go`): Calls `r.reviewer.RunThorough()` unconditionally.
5. **`runEpilogueRetro()`** (`internal/runner/epilogue.go`): Creates `retro.Retro` via `NewRetroWithProvider()` wrapping a router-selected provider, calls `Run()`, `ParseProposals()`, then `ApplyProposals()`. Records retro in state.
6. **`ApplyProposals()`** (`internal/retro/apply.go`): Applies each proposal type using existing learnings methods (`Replace()` for consolidations, `Archive()` for archives) and direct RULES.md text manipulation (for promotions and rule changes).

**Integration Points:**
- `finishRun()` in `run_init.go` calls `runSessionEpilogue()` after clean-exit marking, before retro suggestion check
- When epilogue runs retro, `checkRetroSuggestion()` is skipped
- `session.iterations` overrides `loop.max_iterations` for stopping when set and non-zero
- Session completion (`git push/sync`) runs after epilogue, committing all artifacts

**Data Flow:**
1. Bead loop runs for `session.iterations` successful iterations
2. `finishRun()` → `runSessionEpilogue()`
3. Test-fix: shell exec test command → capture output → render fix prompt → invoke provider → re-run tests → repeat or create beads
4. Review: `reviewer.RunThorough()` → diff, prompt, invoke, parse, create beads/backlog
5. Retro: `retro.Run()` → analysis → `ParseProposals()` → `ApplyProposals()` → update LEARNINGS.md + RULES.md
6. Session completion commits and pushes all changes

**Files to Modify:**
- `internal/config/config.go` — Add `SessionConfig` struct, wire into `Config`
- `internal/runner/run_init.go` — Insert epilogue call in `finishRun()`, skip retro suggestion when epilogue ran
- `internal/runner/runner.go` — Respect `session.iterations` in loop stopping
- `gromit.yaml` — Add `session:` block

**Files to Create:**
- `internal/runner/epilogue.go` — Epilogue methods on `*Runner`
- `internal/runner/epilogue_test.go` — Epilogue orchestration and test-fix tests
- `internal/retro/apply.go` — `ApplyProposals()` function
- `internal/retro/apply_test.go` — Apply proposal tests
- `.gromit/templates/PROMPT_test_fix.md` — Test-fix prompt template

**Tradeoffs:**
- Reuse `RunThorough()` directly rather than duplicating review logic
- Wrap router into `retro.ProviderRunner` adapter rather than modifying retro's interface
- Shell exec via `r.runCmd()` for test command (simple, testable via mock `cmdRunnerFn`)
- `*bool` for review/retro flags (default true, matches existing config patterns)

## Test Strategy

**Unit Tests** (`internal/retro/apply_test.go`):
- `ApplyProposals()` with each proposal type in isolation (consolidation, promotion, archive, rule change)
- Mixed proposals applied in one call
- Error handling: missing learning hash, missing RULES.md section, file write errors
- Empty proposals is a no-op

**Unit Tests** (`internal/runner/epilogue_test.go`):
- Orchestration: all three phases run in order when enabled
- Phase skipping: review=false skips review, retro=false skips retro
- Disabled: skipped when `session.iterations == 0`
- Test-fix: tests pass on first try (no fix attempts)
- Test-fix: tests fail, fix succeeds on retry
- Test-fix: all retries exhausted, creates beads, does not block review/retro
- Review: delegates to `reviewer.RunThorough()`
- Retro: runs analysis, parses proposals, applies them, records in state

**Unit Tests** (`internal/config/config_test.go`):
- `SessionConfig` defaults applied by `SetDefaults()`
- YAML deserialization of `session:` block
- Zero/absent iterations means disabled

**Mocking Strategy:**
- Mock `cmdRunnerFn` for test command execution
- Mock provider via router for fix/retro invocations
- Mock `reviewer.RunThorough()` for review delegation
- Temp files for RULES.md and LEARNINGS.md in apply tests
- `NewRunnerWithDeps` pattern for runner method tests

## Implementation Tasks

### Task 1: Add SessionConfig to config

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`
- Modify: `gromit.yaml`

**What to Do:**
Add `SessionConfig` struct with fields: `Iterations int`, `TestCommand string`, `MaxFixRetries int`, `FixTier string`, `Review *bool`, `Retro *bool` (all with yaml tags). Add `Session SessionConfig` to `Config` struct. In `SetDefaults()`, set `MaxFixRetries=3`, `FixTier="sonnet"`, and default `Review`/`Retro` to `true` (using `*bool` pattern). Add commented `session:` block to `gromit.yaml` showing all fields with explanatory comments.

**Acceptance Criteria:**
- `SessionConfig` struct exists with correct yaml tags
- `SetDefaults()` applies defaults for `MaxFixRetries`, `FixTier`, `Review`, `Retro`
- Config tests verify defaults and YAML deserialization

**Dependencies:** None

### Task 2: Implement ApplyProposals in retro package

**Files:**
- Create: `internal/retro/apply.go`
- Create: `internal/retro/apply_test.go`

**What to Do:**
Create `ApplyProposals(proposals *Proposals, lf *learnings.File, rulesPath string) error` function. For consolidations: call `lf.Replace(hashes, consolidatedText, category)`. For archives: call `lf.Archive(hash, rationale)`. For promotions: read RULES.md, find the target section header, append the proposed rule text after the section's existing content, write RULES.md, then call `lf.Archive(hash, "promoted to rule")`. For rule changes: read RULES.md, find `currentRule` text, replace with `proposedRule`, write RULES.md. Call `lf.Save()` once at the end. Handle errors per-proposal (log and continue) so one bad proposal doesn't block others.

**Acceptance Criteria:**
- Consolidations call `Replace()` with correct hashes and merged text
- Archives call `Archive()` with hash and rationale
- Promotions append rule to correct RULES.md section and archive the learning
- Rule changes find-and-replace in RULES.md
- Errors in individual proposals are logged but don't block others

**Dependencies:** None

### Task 3: Create test-fix prompt template

**Files:**
- Create: `.gromit/templates/PROMPT_test_fix.md`
- Modify: `internal/prompt/prompt.go`
- Test: `internal/prompt/prompt_test.go`

**What to Do:**
Create `PROMPT_test_fix.md` template that includes: project CLAUDE.md content, RULES.md content, the test command that was run, the test failure output (stdout+stderr), and instructions to fix the failing tests without changing test expectations. Add `TestFixContext` struct to prompt package with fields: `ClaudeMD`, `Rules`, `TestCommand`, `TestOutput`. Add `RenderTestFix(ctx *TestFixContext) (string, error)` method to `Renderer` and `PromptRenderer` interface. Update mock implementations.

**Acceptance Criteria:**
- Template renders with all context fields
- `RenderTestFix` method exists on `Renderer` and `PromptRenderer` interface
- Prompt test verifies template renders cleanly with sample data

**Dependencies:** None

### Task 4: Implement epilogue methods on Runner

**Files:**
- Create: `internal/runner/epilogue.go`
- Create: `internal/runner/epilogue_test.go`

**What to Do:**
Implement four methods on `*Runner`:

`runSessionEpilogue(ctx, st *runLoopState) (epilogueRanRetro bool, err error)`: Gate on `cfg.Session.Iterations > 0`. Log "Running session epilogue...". Call `runTestFixLoop()`, then conditionally `runEpilogueReview()` and `runEpilogueRetro()` based on config flags. Return whether retro ran (so caller can skip `checkRetroSuggestion`).

`runTestFixLoop(ctx) error`: Run `cfg.Session.TestCommand` via `r.runCmd()`. If exit code 0, log success, return. If non-zero, loop up to `cfg.Session.MaxFixRetries` times: load CLAUDE.md and RULES.md via renderer, build `TestFixContext`, render fix prompt, select provider via `r.router.Select("build", fixTier)`, call `p.StreamRun()`, re-run test command. If tests still fail after retries, create beads for residual failures via `r.beads.CreateWithParentAndDescription()`.

`runEpilogueReview(ctx) error`: Call `r.reviewer.RunThorough()` with appropriate parameters. Log results.

`runEpilogueRetro(ctx) error`: Select provider at high tier, wrap in adapter satisfying `retro.ProviderRunner`. Create `retro.NewRetroWithProvider(adapter, r.gromitDir)`. Call `retro.Run(ctx, nil)`. Call `retro.ParseProposals(result.Analysis)`. Call `apply.ApplyProposals(proposals, learningsFile, rulesPath)`. Record retro in state via `r.stateFile`.

**Acceptance Criteria:**
- Orchestration runs all three phases in order
- Phase skipping works via config flags
- Test-fix retries up to max, creates beads on exhaustion
- Review delegates to existing reviewer
- Retro runs analysis, applies proposals, records in state
- All phases log their progress

**Dependencies:**
- Task 1 (SessionConfig)
- Task 2 (ApplyProposals)
- Task 3 (test-fix prompt template)

### Task 5: Wire epilogue into finishRun and loop stopping

**Files:**
- Modify: `internal/runner/run_init.go`
- Modify: `internal/runner/runner.go`

**What to Do:**
In `finishRun()`, after the clean-exit save (step 3) and before `checkRetroSuggestion()` (step 4), call `runSessionEpilogue(ctx, st)`. If it returns `epilogueRanRetro == true`, skip `checkRetroSuggestion()`.

In `Run()` or `shouldStopLoop()`, when `cfg.Session.Iterations > 0`, use it as the effective max iterations for loop stopping (overriding `loop.max_iterations`). The simplest approach: compute `effectiveMaxIterations` at the start of `Run()` that picks `session.iterations` when non-zero, else `loop.max_iterations`.

**Acceptance Criteria:**
- `finishRun()` calls epilogue when `session.iterations > 0`
- Retro suggestion skipped when epilogue ran retro
- Bead loop stops at `session.iterations` when configured
- Behavior unchanged when `session.iterations` is 0 or absent

**Dependencies:**
- Task 4 (epilogue methods)

### Task 6: Verify all tests pass

**Files:**
- Test: `internal/runner/...`
- Test: `internal/retro/...`
- Test: `internal/config/...`

**What to Do:**
Run `go test ./internal/runner/... -count=1`, `go test ./internal/retro/... -count=1`, and `go test ./internal/config/... -count=1` to verify all new and existing tests pass. Fix any regressions.

**Acceptance Criteria:**
- `go test ./internal/runner/... -count=1` passes
- `go test ./internal/retro/... -count=1` passes
- `go test ./internal/config/... -count=1` passes

**Dependencies:**
- Task 5 (all wiring complete)

---

## Notes

- The `retro.ProviderRunner` adapter is a simple struct wrapping `provider.Select()` + `p.Run()`/`p.StreamRun()`. Define it in `epilogue.go` as a private type.
- The test-fix prompt should instruct the provider to fix implementation code only — not modify test expectations. This is the key behavioral constraint.
- For residual test failures (after max retries), create beads with priority P0 and a `from-epilogue` label so they get picked up in the next session.
- The epilogue runs after `setCleanExit(true)` — this is intentional. The session completed its build iterations successfully; epilogue failures shouldn't mark the session as unclean.
- When `session.test_command` is empty and `session.iterations > 0`, skip the test-fix phase (no command to run). The review and retro still run.
