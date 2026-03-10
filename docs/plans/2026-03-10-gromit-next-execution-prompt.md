# Gromit Next Spec 0001 — Execution Prompt

Execute the Gromit Next Spec 0001 implementation plan at `docs/plans/2026-03-10-gromit-next-implementation-plan.md` using the `superpowers:executing-plans` skill.

## Key Documents

- **Design doc:** `docs/plans/2026-03-10-gromit-next-project-cell-design.md`
- **Implementation plan:** `docs/plans/2026-03-10-gromit-next-implementation-plan.md`
- **Verification plan:** `docs/plans/2026-03-10-gromit-next-verification-plan.md`
- **Agent guide:** `internal/next/AGENTS.md`

## Execution Rules

- The plan has 20 tasks across 6 phases. Follow TDD strictly: red test, green implementation, commit.
- **Use subagents to parallelize independent tasks.** The dependency graph in the implementation plan shows which tasks can run concurrently. Specifically:
  - Tasks 1 and 3 are fully independent — run in parallel.
  - After Task 1: Tasks 2, 5, 6, 7, 10, 12, 13, 14 can be parallelized.
  - After Task 3: Task 4 can start.
  - Tasks 8 and 9 can parallelize after Task 7.
  - Use `superpowers:dispatching-parallel-agents` for parallel task batches.
- Use the verification plan to confirm each phase works before moving to the next.
- Commit after each task per the plan.
- Do not push until everything passes.

## Final Verification

After all 20 tasks are complete, run the full verification checklist from the verification plan:

```bash
go test ./internal/next/... -v
go test -race ./internal/next/...
go vet ./internal/next/... && go vet ./cmd/gromit-next/...
go mod tidy && git diff --exit-code go.mod go.sum
go build ./cmd/gromit-next/
```

Then run the integration test specifically:

```bash
go test ./internal/next/ -run TestIntegration -v
```

Only push after all checks pass.
