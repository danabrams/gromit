---
epic_id: spec-first-atdd-execution
created: 2026-02-15
---

# Spec-First ATDD Execution

## Problem Space

Per-bead ATDD ceremony is increasing execution cost and failure churn. Bead-level acceptance test generation and repeated verification are causing repeated retries, while the real behavior contract usually lives at the spec level.

## Goal

Move ATDD up to the spec boundary and make scoped execution (`--spec` / `--epic`) the normal way to run methodology-heavy work.

## Why This Epic Exists

- Align ATDD with behavior contracts (spec outcomes), not implementation fragments (beads).
- Reduce invocation overhead and failure loops during `gromit run`.
- Improve focus by finishing one spec (or epic slice) before jumping across unrelated queue items.

## Linked Specs

- Existing: `atdd-simplification`
- Existing: `run-scope-flags`
- New: `spec-level-atdd-execution`

## Outcomes

1. Scoped run adoption increases for active implementation sessions.
2. ATDD-related failures shift from bead-level noise to actionable spec-level gate failures.
3. Failed spec gates produce explicit follow-up beads instead of silent drift.

## Open Questions

- Should scoped mode become the default when methodology is enabled, or remain opt-in?
- Should spec-level acceptance tests live in per-spec files/directories for stable ownership?
- What retry budget should exist before human intervention is required on spec gate failures?
