---
id: strengthen-quality-gates
source_ideas: []
created: 2026-03-02
accepted: true
---

# Strengthen Quality Gates

## Specification

Two new quality gate checks run after a bead's build phase succeeds and validation passes, targeting the two most common defect categories that currently leak to human QA: regressions (breaking previously-working behavior) and incomplete wiring (new code that isn't reachable from its intended entry point).

### Regression Gate

After a bead's validation passes, run the full project test suite — not just the tests related to the current bead's changes. The current validation stage only runs configured validation commands, which typically scope to changed packages. The regression gate broadens the blast radius to catch cases where a bead's changes break unrelated tests.

**Behavior:**
- After per-bead validation passes, run the project test suite for packages *not already covered* by the validation stage. The validation stage typically tests changed packages; the regression gate tests everything else.
- Runs concurrently with the review stage. Since review is non-blocking, the regression gate and review execute in parallel — no additional wall-clock time beyond whichever finishes last.
- If any test outside the bead's changed packages fails, the bead fails with a regression failure reason.
- The failure feeds back into the normal retry/escalation loop — the LLM gets the failing test output and is asked to fix the regression without losing the new behavior.
- Configurable: `quality_gates.regression.enabled` (default: true), `quality_gates.regression.command` (default: `go test ./...`).
- Skippable per-bead via a `skip-regression-check` label for beads that intentionally change cross-cutting behavior.

**Cost control:** The gate only tests packages not already validated by the per-bead validation stage, avoiding redundant test runs. It runs concurrently with review, so wall-clock cost is typically zero (hidden behind review latency). If the bead retries due to regression failure, subsequent validation runs include the full suite so the fix is verified.

### Wiring Gate

After a bead's build phase succeeds, verify that newly-exported symbols (functions, types, methods) introduced by the bead are actually referenced from existing code — not just defined in isolation. This catches the "feature exists but nobody calls it" problem.

**Behavior:**
- Diff the bead's changes and extract newly-added exported Go symbols (functions, types, struct fields, interface methods).
- For each new exported symbol, check whether it's referenced from at least one file outside the file where it's defined. References include: direct calls, type assertions, struct literal usage, interface satisfaction.
- Symbols that are only defined but never referenced are flagged as "unwired."
- Unwired symbols are fed back to the LLM with the instruction: "These new symbols are not called from anywhere. Either wire them into the appropriate call site or remove them if they're unnecessary."
- The check is structural (grep/AST-based), not LLM-based — zero token cost.
- Configurable: `quality_gates.wiring.enabled` (default: true).
- Exceptions: test helpers (`_test.go` exports), interface implementations (checked via interface satisfaction), and symbols with a `// wiring:deferred` comment are excluded.

**Scope:** This gate only checks symbols newly introduced by the current bead's diff. It does not audit the entire codebase for dead code.

### Ordering

The wiring gate runs after validation, before review. The regression gate runs concurrently with review:

```
Build → Validate → Wiring Gate → [Regression Gate + Review] (parallel) → Epilogue
```

If the wiring gate fails, the bead re-enters the retry loop before review starts. If the regression gate fails, the bead re-enters the retry loop after the parallel stage completes. Neither gate runs during retries caused by validation failures (only after validation passes).

## Acceptance Criteria

- After validation passes, the project test suite runs for packages not already covered by validation, concurrently with review. Any failures are reported as regression failures.
- Regression failures feed into the retry/escalation loop with the failing test output as context.
- After build succeeds, newly-exported symbols are checked for references from outside their defining file.
- Unreferenced exported symbols are reported as wiring failures with actionable feedback to the LLM.
- Both gates are enabled by default and independently configurable via `quality_gates.regression.enabled` and `quality_gates.wiring.enabled`.
- The `skip-regression-check` label bypasses the regression gate for a specific bead.
- Test helpers in `_test.go` files and symbols with `// wiring:deferred` comments are excluded from wiring checks.
- Existing beads that pass all current gates continue to pass (no false positives on already-wired, non-regressing code).

## Decisions

1. **Two separate gates, not one.** Regressions and incomplete wiring are different failure modes with different detection mechanisms (test execution vs. structural analysis). Keeping them separate allows independent configuration and clearer failure messages.

2. **Structural wiring check, not LLM-based.** Checking whether a symbol is referenced is a deterministic, zero-cost operation (grep or AST walk). Using an LLM would add cost and non-determinism for a problem that has a precise answer.

3. **Incremental suite minus already-validated packages.** The validation stage already tests changed packages. The regression gate tests everything else, avoiding redundant work. This is cheaper than running the full suite from scratch while still covering the entire project.

4. **Gates fail into the existing retry loop.** No new orchestration — a gate failure is treated like a validation failure. The LLM gets the failure context and retries normally. This keeps the system simple and leverages the existing escalation chain.

5. **Regression gate runs concurrently with review.** Review is non-blocking and already runs after validation. Running the regression gate in parallel with review means the regression check adds zero wall-clock time in the common case (review typically takes longer than test execution). This was chosen over sequential execution to avoid slowing down the loop.

6. **No test result snapshotting.** Comparing "tests that passed before" vs "tests that pass now" would require snapshotting test state, which adds complexity. Simply running the remaining suite and failing on any failure is simpler and catches the same issues.

## Research & Context

### Current State

**Validation stage** (`internal/runner/validation/runner.go`): Runs configured validation commands sequentially. Commands are typically scoped to changed packages (e.g., `go test ./internal/runner/...`). Does not run the full project suite.

**Review stage** (`internal/pipeline/review/review.go`): LLM-based code review across 6 dimensions. Non-blocking — always returns "Proceed." Creates follow-up beads for significant issues but doesn't prevent the current bead from closing.

**Orchestrator stages** (`internal/runner/orchestrator.go`): Gate → Build → Validate → LocalGate → Review → Epilogue. The regression and wiring gates would insert between Validate and LocalGate (or replace LocalGate's position if LocalGate is not enabled).

### Why These Defects Leak

**Regressions leak** because validation commands are scoped to changed packages. A bead that modifies `internal/config/` might break a consumer in `internal/runner/`, but if validation only tests `internal/config/...`, the runner breakage is invisible until human QA.

**Wiring leaks** because neither validation nor review checks reachability. The LLM can build a complete feature in a new file, pass all tests (because the tests test the new code in isolation), pass review (because the code quality is fine), and close the bead — but the feature is never called from `main()` or any command handler. Human QA discovers the feature doesn't actually work end-to-end.
