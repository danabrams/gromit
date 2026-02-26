---
created: 2026-02-26T00:00:00Z
decomposed: true
decomposed_at: "2026-02-26T09:18:03Z"
id: explore-command-delegation-test
source_spec: explore-command-delegation-test
---

# Explore Command Delegation Unit Test Implementation Plan

**Goal:** Add focused command-level coverage proving `explore` delegates once to `pipeline.Explore(ctx, input)` with correctly mapped CLI fields and reports returned artifact paths.

**Architecture:** Introduce a narrow CLI test seam for the explore runner so tests can inject a mock and assert argument mapping and output behavior without filesystem or subprocess dependencies.

**Tech Stack:** Go, Cobra CLI, existing `internal/pipeline` types, table-free unit testing with lightweight mocks.

**Spec:** `.gromit/specs/explore-command-delegation-test.md`

---

## Architecture

**Overview:**
Add a small test seam in `explore` command wiring so a unit test can inject a mock pipeline and assert `runExplore` delegates correctly with expected `ExploreInput` fields.

**Key Components:**
1. **`exploreRunner` interface (CLI-local):** Minimal interface exposing `Explore(ctx, input)` for mockability.
2. **Factory seam for command wiring:** Package-level function variable to build/inject the explore runner in tests.
3. **Delegation unit test:** Mock runner captures `ExploreInput`, returns a fixed result, and asserts output includes the artifact path/name.

**Integration Points:**
- `runExplore` continues to own CLI arg/flag parsing and output handling.
- `buildExplorePipeline` remains the production constructor.
- `runExploreInSession` adapts to accept the runner seam instead of requiring only `*pipeline.Pipeline`.

**Data Flow:**
`explore` CLI args/flags (`topic`, `--agent`, `--choose-agent`, `--model`) are parsed into `pipeline.ExploreInput`, delegated to one `Explore(ctx, input)` call on the injected runner, then rendered through `handleExploreOutput` using the returned result.

**Files to Modify:**
- `cmd/gromit/explore.go` - add runner seam and route `runExplore` through it.
- `cmd/gromit/explore_test.go` - add delegation test with input/output assertions.

**Files to Create:**
- None.

**Tradeoffs:**
- Prefered a narrow interface seam over full command integration execution for deterministic, dependency-free coverage.
- Limited changes to CLI wiring to reduce risk to existing explore behavior.

## Test Strategy

**Test Levels:**
1. **Unit Tests:** Add one command-level unit test for `runExplore` delegation behavior.
2. **Integration Tests:** Keep existing `runExploreInSession` tests as-is; no new integration cases required.
3. **Manual Testing:** Not required for this scope.

**Key Test Cases:**
- `runExplore` calls `Explore(ctx, input)` exactly once.
- Captured input fields match CLI-provided values for `Topic`, `AgentName`, `ChooseAgent`, and `Model`.
- Command output contains the created artifact path/name returned by the mock result.
- Test executes without filesystem state or subprocess execution.

**Mocking Strategy:**
- Mock only the explore runner boundary (`Explore(ctx, input)`).
- Keep `handleExploreOutput` real so assertions verify actual command reporting behavior.
- Stub command-level seams (`loadConfig`, runner factory/session path) only as needed to keep test hermetic.

**Coverage Goals:**
- Protect CLI-to-pipeline wiring from regressions.
- Ensure single invocation semantics to catch accidental double-run regressions.

**Test Organization:**
- Add `TestExploreCommand_DelegatesToPipeline` in `cmd/gromit/explore_test.go`.
- Follow existing package patterns for temporary seam overrides and restoration via `t.Cleanup`.

## Implementation Tasks

### Task 1: Add Explore Runner Injection Seam

**Files:**
- Modify: `cmd/gromit/explore.go`

**What to Do:**
Introduce a minimal interface and package-level seam that allows tests to inject a mock explore runner while preserving current production behavior based on `buildExplorePipeline` and session execution.

**Acceptance Criteria:**
- `runExplore` can obtain its explore runner via an overridable seam.
- Production flow still uses the real pipeline when no test override is present.
- Existing explore tests continue to compile against the updated function signatures.

**Dependencies:**
- None.

**Notes:**
Keep the seam narrow and local to avoid broad refactors across commands.

### Task 2: Add Command Delegation Unit Test

**Files:**
- Modify: `cmd/gromit/explore_test.go`

**What to Do:**
Add `TestExploreCommand_DelegatesToPipeline` with a mock runner that captures `ExploreInput`, returns a fixed `ExploreResult`, and verifies both delegation input correctness and output content.

**Acceptance Criteria:**
- Test asserts mock `Explore` is called exactly once.
- Test asserts captured `ExploreInput` fields match provided CLI flags/args.
- Test asserts command output includes artifact path/name returned by the mock result.

**Dependencies:**
- Task 1.

**Notes:**
Ensure the test is hermetic and does not require real `.gromit` directories, prompt rendering, or agent execution.

### Task 3: Validate Command Package Test Suite

**Files:**
- Test: `cmd/gromit/...`

**What to Do:**
Run command package tests to verify the new delegation coverage passes and no existing explore/refine/decompose command tests regress.

**Acceptance Criteria:**
- `go test ./cmd/gromit/...` passes.
- New test runs without filesystem/subprocess dependencies.

**Dependencies:**
- Task 2.

**Notes:**
If unrelated flakes occur, capture them as follow-up work rather than broadening this change scope.

---

## Notes

This plan intentionally focuses on command delegation wiring coverage and does not alter pipeline explore business logic. The resulting test should be fast, deterministic, and suitable as a regression guard for future CLI flag handling changes.
