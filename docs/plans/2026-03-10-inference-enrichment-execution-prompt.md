# Spec 0001a — Inference Enrichment Execution Prompt

Execute the Spec 0001a implementation plan at `docs/plans/2026-03-10-inference-enrichment-plan.md` using the `superpowers:executing-plans` skill.

## Key Documents

- **Design doc:** `docs/plans/2026-03-10-inference-enrichment-design.md`
- **Implementation plan:** `docs/plans/2026-03-10-inference-enrichment-plan.md`
- **Verification plan:** `docs/plans/2026-03-10-inference-enrichment-verification-plan.md`

## Existing Code

The following Spec 0001 packages are already built and should be reused:

- `internal/next/fact/` — Fact type with Declared/Observed/Inferred categories
- `internal/next/infer/` — Inferrer interface (currently stubbed)
- `internal/next/artifact/` — JSON artifact storage
- `internal/next/projectcell/` — Project cell CRUD
- `internal/next/workspace/` — Workspace resolution
- `internal/next/extract/` — Deterministic extractors (file tree, go.mod, validation commands)
- `internal/next/inspect/` — Inspection orchestration
- `internal/next/guide/` — Agent guide markdown renderer
- `internal/next/contextpkt/` — Context packet compiler
- `internal/next/provenance/` — Provenance tracking
- `internal/next/architecture/` — Architecture types (modules, dependencies, components)
- `internal/next/doctrine/` — Doctrine rules storage
- `internal/next/sourcemap/` — Source map types
- `internal/next/validation/` — Validation command types
- `internal/provider/` — Provider interface, Claude/Codex/Gemini implementations

The enricher consumes these as pre-serialized JSON via EnrichInput, not by importing the packages directly.

All new enrichment code goes in `internal/next/enrich/`. Guide and context compiler modifications go in their existing packages.

## Execution Rules

- The plan has 17 tasks across 6 phases. Follow TDD strictly: red test, green implementation, commit.
- **Use subagents to parallelize independent tasks.** The dependency graph in the implementation plan shows which tasks can run concurrently. Specifically:
  - Tasks 1 and 2 are fully independent — run in parallel.
  - After Task 1: Tasks 3, 4, 5, 8, 9 can all run in parallel.
  - After Task 5: Task 6 can start.
  - After Tasks 3, 4, 6: Task 7 can start. After Tasks 3, 4: Task 10 can start.
  - Tasks 12 and 13 only need Task 3 and can run in parallel with each other. Task 14 needs Tasks 8, 9. Task 11 needs Tasks 2, 7, 10.
  - Tasks 15, 16, 17 are sequential — each depends on Task 15 completing first.
  - Use `superpowers:dispatching-parallel-agents` for parallel task batches.
- Use the verification plan to confirm each phase works before moving to the next.
- Commit after each task per the plan.
- Do not push until everything passes.

## Provider Integration

The LLM enricher must:
- Use the existing `provider.Provider` interface directly (no router)
- Support at least Claude and Codex providers
- Accept configurable model name and reasoning level
- Capture `CostUSD`, `InputTokens`, and `OutputTokens` from `provider.Result`
- Store per-category and aggregate cost data in run artifacts
- Use `provider.TierFromLegacyModel(model)` to convert config model names to tier strings before calling `Provider.Run()`. The `reasoning` field is stored for provenance but is not a parameter to `Provider.Run()`.

## Key Design Constraints

- Inferred facts never written to `artifacts/` — only to `inferred/`
- Default guide and context packets exclude inferred content
- `[INFERRED]` markers on all inferred content when included
- Content-hash fact IDs for deduplication
- Full re-derive on each enrichment run; accepted statuses preserved by hash match
- 30-day staleness expiry (configurable via per-project `enrichment.json`)
- Partial failure tolerance — if one category pass fails, successful passes are persisted and the failure is reported
- Zero writes to the target repo

## Final Verification

After all 17 tasks are complete, run the full verification checklist:

```bash
go test ./internal/next/... -v
go test -race ./internal/next/...
go vet ./internal/next/... && go vet ./cmd/gromit-next/...
go mod tidy && git diff --exit-code go.mod go.sum
go build ./cmd/gromit-next/
```

Then run the enrichment integration tests specifically:

```bash
go test ./internal/next/enrich/ -run TestIntegration -v
```

Only push after all checks pass.
