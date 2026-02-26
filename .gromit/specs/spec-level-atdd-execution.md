---
id: spec-level-atdd-execution
source_ideas: []
created: 2026-02-15
updates: [atdd-simplification, run-scope-flags]
epic: spec-first-atdd-execution
---

# Spec-Level ATDD With Scoped Execution

## Specification

Shift ATDD responsibility from individual beads to the spec lifecycle. `gromit run` should execute beads within an explicit scope (`--spec` or `--epic`) instead of the global queue by default in methodology-driven workflows. Acceptance tests are authored and reviewed once per spec, then validated at a spec gate after bead execution.

This keeps beads implementation-focused while preserving ATDD's role as behavioral specification.

## Workflow

1. Resolve execution scope (single spec or epic set of specs).
2. Before bead execution for a spec, generate/update acceptance tests for the full spec behavior contract.
3. Run scoped bead execution without per-bead ATDD phases.
4. Run spec acceptance gate.
5. If gate fails, synthesize fix beads from failing scenarios/gaps and continue within the same spec scope.
6. Mark spec complete when scoped beads are done and spec gate passes.

## Acceptance Criteria

- `gromit run` supports `--spec` and `--epic` for scoped bead selection using existing label filter infrastructure.
- A config toggle exists for ATDD granularity: `bead` (current) vs `spec` (new).
- In `spec` granularity mode, per-bead ATDD phases (acceptance write/verify/refactor coupling) are skipped.
- In `spec` granularity mode, acceptance tests are generated/reviewed once per spec and persisted before implementation beads run.
- A spec-level acceptance verification step runs after scoped bead batches and produces structured failures.
- On spec-level acceptance failure, Gromit can create targeted follow-up beads tied to the same `spec:<name>` label.
- Existing non-scoped `gromit run` behavior remains available for backward compatibility.

## Decisions

1. ATDD remains mandatory for behavior-critical work, but the boundary moves from bead to spec.
2. Scoped execution is preferred for methodology-enabled runs to reduce cross-spec context switching.
3. Refactor remains a TDD concern; ATDD focuses on behavior contracts.
4. Spec acceptance failures should produce artifacts (new/fix beads), not silent retries.

## Rollout

### Phase 1: Scope-first execution
Wire `run-scope-flags` in `gromit run` and document scoped mode as recommended for methodology runs.

### Phase 2: Granularity flag
Add methodology granularity config and disable per-bead ATDD phases when set to `spec`.

### Phase 3: Spec acceptance gate
Add pre-batch acceptance authoring and post-batch gate verification with structured failure output.

### Phase 4: Failure-to-bead synthesis
Generate fix beads from gate failures and rerun scoped loop until gate passes or retry policy is exhausted.

## Success Metrics

- Reduce ATDD-related bead failures per 100 iterations.
- Reduce median iterations-to-close for methodology-enabled specs.
- Reduce total Claude invocations per completed spec.
- Maintain or improve post-merge defect rate for scoped specs.

## Related Specs

- `atdd-simplification`
- `run-scope-flags`
- `epic-scoped-execution`
