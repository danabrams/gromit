---
epic_id: cross-project-portability
created: 2026-02-20
---

# Cross-Project Portability

## Problem

Gromit is effective for self-hosting on this Go repository, but key parts of runtime policy and workflow are tightly coupled to this environment:
- Go-specific validation defaults and command assumptions
- bead/`bd`-specific work item semantics
- methodology command adaptation oriented around `go test`
- bootstrap templates and guidance tuned to Gromit internals

This coupling limits Gromit's ability to orchestrate work in other repositories and stacks.

## Vision

Make Gromit a reusable orchestration engine that can be initialized and run across different languages, build systems, and issue trackers without forking core logic.

The core stays responsible for loop orchestration, model routing, escalation, and artifact flow. Project-specific behavior is supplied through explicit profiles and adapters.

## Architecture Direction

1. **Project profiles** define stack-aware defaults (validation commands, rules guidance, methodology behavior, template set).
2. **Tracker adapters** decouple work-item operations from bead/`bd` internals.
3. **Methodology adapters** map ATDD/TDD verification to the active test runner ecosystem.
4. **Bootstrap profiles** let `gromit init` scaffold opinionated but stack-appropriate defaults.

## Phases

### Phase 1: Project Profile System

Introduce a first-class profile model (for example: `go`, `node`, `python`, `custom`) controlling:
- default validation command sets
- mandatory command policy
- rules/template seed content
- methodology defaults and constraints

Profiles must support local override without editing core code paths.

### Phase 2: Tracker Adapter Layer

Extract a tracker interface from bead-specific operations so run/refine/plan/review flows can target:
- `bd` (existing backend)
- future adapters (GitHub issues/projects, linear, file-backed queue, etc.)

Preserve existing `bd` behavior as the reference implementation.

### Phase 3: Methodology Runner Abstraction

Replace Go-specific ATDD/TDD command mutation with an adapter that can:
- inject tag/filter semantics per ecosystem
- validate pre-implementation failure checks in a toolchain-aware way
- keep current Go behavior unchanged through a Go adapter

### Phase 4: Profile-Aware Init and Docs

Update `gromit init` and generated artifacts to:
- prompt/select profile
- scaffold profile-matched templates and validation commands
- provide profile-specific next steps

### Phase 5: Migration and Compatibility Guardrails

Add migration-safe defaults and diagnostics so existing repositories continue to work unchanged while new abstractions roll out:
- backward-compatible config reads for missing profile/backend fields
- parity-focused contract coverage for adapterized paths
- debug/status visibility into resolved profile/backend/adapter

## Success Criteria

- A non-Go repository can run `gromit init` and get a valid first-run configuration without manual surgery.
- Core orchestration paths no longer assume bead-only semantics.
- Methodology verification works through profile adapters, not hard-coded `go test` transforms.
- Existing behavior in this repository remains backward-compatible.

## Candidate Specs

1. `project-profiles-core`
2. `tracker-adapter-interface`
3. `methodology-runner-adapter`
4. `profile-aware-init-bootstrap`
5. `migration-compatibility-and-defaults`

## Risks

- Over-generalization can dilute current reliability if abstractions are introduced without contract tests.
- Adapter boundaries may add complexity in critical run-loop code paths.
- Migration burden for existing users if defaults change abruptly.

## Mitigations

- Preserve current Go + `bd` path as default profile and default adapter.
- Add contract tests around tracker and methodology interfaces before switching call-sites.
- Roll out behind config flags and staged migration warnings.
