# Spec 0002b — Continue from Phase 10 (or finishing)

## Instructions for Next Agent

All implementation phases (1-9) are complete. Task 45 is deferred per the plan. Use `superpowers:finishing-a-development-branch` to integrate the work, or continue with any remaining work if the plan has more phases.

## Key Files

- **Plan:** `docs/plans/2026-03-12-spec-0002b-execution-plan.md`
- **Design:** `docs/plans/2026-03-11-spec-0002b-review-acceptance-design.md`
- **Testing plan:** `docs/plans/2026-03-12-spec-0002b-testing-plan.md`
- **Execution prompt:** `docs/plans/2026-03-12-spec-0002b-execution-prompt.md`

## Worktree

- **Path:** `/Users/dabrams/gromit/.worktrees/spec-0002b`
- **Branch:** `feature/spec-0002b-review-acceptance`
- You are already IN the worktree. Do not create a new one.

## What's Done — Phase 1 (5 commits) + Phase 2 (12 commits) + Phase 3 (8 commits) + Phase 4 (7 commits) + Phase 5 (10 commits) + Phase 6 (5 commits) + Phase 7 (5 commits) + Phase 8 (14 commits) + Phase 9 (4 commits)

### Phase 1 (5 commits — bug fixes and test fakes)

1. `332fe867b` — test(next): confirm exec list returns exit 0 on empty results (Bug 3)
2. `9b28b0004` — fix(next): spec list resolves project.json from workspace root (Bug 2)
3. `a8421169b` — fix(next): replace stub stage provider with real provider (Bug 1)
4. `37ffe3173` — test(next): add shared test fakes in internal/next/testutil/
5. `e8b0afc06` — fix(next): wire RealStageProvider as default and remove defaultStageProvider stub

### Phase 2 (12 commits — `internal/next/review/` package)

6. `24d99cb88` — feat(next): add review severity type with ordering and parsing
7. `8b1dcb31c` — feat(next): add review Finding and FindingSet types with JSON support
8. `8ab048b9f` — feat(next): add review threshold logic for blocking determination
9. `0d8ca2eeb` — feat(next): add review facet registry with 5 built-in facets
10. `ae0dbb2d5` — feat(next): add review prompt template rendering
11. `17d39041c` — feat(next): add new-vs-preexisting finding matching logic
12. `bc7f83d56` — feat(next): add DiffProvider interface in review package
13. `d0d934024` — feat(next): add review runner with per-facet orchestration and fix-cycle support
14. `0a3606fd7` — feat(next): configurable facet retry on invalid LLM output with ParseError detection
15. `43a7b0515` — fix(next): add NormalizeNilFields to RunResult, fix nil slice returns, add all-facets-errored test
16. `5d324dc8e` — fix(next): guard empty descriptions in matching, use errors.As for ParseError detection

### Phase 3 (8 commits — `internal/next/acceptor/` package + Bundler methods)

17. `be392a40a` — feat(next): add acceptor types for criterion evaluation results
18. `37e33afff` — feat(next): add acceptance evaluator with per-criterion LLM evaluation
19. `cd75aa9c1` — feat(next): add acceptance prompt rendering
20. `91d24e126` — feat(next): add acceptance failure context for planner replanning
21. `e70c0e598` — feat(next): add WriteReviewFindings to evidence Bundler
22. `ad1fa0a23` — feat(next): add WriteAcceptanceResults to evidence Bundler
23. `8d04a993b` — fix(next): use RenderAcceptancePrompt in evaluator, add Cycle field and NormalizeNilFields to AcceptanceFailure
24. `33e4ee84e` — style(next): fix gofmt formatting in acceptor package

### Phase 4 (7 commits — ReviewStage, AcceptStage, pipeline insertion)

25. `779c3f7a1` — feat(next): add ReviewStage with blocking/continue logic
26. `690a75a6e` — feat(next): add ParseAcceptanceCriteria markdown parser
27. `b0fd0dd05` — feat(next): add AcceptStage with fail/unclear replanning
28. `184516767` — feat(next): insert ReviewStage and AcceptStage into execution pipeline
29. `7d0a4b231` — test(next): verify dry-run filter excludes review and accept stages
30. `fc1b0b177` — fix(next): add evidence writes to ReviewStage/AcceptStage, fix dead test code, add edge case tests
31. `1bb309086` — fix(next): fix no-op assertion in DiffProvider error test, align evidence write guards

### Phase 5 (10 commits — fix-cycle extensions, failure context, cycle reset)

32. `ac9fcef64` — feat(next): add review failure context builder for planner replanning
33. `9a965341c` — feat(next): add review/acceptance fields to RunState
34. `7664d0f19` — fix(next): NormalizeNilFields handles ReviewFindings and AcceptanceResults slices
35. `6ea0c0398` — feat(next): ReviewStage stores findings as strings in RunState
36. `8ec834b7c` — feat(next): AcceptStage stores results and flags in RunState
37. `ab2d8b0e0` — feat(next): ReviewStage passes prior findings to runner on fix cycles
38. `88f007d1f` — feat(next): reset gate booleans and review/acceptance fields at cycle start
39. `949b63488` — test(next): add SpecLoop stage-reuse invariant comment and regression guard
40. `7b7bd4bd6` — fix(next): move containsAny to test file, snapshot values in cycle reset test
41. `cf2b49e43` — style(next): fix gofmt formatting in failctx.go

