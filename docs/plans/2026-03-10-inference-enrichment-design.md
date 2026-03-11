# Design: Spec 0001a — Optional Inference Enrichment for Project Memory

Date: 2026-03-10

## Overview

Add an optional inference enrichment phase to the Spec 0001 project-memory system. An LLM generates candidate inferred facts about a project while preserving the core guarantees: no repo pollution, multi-project isolation, structured external storage, provenance on all facts, and deterministic inspect as the canonical baseline.

Inferred content is not automatically promoted to declared truth and is not included in default context packets unless explicitly requested.

## Relationship to Inspect-Time Inference

The existing `inspect.DefaultInspector` calls `infer.StubInferrer`, which returns empty results. The stub inferrer remains the production behavior for inspect — inspect stays deterministic and produces only observed facts.

`enrich` is a separate, optional command that runs the real LLM-backed inference. It does not use the existing `infer.Inferrer` interface. Instead, enrichment introduces its own `CategoryEnricher` interface in `internal/next/enrich/`, purpose-built for category-scoped LLM passes.

This preserves the design principle: deterministic inspect is the canonical baseline. Enrichment is always opt-in and never modifies the inspect pipeline.

## Design Decisions

### 1. Enrichment scope and strategy

**Decision: Multiple focused passes by category.**

Each enrichment category gets its own LLM call with relevant observed facts, the file tree, and optionally sampled file contents. Passes can run in parallel.

This aligns with the existing extractor pattern (each extractor is focused), gives granular provenance ("this fact came from the architecture enrichment pass"), and supports selective re-runs of individual categories.

The enrichment input for each pass may include: project metadata, doctrine, architecture summary, source-map summary, validation summary, glossary, selected file/path summaries, and optionally selected code excerpts.

### 2. Staleness and re-enrichment

**Decision: Full idempotent re-derive with promoted fact immunity.**

Each enrichment run replaces `facts.json` wholesale. Promoted facts live in `doctrine/` as declared truth — immune to re-enrichment.

Previously accepted facts that re-appear (content hash match) retain their `accepted` status. Previously accepted facts that don't re-appear are marked `superseded`. Previously rejected facts are not carried forward — re-derive may re-propose them as `proposed`.

The guide renderer and context compiler exclude `superseded` facts. The `review-inferred` command shows `superseded` facts with their status for transparency.

Run history is preserved in `runs/` for provenance and auditability. The provenance tracker provides staleness warnings via git SHA comparison.

### 3. Fact identity and deduplication

**Decision: Content hash.**

`fact_id` = hash of (category + statement + key fields). This naturally deduplicates and allows status preservation across re-enrichment runs when the same inference is produced.

Since promoted facts copy their content into declared storage at promotion time (no back-reference), ID stability across runs is nice-to-have, not critical. Can upgrade to semantic key hashing later if LLM rewording causes friction.

### 4. Review UX

**Decision: List + selective accept/reject.**

`review-inferred` is a read-only listing showing facts with their IDs, categories, statements, and statuses. Accept and reject are separate explicit commands. No interactive mode, no file editing, no complex state tracking.

Promotion (converting accepted facts into declared truth) is deferred to a later spec. For now, `accepted` means "endorsed for future use" and `rejected` means "do not use."

### 5. Enrichment categories

**Decision: Fixed categories, hardcoded.**

Initial set:
- `component_boundary`
- `component_responsibility`
- `entrypoint`
- `risky_area`
- `integration_point`
- `glossary_term`
- `likely_validation_surface`
- `likely_ownership_boundary`

Each has a dedicated prompt and output schema. Adding a new category is a small, well-scoped PR. No plugin system (YAGNI).

### 6. Storage format

**Decision: Single `facts.json` + durable run artifacts.**

All inferred facts live in one `facts.json` with category fields, rather than per-category files. This keeps accept/reject state in one place and simplifies the review workflow. Promotion mechanics (including a proposals file) are deferred to a future spec.

### 7. CLI flags

**Decision: `--refresh` flag, no `--include existing-inspect`.**

