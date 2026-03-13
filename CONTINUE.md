# Spec 0002c — Continue

## Status
- **Current phase:** Phase 4 — COMPLETE
- **Next phase:** Phase 5 — Contract test framework (sequential, single agent)
- **Date:** 2026-03-13

## Completed Phases
- Phase 1: LLMAdapter package — COMPLETE (12 tests passing)
- Phase 2: Invoker interfaces + ExtractJSON — COMPLETE (28 tests passing)
- Phase 3: Per-domain adapters — COMPLETE (281 tests across 6 packages)
- Phase 4: RealStageProvider wiring — COMPLETE (638 tests total)

## Phase 4 Summary
- Files modified: `cmd/gromit-next/stage_provider.go`, `cmd/gromit-next/stage_provider_test.go`
- Tests added: 2 (WithProvider_ReturnsRealAdapters, NilProviderFallsBackToNoops)
- Review rounds: 2 (round 1 found 1 important + 2 suggestions, fixed; round 2 clean)
- Final checkpoint: PASS

### What was done:
- Added `Provider provider.Provider` field to `RealStageProviderConfig`
- When Provider is non-nil, wires real LLM-backed adapters:
  - Plan: ProviderPlanAgent → Planner (with FixPlanCreator)
  - Execute: ProviderTaskRunner (OnCost nil to avoid double-counting)
  - Validate: ShellValidator wrapping real Runner
  - Review: ProviderReviewAgent → Runner (with threshold, facets, tiers)
  - Accept: ProviderAcceptAgent → Evaluator
  - Diff: GitDiffProvider for review, accept, and evidence stages
- When Provider is nil, falls back to noop implementations (backward compat)
- Compile stage remains noop (TODO for 0002d — needs ArtifactStore)
- Cost callbacks wired to budget.AddCost for plan, review, accept adapters

### Review fixes applied:
- Reused parsed threshold from early validation instead of re-parsing
- Wired diffProv into evidence stage (was incorrectly using noopDiffProvider)

## Next Phase Instructions
1. Read this file
2. Read the execution prompt: docs/plans/2026-03-13-spec-0002c-execution-prompt.md
3. Read the implementation plan: docs/plans/2026-03-13-spec-0002c-implementation-plan.md
4. Skip to "Phase 5" section — Contract test framework
5. Implement Phase 5 following the Phase -> Review Loop -> CONTINUE.md workflow

## Worktree
- **Path:** `/Users/dabrams/gromit/.worktrees/spec-0002c`
- **Branch:** `feature/spec-0002c`
- You are already IN the worktree. Do not create a new one.

## Verification
```
=== Phase 4 Checkpoint ===
go test ./cmd/gromit-next/ -v -count=1 -> 44 passed
go build ./cmd/gromit-next/ -> clean
go test ./internal/next/... -count=1 -> 594 passed in 28 packages
go vet ./internal/next/... ./cmd/gromit-next/ -> clean
gofmt -l internal/next/ cmd/gromit-next/ -> clean
```
