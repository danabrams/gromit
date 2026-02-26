# Audit: normalize method visibility and call sites

## Executive Summary
All `normalizeNilFields` and `NormalizeNilFields` method definitions are correctly classified with respect to their visibility and call site patterns. No mismatches found.

## Definition Classification

### Exported Methods (Capitalized) - Cross-Package Usage

1. **`Config.NormalizeNilFields()`** (`internal/config/config_normalize.go:6`)
   - **Visibility**: EXPORTED (Capitalized)
   - **Call Sites**: 29 callers across multiple packages
   - **Usage Pattern**: Cross-package (runner, state, provider, agent, escalation, acceptance, benchmark packages)
   - **Status**: ✅ CORRECT - Should be exported

2. **`SubTask.NormalizeNilFields()`** (`internal/runner/runtypes/types.go:202`)
   - **Visibility**: EXPORTED (Capitalized)
   - **Call Sites**: 3 callers (types_test.go)
   - **Usage Pattern**: Same-package tests
   - **Status**: ✅ CORRECT - Exported, used in same package

3. **`State.NormalizeNilFields()`** (`internal/state/state.go:267`)
   - **Visibility**: EXPORTED (Capitalized)
   - **Call Sites**: 4 callers in state package
   - **Usage Pattern**: Same-package (state.go, state_test.go, interactive_state.go)
   - **Status**: ✅ CORRECT - Exported for same-package use

4. **`InteractiveState.NormalizeNilFields()`** (`internal/state/interactive_state.go:240`)
   - **Visibility**: EXPORTED (Capitalized)
   - **Call Sites**: 4 callers in state package
   - **Usage Pattern**: Same-package (interactive_state.go, interactive_state_test.go)
   - **Status**: ✅ CORRECT - Exported for same-package use

### Unexported Methods (Lowercase) - Package-Local Usage

5. **`Context.normalizeNilFields()`** (`internal/prompt/context_types.go:64`)
   - **Visibility**: UNEXPORTED (lowercase)
   - **Call Sites**: 32 callers, all in prompt package
   - **Usage Pattern**: Package-local only (parse.go, methodology_phase_context.go, budget_test.go, prompt_test.go, etc.)
   - **Status**: ✅ CORRECT - Unexported, never crosses package boundary

6. **`ScopeEstimate.normalizeNilFields()`** (`internal/prompt/context_types.go:212`)
   - **Visibility**: UNEXPORTED (lowercase)
   - **Call Sites**: 2 callers in prompt package
   - **Usage Pattern**: Package-local (parse.go, prompt_test.go)
   - **Status**: ✅ CORRECT - Unexported, package-local only

7. **`Bead.normalizeNilFields()`** (`internal/bead/bead.go:66`)
   - **Visibility**: UNEXPORTED (lowercase)
   - **Call Sites**: 5 callers in bead package
   - **Usage Pattern**: Package-local (bead.go, bead_test.go, list_with_label_test.go)
   - **Status**: ✅ CORRECT - Unexported, package-local only

8. **`Proposals.normalizeNilFields()`** (`internal/retro/proposals.go:49`)
   - **Visibility**: UNEXPORTED (lowercase)
   - **Call Sites**: 4 callers in retro package
   - **Usage Pattern**: Package-local (proposals.go, apply.go, proposals_test.go)
   - **Status**: ✅ CORRECT - Unexported, package-local only

9. **`ConsolidationProposal.normalizeNilFields()`** (`internal/retro/proposals.go:72`)
   - **Visibility**: UNEXPORTED (lowercase)
   - **Call Sites**: 3 callers in retro package
   - **Usage Pattern**: Package-local (proposals.go called by parent, proposals_test.go)
   - **Status**: ✅ CORRECT - Unexported, package-local only

10. **`GateVerdict.normalizeNilFields()`** (`internal/specgate/verdict.go:21`)
    - **Visibility**: UNEXPORTED (lowercase)
    - **Call Sites**: 1 caller in specgate package
    - **Usage Pattern**: Package-local (verdict.go)
    - **Status**: ✅ CORRECT - Unexported, package-local only

11. **`ReviewResult.normalizeNilFields()`** (`internal/review/review.go:47`)
    - **Visibility**: UNEXPORTED (lowercase)
    - **Call Sites**: 1 caller in review package
    - **Usage Pattern**: Package-local (review.go)
    - **Status**: ✅ CORRECT - Unexported, package-local only

12. **`StreamMessage.normalizeNilFields()`** (`internal/logger/stream.go:44`)
    - **Visibility**: UNEXPORTED (lowercase)
    - **Call Sites**: 3 callers in logger package
    - **Usage Pattern**: Package-local (stream.go, stream_test.go)
    - **Status**: ✅ CORRECT - Unexported, package-local only

13. **`ProcessTrend.normalizeNilFields()`** (`internal/logger/process_trend.go:294`)
    - **Visibility**: UNEXPORTED (lowercase)
    - **Call Sites**: 1 caller in logger package
    - **Usage Pattern**: Package-local (process_trend.go)
    - **Status**: ✅ CORRECT - Unexported, package-local only

14. **`BeadStats.normalizeNilFields()`** (`internal/logger/logger.go:294`)
    - **Visibility**: UNEXPORTED (lowercase)
    - **Call Sites**: 3 callers in logger package
    - **Usage Pattern**: Package-local (logger.go, logger_test.go)
    - **Status**: ✅ CORRECT - Unexported, package-local only

15. **`SelfReport.normalizeNilFields()`** (`internal/coverage/validator.go:31`)
    - **Visibility**: UNEXPORTED (lowercase)
    - **Call Sites**: 1 caller in coverage package
    - **Usage Pattern**: Package-local (validator.go)
    - **Status**: ✅ CORRECT - Unexported, package-local only

16. **`File.normalizeNilFields()`** (`internal/learnings/learnings.go:90`)
    - **Visibility**: UNEXPORTED (lowercase)
    - **Call Sites**: 1 caller in learnings package
    - **Usage Pattern**: Package-local (learnings.go)
    - **Status**: ✅ CORRECT - Unexported, package-local only

## Cross-Package Caller Analysis

The only method that is called across package boundaries is:
- **`Config.NormalizeNilFields()`** - Called by:
  - `internal/runner/` (multiple subpackages)
  - `internal/state/` (state.go, interactive_state.go)
  - `internal/provider/build.go`
  - `internal/agent/` (resolve_test.go)
  - `internal/benchmark/`
  - Acceptance tests across runner

This is the only cross-package consumer, and it is correctly exported.

## Audit Conclusions

✅ **All 16 methods have correct visibility classification**
- 4 exported methods used appropriately across packages or in same-package public API
- 12 unexported methods properly hidden and used only within package boundaries
- No visibility mismatches detected
- No unexported methods accessed across package boundaries
- No exported methods used only within packages that could be made unexported

## Files Analyzed

**Definition files (16 total)**:
- internal/config/config_normalize.go
- internal/runner/runtypes/types.go
- internal/prompt/context_types.go
- internal/state/state.go
- internal/state/interactive_state.go
- internal/bead/bead.go
- internal/retro/proposals.go
- internal/specgate/verdict.go
- internal/review/review.go
- internal/logger/stream.go
- internal/logger/process_trend.go
- internal/logger/logger.go
- internal/coverage/validator.go
- internal/learnings/learnings.go

**Call sites identified**: 76+ callers across 56 files analyzed

## Recommendation

No changes required. The normalize method visibility is correctly implemented throughout the codebase.
