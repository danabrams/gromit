---
id: tskip-audit
title: Audit and fix all t.Skip() calls to comply with RULES.md
priority: 1
epic: test-quality
---

# t.Skip() Audit

## Problem

RULES.md states: "Do not write tests with `t.Skip()` for scenarios that can't run in the test environment. Every committed test must be runnable. If a test needs external dependencies, use `//go:build acceptance` so it runs in the right context."

There are 170 `t.Skip`/`t.Skipf` calls across 35 files. Many violate this rule — dead placeholder tests, always-skip tautologies, disabled tests from a past migration, and tests that should use build tags instead.

## Triage

### A. KEEP as-is (33 skips, 10 files) — Legitimate runtime guards

These are proper skip patterns guarding against genuinely unavailable resources at test time:

| File | Skips | Guard |
|------|-------|-------|
| `cmd/gromit/install_skill_test.go` | 4 | `testing.Short()` |
| `internal/provider/codex_test.go:794` | 1 | `testing.Short()` |
| `cmd/gromit/refine_test.go:140` | 1 | `os.Geteuid() == 0` |
| `internal/logger/globalstats_test.go:341` | 1 | `os.Getuid() == 0` |
| `internal/runner/globalstats_integration_test.go:210` | 1 | `os.Getuid() == 0` |
| `internal/tmux/tmux_test.go:77,81` | 2 | `!InTmux()` / socket check |
| `internal/runner/runner_test.go:496,514` | 2 | git not available |
| `internal/learnings/learnings_test.go:127,171` | 2 | similarity threshold guard |
| `internal/config/timeout_documentation_acceptance_test.go:90,116` | 2 | config field not present |
| `internal/provider/codex_test.go:961` | 1 | `exec.LookPath("bash")` |
| `internal/runner/runner_provider_wiring_acceptance_test.go:45` | 1 | test precondition |

**Action:** None. These are correct usage.

### B. DELETE — Dead/never-run tests (~56 skips, 9 files)

**B1. `cmd/gromit/explore_pipeline_adapter_test.go` (28 skips)**
Delete entire file. Every single test immediately calls `t.Skip()`. These were scaffolding for a pipeline refactor that was completed differently.

**B2. `internal/provider/provider_test.go` (7 skips)**
Delete 7 tests that use tautological pattern: `var p Provider; if p == nil { t.Skip(...) }`. A nil interface is always nil — these can never execute their test body.
- `TestProviderRunMethod`, `TestProviderStreamRunMethod`, `TestProviderRunValidationMethod`, `TestProviderIsUsageLimitErrorMethod` (standalone)
- Plus 3 subtests in `TestProviderRunWithCustomTier` and `TestProviderRunValidationWithCustomTier` and `TestProviderIsUsageLimitErrorTableDriven`

**B3. `internal/runner/integration_test.go` (13 skips)**
Delete 11 tests disabled with "needs update for router-based provider calls - covered by acceptance tests" and 2 disabled with "TODO: Update test to use Router pattern":
- `TestIntegration_EscalationChainFullFlow`, `TestIntegration_ValidationFailureKeepsBeadOpen`, `TestIntegration_DecompositionOnExhaustedEscalation`, `TestIntegration_RecoverableRetryThenSuccess`, `TestIntegration_StopOnFailure`, `TestIntegration_MixedResultsMultiBead`, `TestIntegration_ScopeTooLargeDetection`, `TestIntegration_FullFlowBuildValidateCloseNext`, `TestIntegration_UnclearSpecStopsProcessing`, `TestIntegration_BetweenIterationsCommand`
- `TestIntegration_MultipleEscalationsWithRetries`, `TestIntegration_DryRunMultipleBeads`, `TestIntegration_LabelOverrideModelSelection`

**B4. `internal/runner/globalstats_integration_test.go:355` (1 skip)**
Delete `TestRun_MergesWithExistingGlobalStats` — same "needs update for router-based provider calls" reason.