Default behavior: enrichment reads existing observed facts from the cell and warns if stale (provenance SHA doesn't match HEAD). `--refresh` re-runs inspect synchronously to completion before starting enrichment passes. Users can also run `inspect && enrich` separately.

### 8. Preventing de facto truth

**Decision: Visual markers + staleness expiry.**

All inferred content in guides and packets is marked with `[INFERRED]` labels. Inferred facts older than 30 days (configurable) are excluded even when `--include-inferred` is passed. Two independent guardrails against silent promotion through habit.

## Fact Model

`InferredFact` is a new type in `internal/next/enrich/`, separate from the existing `fact.Fact` type in `internal/next/fact/`. The existing `fact.Category` (declared/observed/inferred) is the truth-tier; the enrichment `category` field uses `EnrichmentCategory` (component_boundary, entrypoint, etc.) to avoid collision.

Every inferred fact includes:

| Field | Description |
|-------|-------------|
| `fact_id` | Content hash of category + statement + key fields |
| `source_type` | Always `inferred` |
| `category` | One of the fixed enrichment categories |
| `statement` | The inferred fact as a clear statement |
| `rationale` | Why the LLM inferred this |
| `evidence_refs` | `[]string` — references to fact IDs, file paths, or artifact names |
| `confidence` | Enum string: `"high"`, `"medium"`, or `"low"` |
| `scope` | What part of the project this applies to |
| `status` | `proposed`, `accepted`, `rejected`, or `superseded` |
| `created_at` | Timestamp of the enrichment run |
| `inference_run_id` | ID of the enrichment run that produced this fact |

## Storage Layout

```
projects/<name>/
  artifacts/              # observed (unchanged)
  inferred/
    facts.json            # all inferred facts, normalized
    runs/
      <run-id>/
        request.json      # what was requested
        inputs.json       # what the LLM saw
        output.json       # raw LLM output
        summary.md        # human-readable summary
  doctrine/               # declared (unchanged)
  provenance/             # unchanged
  guide/                  # unchanged
```

## CLI Commands

```bash
gromit project enrich <project>                              # run enrichment
gromit project enrich <project> --refresh                    # re-inspect then enrich
gromit project enrich <project> --dry-run                    # preview without writing
gromit project review-inferred <project>                     # list facts with statuses
gromit project accept-inferred <project> --fact <id>         # mark fact as accepted
gromit project reject-inferred <project> --fact <id>         # mark fact as rejected
gromit project guide <project> --include-inferred            # guide with inferred sections
gromit context build <project> --level <level> --include-inferred  # packets with inferred facts
```

Promotion (`promote-inferred`) is deferred to a later spec.

## Guide Rendering

Default guide excludes inferred content. With `--include-inferred`, inferred facts appear in separate, clearly labeled sections:

- Inferred Component Structure `[INFERRED]`
- Inferred Likely Entrypoints `[INFERRED]`
- Inferred Risky Areas `[INFERRED]`
- Inferred Integration Points `[INFERRED]`
- Inferred Glossary `[INFERRED]`

Inferred sections include confidence markers where appropriate. Canonical sections are never modified or blurred.

## Context Compiler

Default packets exclude inferred facts. With `--include-inferred`:

- **Project packet**: may include inferred project-level observations
- **Spec packet**: may include inferred facts relevant to the spec scope only
- **Task packet**: may include inferred facts relevant to the task scope only

Rules:
1. Inferred facts are filtered by packet scope
2. Inferred facts carry provenance into the packet
3. Inferred facts are excluded from default packets
4. Unrelated inferred facts do not leak across scopes

Initial implementation includes all non-expired inferred facts at the project level regardless of scope. Scope-based filtering for spec and task packets is deferred — the `scope` field is stored but not yet used for filtering.

## Staleness and Expiry

- Inferred facts carry timestamps from their enrichment run
- Facts older than 30 days (configurable) are excluded even with `--include-inferred`
- Provenance tracker warns when observed facts are stale (git SHA mismatch)
- Stale warning does not block enrichment — it proceeds with a warning

## Scenarios

### Happy path

**Scenario 1 — First enrichment of a new project**

1. User runs `gromit project inspect payments-api`
2. User runs `gromit project enrich payments-api`
3. System runs 8 category passes in parallel, each seeing relevant observed facts
4. System writes `inferred/facts.json` with ~15-30 candidate facts, all status `proposed`
5. System writes run artifacts to `inferred/runs/<run-id>/`
6. User runs `gromit project review-inferred payments-api` — sees the list
7. User accepts 5 facts, rejects 2, leaves the rest as `proposed`
8. User runs `gromit project guide payments-api --include-inferred` — guide has separate `[INFERRED]` sections

**Scenario 2 — Re-enrichment after code changes**

1. User makes significant changes to payments-api
2. User runs `gromit project enrich payments-api --refresh`
3. System produces new `facts.json` — some facts match previous content hashes, some are new
4. Previously accepted facts that re-appear retain `accepted` status
5. Previously accepted facts that don't re-appear get marked `superseded`
6. Previously rejected facts may re-appear as `proposed`
7. New run artifacts are written; old run artifacts remain for provenance

**Scenario 3 — Scoped context packets with inference**

1. User runs `gromit context build payments-api --level task --include-inferred`
2. System compiles task-level packet with scoped inferred facts
3. Only inferred facts matching the task scope are included
4. Inferred facts carry provenance and `[INFERRED]` markers
5. Unrelated project-level inferred facts are excluded

### Error and edge cases

**Scenario 4 — Enrichment with stale observed facts**

1. User hasn't run inspect since 20 commits ago
2. User runs `gromit project enrich payments-api`
3. System warns: "Observed facts are stale (last inspect at abc123, HEAD is def456). Run with --refresh or run inspect first."
4. Enrichment proceeds with stale data (warning only, not blocking)

**Scenario 5 — Enrichment with no observed facts**

1. User runs `gromit project enrich payments-api` before ever running inspect
2. System errors: "No observed facts found. Run `gromit project inspect payments-api` first."
3. No inferred artifacts are written

**Scenario 6 — Expired inferred facts in guide**

1. User ran enrichment 45 days ago, never re-enriched
2. User runs `gromit project guide payments-api --include-inferred`
3. All inferred facts are past the 30-day expiry
4. Guide renders without inferred sections
5. System warns: "Inferred facts expired (last enrichment 45 days ago). Run `gromit project enrich` to refresh."

**Scenario 7 — LLM pass fails for one category**

1. User runs `gromit project enrich payments-api`
2. 7 of 8 category passes succeed; `likely_ownership_boundary` fails
3. System writes facts from the 7 successful passes
4. System reports: "Enrichment partially complete. Failed categories: likely_ownership_boundary."
5. Run artifacts capture the failure for debugging

**Scenario 8 — Accepted fact gets superseded**

1. User accepts fact `fact-abc` ("payments-api uses hexagonal architecture")
2. User restructures the codebase
3. User runs `gromit project enrich payments-api --refresh`
4. Re-enrichment doesn't produce a matching content hash
5. `fact-abc` is marked `superseded`
6. New fact `fact-def` ("payments-api uses vertical slice architecture") appears as `proposed`
7. `review-inferred` shows both the superseded fact and the new proposal

**Scenario 9 — Multi-project isolation**

1. User has `payments-api` and `auth-service` attached
2. User enriches both
3. `gromit project guide payments-api --include-inferred` shows only payments-api inferences
4. `gromit context build auth-service --level project --include-inferred` shows only auth-service inferences
5. No cross-contamination at any layer

**Scenario 10 — Dry run**

1. User runs `gromit project enrich payments-api --dry-run`
2. System runs all category passes but writes nothing
3. Candidate facts are output to stdout for review
4. No `facts.json`, no run artifacts created

## Acceptance Criteria

1. **Optional enrichment** — enrichment can run without affecting deterministic inspect
2. **Separation of truth layers** — inferred facts stored separately; no canonical artifact modified by enrichment
3. **Provenance** — every inferred fact includes provenance, rationale, confidence, and run identity
4. **Guide support** — guide optionally includes clearly marked inferred sections; default excludes them
5. **Context compiler support** — packets optionally include scope-filtered inferred facts; default excludes them
6. **Scope discipline** — inferred facts in spec/task packets are filtered to relevant scope
7. **Multi-project isolation** — inferred facts from one project never appear in another's outputs
8. **Reviewability** — human can inspect, accept, and reject inferred facts
9. **Zero repo pollution** — enrichment writes nothing to the target repo
10. **Staleness expiry** — inferred facts older than 30 days excluded even with `--include-inferred`
11. **Partial failure tolerance** — if one category enrichment pass fails, successful passes are still persisted and the failure is reported
12. **Dry run** — `--dry-run` flag runs all passes but writes nothing to the cell

## Evidence Required

- Example enrichment run for at least one fixture repo
- Proof that enrichment artifacts are stored in the external project cell
- Example guide rendered with and without inferred sections
- Example project/spec/task packet rendered with and without inferred content
- Proof that default packet compilation excludes inferred facts
- Proof that inferred facts are isolated between two attached projects
- Example review surface showing at least one accepted and one rejected inferred fact

## Relationship to Other Specs

**Depends on:** Spec 0001 — External Project Cell, Context Compiler, and Agent Guide

**Supports:** Spec 0002 — Minimal Spec Execution Loop, TaskLoop, and Evidence Bundle

## Open Questions

- Should accepted inferred facts be eligible for default packet inclusion before full promotion, or only after promotion?
- Should the guide show confidence numerically, qualitatively, or both?
- Should there be separate inference profiles for inspect, guide, and execution support?