### Phase 6 (5 commits — execution policy extensions)

42. `e99f0da30` — feat(next): add review config section to execution policy
43. `7df115527` — feat(next): add evaluator model tier to execution policy
44. `8ca281929` — feat(next): add review facet and threshold validation to policy
45. `56e8dfd2e` — test(next): verify review config loads from JSON with defaults
46. `a403eacbe` — fix(next): integrate ValidateReviewConfig into Validate() to catch invalid thresholds

### Phase 7 (5 commits — evidence bundle extensions)

47. `9e63a9dfc` — feat(next): extend review.md with review findings and acceptance criteria sections
48. `bfa6c956a` — feat(next): EvidenceStage reads review.json and acceptance.json from disk for review.md
49. `8f8fbce72` — fix(next): aggregate review findings per facet with severity counts, sort facets deterministically
50. `8834b7ace` — fix(next): sort severity keys for deterministic output, add malformed JSON fallback test
51. `54e692c02` — style(next): fix gofmt formatting in bundle.go and evidence_test.go

### Phase 8 (14 commits — integration wiring, events, integration tests)

52. `9325db03f` — feat(next): parallel facet failure continues with successful facets, marks failed ones as errored
53. `99b613b31` — feat(next): DiffProvider interface for ReviewStage diff computation
54. `7462c8067` — feat(next): all-facets-error returns Blocked from ReviewStage
55. `cb3f82a53` — feat(next): add review and acceptance event types for event log
56. `db11d2bd9` — feat(next): add Source field to ReplanTriggeredEvent for replan origin tracking
57. `73a5c1e64` — feat(next): ReviewStage and AcceptStage emit event log entries
58. `2d3750d44` — test(next): integration test verifying review blocks ready_for_review
59. `6fbc5f466` — test(next): integration tests verifying acceptance fail triggers fix cycle and budget exhaustion
60. `2c377121b` — test(next): integration test verifying configurable threshold behavior
61. `9460d7ea7` — test(next): integration test verifying facet enablement via config
62. `5425cd83a` — test(next): integration test verifying new-vs-preexisting matching in fix cycles
63. `5042efdd0` — fix(next): include errored facets in review event, strengthen event emission test assertions
64. `7e216faa9` — fix(next): check context cancellation in review runner retry loop
65. `6425277e7` — style(next): fix gofmt formatting in severity.go and types.go

### Phase 8 details:

- **runner.go:** Added `AllFacetsErrored bool` field to `RunResult`. Set after processing facets when all facets errored and zero findings produced. Added context cancellation check in retry loop.
- **diff.go:** Added `GitDiffProvider` production implementation that runs `git diff baseBranch...HEAD`.
- **events.go:** Added `ReviewResultEvent` (TotalFindings, BlockingFindings, FacetsReviewed) and `AcceptanceResultEvent` (TotalCriteria, PassCount, FailCount, UnclearCount) structs. Added `Source string` field to `ReplanTriggeredEvent`. Added unmarshalEvent cases for `review_result` and `acceptance_result`.
- **specloop.go:** Captures `replanSource = stage.Name()` and passes it as `Source` when emitting `ReplanTriggeredEvent`.
- **review.go (stages):** AllFacetsErrored handling returns `Blocked` with sorted error messages. Event emission includes both successful and errored facets.
- **accept.go (stages):** Event emission counts pass/fail/unclear criteria.
- **Integration tests:** 9 tests covering review blocking, acceptance replan/budget exhaustion, threshold config, facet enablement, and fix-cycle matching.

### Review findings addressed (Phase 8):
- Event emission now includes errored facets in FacetsReviewed list
- Event emission tests assert event type
- Fire-and-forget pattern documented with comments (consistent with SpecLoop.emitEvent)
- Context cancellation check added to review runner retry loop
- Pre-existing gofmt issues in severity.go and types.go fixed

**Test status (after Phase 8):** 420 internal/next tests + 33 cmd/gromit-next tests passing. `go vet` clean, `gofmt` clean, build clean.

### Phase 9 (4 commits — FinalizeStage gates, blocked worktree lifecycle)

66. `2be82974b` — feat(next): FinalizeStage requires review+acceptance gates for ready_for_review
67. `8a7c2b843` — feat(next): add blocked_worktree_cleaned event and unmarshalEvent switch cases
68. `3226e218c` — feat(next): InitStage cleans up prior blocked worktrees on new run
69. `7bd082636` — fix(next): capture worktree path before clearing in cleanup event, remove duplicate test, add event assertions

### Phase 9 details:

