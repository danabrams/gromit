---
id: fix-llm-worktree-dir
spec: immutable-pipeline
created: 2026-03-07
decomposed: false
---

# Fix: Thread worktree directory to Claude CLI process

The Claude CLI launched by the build stage (and other LLM-invoking stages) inherits the process CWD instead of running in the spec worktree. This causes all code changes to land in the main repo, leaving the spec branch with zero unique commits.

## Research & Context

See investigation report: `.gromit/reports/debug-20260307-150000.md`

## Architecture

The fix threads a `Dir` field through three layers:

```
build.Stage.Run (has req.Worktree)
  → invokeWithEscalation (needs Dir param)
    → llm.StreamInvoke (LLMStreamInvokeRequest needs Dir field)
      → claude.Client.StreamRun (needs dir param → cmd.Dir)
```

Same pattern applies to `Invoke` (non-streaming) used by review, accept, plan, triage, decompose stages.

## Tasks

### Task 1: Add Dir field to LLM request types
**Files:** `internal/v2/llmtypes/types.go`
**Size:** ~4 lines

Add `Dir string` to both `LLMInvokeRequest` and `LLMStreamInvokeRequest`.

### Task 2: Thread Dir through claude.Client
**Files:** `internal/claude/claude.go`
**Size:** ~10 lines

- Add `dir string` parameter to `StreamRun` signature
- Set `cmd.Dir = dir` when dir is non-empty (line ~335)
- Update `runOnce` to accept and use a dir parameter similarly

### Task 3: Thread Dir through claudeAdapter
**Files:** `internal/v2/adapter/llm/claude.go`
**Size:** ~6 lines

- In `StreamInvoke`: pass `req.Dir` to `client.StreamRun`
- In `Invoke`/`runOnce`: pass `req.Dir` to set `cmd.Dir`

### Task 4: Pass worktree from build stage to LLM request
**Files:** `internal/v2/stage/build/build.go`
**Size:** ~8 lines

- Add `dir string` parameter to `invokeWithEscalation`
- Pass `req.Worktree` from `Stage.Run` to `invokeWithEscalation`
- Set `Dir: dir` in the `LLMStreamInvokeRequest`

### Task 5: Pass worktree from other LLM-invoking stages
**Files:** `internal/v2/stage/review/review.go`, `internal/v2/stage/accept/accept.go`, `internal/v2/stage/plan/plan.go`, `internal/v2/stage/triage/triage.go`, `internal/v2/stage/decompose/decompose.go`
**Size:** ~15 lines total

Each stage that calls `StreamInvoke` or `Invoke` needs to pass `req.Worktree` as `Dir` in the LLM request.

### Task 6: Update tests
**Files:** various `*_test.go`
**Size:** ~20 lines

- Update `claude.StreamRun` call sites in tests for new signature
- Add test verifying `cmd.Dir` is set when Dir is provided
- Update fake/mock LLM adapters if needed

## Dependencies

Task 1 → Tasks 2, 3 (type changes first)
Task 2 → Task 3 (claude.Client before adapter)
Tasks 1-3 → Tasks 4, 5 (plumbing before consumers)
Tasks 4, 5 → Task 6 (implementation before test updates)

## Testing Strategy

1. Unit test: verify `claude.StreamRun` sets `cmd.Dir` when dir is non-empty
2. Unit test: verify build stage passes `req.Worktree` as `Dir` in LLM request
3. Integration: run `go test ./...` to catch all broken call sites
4. Manual: run `gromit run2` on a spec and verify commits appear on the spec branch