**B5. Scattered single dead tests (7 skips, 6 files)**
- `internal/claude/claude_test.go`: Delete `TestRunWithTimeout`, `TestStreamRunTimeout`, `TestMultipleContextLayers` (3 empty tests that skip "for CI compatibility")
- `cmd/gromit/explore_test.go:69`: Delete `TestExploreWorkflow_DelegatesPipelineExplore` ("full workflow tested in pipeline package")
- `internal/pipeline/decompose_test.go:682`: Delete `TestDecomposeWorkflow_RespectsContextCancellation` ("not yet implemented")
- `internal/logger/usage_limit_field_test.go:204`: Delete `TestWriteIterationLog_PropagatesUsageLimited` (cross-package placeholder)
- `cmd/gromit/chain_test.go:157`: Delete `TestExecGromit` ("requires integration testing")
- `internal/runner/runner_claude_field_removal_test.go:94`: Delete `TestNewRunnerWithDepsWorksWithOnlyRouter` ("documents desired behavior")

### C. CONVERT to `//go:build acceptance` (29 skips, 2 files)

**C1. `internal/bead/label_integration_test.go` (15 skips)**
No build tag. All tests require `bd` binary via `newIsolatedClient()` or `BD_AVAILABLE` env var. Add `//go:build acceptance` at top.

**C2. `cmd/gromit/epic_test.go` (14 skips)**
No build tag. Tests run `gromit` binary and skip when `bd` or external deps unavailable. Add `//go:build acceptance` at top.

Note: `internal/bead/bead_test.go` also has `newIsolatedClient()` which calls `t.Skipf` when `bd init` fails (1 skip). The affected tests in this file should also get a build tag, but this needs careful inspection since the file likely contains both unit tests (no bd) and integration tests (needs bd). Split if necessary.

### D. CONVERT `t.Skipf` to `t.Fatalf` — Source/template file reads (15 skips, 6 files)

These skip when reading source files or template files that should always exist. A missing source file is a test infrastructure failure, not a skip condition.

**D1. Source code structural tests (9 skips, 3 files)**
- `cmd/gromit/review_agent_test.go`: 5 `t.Skipf("Cannot read ...")` + 2 `t.Skip("Cannot find func...")`
- `cmd/gromit/plan_agent_test.go`: 2 `t.Skipf("Cannot read plan.go...")`
- `cmd/gromit/refine_agent_test.go`: 1 `t.Skipf("Cannot read pipeline/refine.go...")`

**D2. Template/rules file guards (6 skips, 4 files)**
- `internal/prompt/prompt_test.go:738`: `t.Skipf` when decompose template not found
- `internal/prompt/build_template_validation_section_test.go:18`: `t.Skipf` when build template not found
- `internal/prompt/decompose_guidelines_acceptance_test.go`: 4 `t.Skipf` when template not found (already has `//go:build acceptance`)
- `internal/rules/rules_test.go:666`: `t.Skipf` when RULES.md not found

### E. KEEP with note — Acceptance test setup failures (~46 skips, 5 files)

These already have `//go:build acceptance` and use `t.Skipf` for bead creation failures during test setup. Ideally these would be `t.Fatalf` (setup failure = broken infrastructure, not a skip), but they're behind build tags and not violating the letter of the rule.

| File | Skips |
|------|-------|
| `internal/bead/list_with_label_acceptance_test.go` | 12 |
| `internal/bead/list_all_statuses_acceptance_test.go` | 7 |
| `internal/bead/list_with_label_test.go` | 7 |
| `cmd/gromit/run_scope_flags_acceptance_test.go` | 11 |
| `cmd/gromit/epic_bead_counts_acceptance_test.go` | 9 |

**Action:** Optional low-priority cleanup. Convert `t.Skipf` to `t.Fatalf` in test setup sections.

## Scope

Categories B, C, D are mandatory. Category E is optional.

Estimated impact:
- **~56 dead tests deleted** (Category B)
- **~29 skips fixed via build tags** (Category C)
- **~15 skips converted to fatalf** (Category D)
- Net result: ~100 skip violations eliminated

## Risks

- Deleting `integration_test.go` tests (B3) assumes acceptance tests truly cover these scenarios. Verify by checking that each deleted test's scenario has a counterpart in the acceptance test suite.
- Adding `//go:build acceptance` to `epic_test.go` (C2) means those tests won't run in normal `go test`. Ensure they run in CI's acceptance stage.
- `bead_test.go` (C note) may need splitting if it contains both unit and integration tests in the same file.
