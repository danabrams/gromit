---
id: precheck-verification-config-consistency
source_ideas: []
created: 2026-02-28
accepted: true
---

# Precheck Verification Config Consistency

## Specification

`gromit.yaml` currently allows this combination:

- `precheck.enabled: false`
- `precheck.verification.enabled: true`

This creates ambiguous operator intent because verification is a child of precheck and has no effect when precheck is off. The config surface should make that relationship explicit and deterministic.

This spec defines consistent behavior for precheck verification settings when precheck is disabled:

1. Precheck remains the parent gate.
2. Verification is only effective when precheck is enabled.
3. Disabled-precheck + enabled-verification is treated as a hard configuration conflict and fails startup/config load with a clear error.

## Acceptance Criteria

- The effective runtime behavior cannot run precheck verification when `precheck.enabled` is false.
- A config containing `precheck.enabled: false` and `precheck.verification.enabled: true` is detected as a conflict.
- Conflict handling is strict: config load/startup fails.
- User-facing output explains the conflict using full field paths (`precheck.enabled`, `precheck.verification.enabled`) and the resulting behavior.
- Defaults and inline config comments do not imply verification can run independently of precheck.
- Config tests cover:
  - default values,
  - explicit enabled/disabled combinations,
  - the conflict case and its expected handling.

## Decisions

1. **Parent-child semantic contract**: `precheck.verification` is subordinate to `precheck`; it is not an independent feature toggle.

2. **Conflict must be explicit**: silently accepting contradictory toggles is not acceptable; users need deterministic and observable handling.

3. **Strict by design**: contradictory parent/child toggles fail fast to prevent ambiguous runtime expectations.

## Research & Context

### Current State

- `gromit.yaml` in this repo sets `precheck.enabled: false` and `precheck.verification.enabled: true`.
- Config defaults currently set:
  - precheck default disabled (`false`)
  - precheck verification default enabled (`true`)
- Accessors encode those defaults independently, which permits contradictory effective configuration.

Relevant files:

- `gromit.yaml`
- `internal/config/config_types.go`
- `internal/config/config_defaults.go`
- `internal/config/config_accessors.go`
- `internal/config/config_test.go`

### Why This Matters

The current shape increases cognitive load and can mislead operators into thinking verification is active when precheck is globally disabled. Making this deterministic reduces misconfiguration risk and improves trust in run-time behavior.