- **finalize.go:** Updated `ready_for_review` condition to require `allDone && FinalValidationPassed && FinalReviewPassed && FinalAcceptancePassed`. Removed worktree removal for blocked runs (worktrees now preserved; cleanup happens in InitStage).
- **events.go:** Added `BlockedWorktreeCleanedEvent` struct with `PriorRunID` and `WorktreePath`. Added `unmarshalEvent` switch case for `"blocked_worktree_cleaned"`.
- **init.go:** Added `cleanBlockedWorktrees()` method that lists runs for the project, filters for blocked runs with same spec ID, removes worktree directories, clears path in store, and emits `blocked_worktree_cleaned` events.
- **Review fix:** Captured worktree path in local variable before clearing to ensure event logs the actual path. Removed duplicate test. Added event content assertions.

**Test status (after Phase 9):** 429 internal/next tests + 33 cmd/gromit-next tests = 462 total passing. `go vet` clean, `gofmt` clean, build clean.

### Phase 10 (3 commits — review fixes and test coverage)

70. `527499a01` — fix(next): add CLAUDE.md nil-field normalization visibility comments to all new NormalizeNilFields methods
71. `ec65addd1` — fix(next): address review findings — stale diff, case sensitivity, event observability
72. `8edfa3fdf` — test(next): add DiffProvider coverage for AcceptStage

### Phase 10 details:

- **CLAUDE comments:** Added required `// See CLAUDE.md nil-field normalization visibility convention:` comments to 10 files in internal/next/ to fix TestNormalizeNilFieldsVisibilityPolicy.
- **AcceptStage stale diff fix:** Replaced static `DiffSummary string` with `DiffProvider review.DiffProvider` + `BaseBranch string` so diffs are computed fresh on each fix cycle (matching ReviewStage pattern).
- **ParseAcceptanceCriteria:** Changed from case-insensitive (`EqualFold`) to exact case match per design spec.
- **ReviewResultEvent:** Added `ErroredFacets []string` field for observability (design required errored facets in event log).
- **Test coverage:** Added `TestAcceptStage_ComputesDiffFromDiffProvider` and `TestAcceptStage_DiffProviderError` tests.

**3 review passes completed:**
- Pass 1: Found 2 major + 7 minor issues. Fixed 3 (stale diff, case sensitivity, event observability).
- Pass 2: Found 1 major (missing DiffProvider tests). Fixed it.
- Pass 3: Clean — all remaining items are design choices, not bugs.

**Test status (after Phase 10):** 7921 total tests passing (431 internal/next + rest of project). `go vet` clean, `gofmt` clean, build clean.

## What's Next — Ready to finish branch

All implementation phases (1-9) + review phase (10) are complete. Task 45 (per-task artifact files) is deferred per the plan.

Use `superpowers:finishing-a-development-branch` to integrate the work.

## Code Conventions

- Module: `github.com/danabrams/gromit`
- Nil-field normalization: exported `NormalizeNilFields()` for cross-package types
- Stage interface: `Name() string`, `Run(ctx, *RunState) (NextAction, error)`
- Noop fakes for LLM deps live in `cmd/gromit-next/stage_provider.go`
- Shared test fakes live in `internal/next/testutil/`

## Deferred Acceptance Criteria

Acceptance criteria 6-13 from the spec (`spec-level-review-and-targeted-remediation.md`) are intentionally deferred to the next phase (agent wiring / bead integration). This branch built the review, acceptance, and pipeline foundations; the deferred criteria require bead creation, CLI flag wiring, and agent-level orchestration that belong in a separate branch.

- **AC 6:** When accept or review fails, their findings become the input to remediation decompose — requires agent wiring to pass FailureContext into remediation runner
- **AC 7:** Remediation decompose creates targeted fix beads from findings, not from the original plan — requires bead creation integration and findings-based decompose prompt
- **AC 8:** The gate satisfaction check closes open beads whose acceptance criteria are already satisfied — requires bead store integration with gate logic
- **AC 9:** When review passes with findings, spec-scoped findings become from-review beads labeled with the spec — requires bead creation from Finding type (TODO added for Scope field extensibility)
- **AC 10:** When review passes with findings, general findings become from-review beads without a spec label — requires bead creation from Finding type (TODO added for Scope field extensibility)
- **AC 11:** `run2 --from-review` runs only beads with the from-review label through the bead loop — requires CLI flag addition and bead query logic
- **AC 12:** `run2 --from-review --spec <id>` scopes to from-review beads for a specific spec — requires CLI flag addition and filtered bead query
- **AC 13:** From-review beads do not trigger spec-level accept/review cycles — requires bead loop mode selection in agent orchestration

**Foundations in place:**
- FailureContext threading (criteria 6-7): `ReviewFailureContext` and `AcceptanceFailure` types are built and tested; planner replanning receives structured failure context
- Finding type extensibility (criteria 9-10): `Finding` struct supports severity, category, description, and affected files; TODO added for `Scope` field to distinguish spec-scoped vs general findings

## Deferred items for later phases

- Real GitDiffProvider implementation wired into stage_provider.go (Task 33b added the type, but stage_provider still uses noopDiffProvider)
- AcceptStage DiffProvider wired into stage_provider.go (currently uses nil DiffProvider in production wiring)
