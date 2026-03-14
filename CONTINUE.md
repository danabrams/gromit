# Spec 0002d — Continue

## Status
- **Current phase:** Phase 2 — COMPLETE
- **Next phase:** Phase 3 — Integration Test Scaffolds (Task 5)
- **Date:** 2026-03-13

## Completed Phases
- Phase 1: FallbackAdapter — COMPLETE (42 tests passing, 6 new)
- Phase 2: Router Wiring + Codex Contract Tests — COMPLETE (684 tests passing: 50 cmd + 634 internal)

## Phase 2 Summary
- Files created: none
- Files modified: `internal/next/execpolicy/policy.go`, `internal/next/execpolicy/policy_test.go`, `cmd/gromit-next/stage_provider.go`, `cmd/gromit-next/stage_provider_test.go`, `cmd/gromit-next/exec.go`, `internal/next/llmadapter/contract_helper.go`, `internal/next/planner/agent_contract_test.go`, `internal/next/review/agent_contract_test.go`, `internal/next/acceptor/agent_contract_test.go`, `internal/next/specloop/taskrunner_contract_test.go`, `internal/next/specloop/pipeline_integration_test.go`
- Tests added: 8 (2 routing validation, 1 provider fields, 2 buildRouter, 1 FallbackAdapter wiring, 4 Codex contract tests in separate files)
- Review rounds: 2, issues fixed: 4 (Important: eliminated ~40 lines of duplicated legacy path via constructor promotion; Important: fixed default preferences "validate"→"accept"; Minor: added model staleness comment; Minor: gofmt fixes)
- Final checkpoint: PASS

### What was done:
- **Task 3a:** Added `RoutingConfig` struct to `execpolicy.Policy` with `Preferences`, `Ratio`, `CooldownSeconds` fields, defaults, ratio-sum-to-100 validation, and `NormalizeNilFields`
- **Task 3b:** Added `ClaudeProvider`, `CodexProvider`, `StateFn`, `CircuitBreaker` fields to `RealStageProviderConfig` and `RealStageProvider`
- **Task 3c:** Implemented `buildRouter` method on `RealStageProvider` that constructs `provider.Router` from policy routing config
- **Task 3d:** Replaced hardcoded `p.cfg.Provider` with `p.claudeProvider` + `FallbackAdapter` wrapping Router in `BuildStages`. Constructor auto-promotes legacy `Provider` to `claudeProvider` for backward compatibility
- **Task 4:** Added `ContractCodexInvoker` helper and Codex contract tests in all 4 domain packages (planner, review, acceptor, specloop), gated by `GROMIT_LLM_CONTRACT=1`

## Next Phase Instructions
1. Read this file
2. Read the execution prompt: docs/plans/2026-03-13-spec-0002d-execution-prompt.md
3. Read the implementation plan: docs/plans/2026-03-13-spec-0002d-implementation-plan.md
4. Skip to "Phase 3" section — Integration Test Scaffolds (Task 5)
5. Implement Phase 3 following the Phase → Review Loop → CONTINUE.md workflow

## Worktree
- **Path:** `/Users/dabrams/gromit/.worktrees/spec-0002d`
- **Branch:** `feature/spec-0002d`
- You are already IN the worktree. Do not create a new one.

## Verification
```
=== Phase 2 Checkpoint ===
go test ./internal/next/execpolicy/ -v -count=1 -> 32 PASS
go test ./cmd/gromit-next/ -v -count=1 -> 50 PASS
go test ./internal/next/... -count=1 -> 634 passed in 28 packages
go vet ./internal/next/... ./cmd/gromit-next/... -> clean
gofmt -l internal/next/ cmd/gromit-next/ -> clean
```
