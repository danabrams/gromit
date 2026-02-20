---
id: migration-compatibility-and-defaults
source_ideas:
  - idea-1771591754833
  - idea-1771591754839
  - idea-1771591754844
  - idea-1771591754849
created: 2026-02-20
---

# Migration Compatibility and Defaults

## Specification

Define a backward-compatible migration strategy for introducing project profiles, tracker adapters, and methodology adapters without breaking existing Gromit installations.

This spec covers rollout controls, config upgrade behavior, compatibility shims, and regression-test expectations. It is the guardrail spec that makes the portability epic safe to land incrementally.

### Compatibility Policy

- Existing repositories with no `project.profile` continue as Go-compatible behavior.
- Existing bead/`bd` workflows remain default backend.
- Existing methodology behavior for Go remains unchanged.
- New abstractions are opt-in by configuration and/or enabled-by-default only when behavior is equivalent.

### Migration Mechanics

1. Add schema fields and defaults with non-breaking reads.
2. Introduce adapters behind existing concrete implementations.
3. Switch internal call-sites to adapters while keeping behavior parity.
4. Add warnings/diagnostics for deprecated assumptions only after parity is verified.

No mandatory one-shot migration command is required in this phase.

### Diagnostics and Observability

Add lightweight diagnostics in debug/status output:
- resolved project profile
- resolved tracker backend
- resolved methodology adapter
- source of resolved defaults (explicit vs profile default)

This enables safe verification during incremental rollout.

### Deprecation Plan

Mark legacy hard-coded assumptions as deprecated internally once adapter parity is proven, but keep compatibility shims for at least one full release cycle after migration.

## Acceptance Criteria

- Repositories without new config fields continue to run without errors.
- Existing Go + `bd` behavior passes current acceptance and integration tests.
- Adapter-based execution paths produce behavior-equivalent results for current default configuration.
- Debug output can show resolved profile/backend/adapter and default source.
- Migration tests cover old-config read compatibility and new-config explicit behavior.

## Execution Order

- Sequence position: 5 (final guardrail pass)
- Dependencies: `project-profiles-core`, `tracker-adapter-interface`, `methodology-runner-adapter`, `profile-aware-init-bootstrap`
- Outcome: locks in compatibility expectations before legacy cleanup begins

## Decisions

1. **Compatibility before cleanup** -- avoid removing legacy paths until parity is established and tested.

2. **Incremental flip, not big bang** -- each abstraction lands with behavior parity checks before next slice.

3. **Visibility over hidden magic** -- expose resolved configuration decisions to reduce migration uncertainty.

## Research & Context

### Risk Areas

- Subtle behavior drift in priority ordering, ready-item semantics, and label filtering during tracker abstraction.
- Methodology regressions if command adaptation changes under default Go profile.
- Init-generated defaults diverging from runtime fallback behavior.

### Files to Change

| File | Change |
|------|--------|
| `internal/config/*` | Backward-compatible defaulting and field resolution |
| `internal/runner/*` | Adapter-based path with compatibility parity tests |
| `cmd/gromit/status.go` / debug paths | Add resolved profile/backend/adapter diagnostics |
| `test/contracts/*` | Add migration/parity contract coverage |
| `cmd/gromit/*_test.go` | Old-config/new-config compatibility coverage |

### Out of Scope

- Automated rewriting of existing `gromit.yaml` files.
- Removing legacy code paths in the same release slice.
