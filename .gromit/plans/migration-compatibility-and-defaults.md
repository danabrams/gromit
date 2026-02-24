---
id: migration-compatibility-and-defaults
source_spec: migration-compatibility-and-defaults
created: 2026-02-24
decomposed: true
decomposed_at: "2026-02-24T11:05:00Z"
---

# Migration Compatibility and Defaults Implementation Plan

**Goal:** Introduce project profile/tracker/methodology adapter migration seams with backward-compatible defaults so existing repositories continue to behave exactly as they do today.

**Architecture:** Add a centralized compatibility resolver in config that computes effective profile/backend/adapter plus resolution source metadata, then wire runner/status/debug paths to use and display those resolved values while preserving legacy behavior.

**Tech Stack:** Go, Cobra CLI, internal config/runner/pipeline packages, existing contract + acceptance test harnesses.

**Spec:** `.gromit/specs/migration-compatibility-and-defaults.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Add a centralized “resolved compatibility context” in config/runtime that computes `profile`, `tracker backend`, and `methodology adapter` with explicit source metadata (`explicit` vs `profile_default` vs `legacy_fallback`), then thread it into runner/status/debug output without changing default behavior.

**Key Components:**
1. **Profile/Backend Resolution Layer (`internal/config`)**: Adds new config schema fields and computes resolved values in one place.
2. **Compatibility Shim Layer (`internal/runner` + adapters)**: Keeps current `bd` + Go methodology behavior as defaults when fields are absent.
3. **Diagnostics Surface (`internal/runner/format.go`, `print_status.go`, debug command paths)**: Displays resolved profile/backend/adapter plus resolution source.
4. **Migration/Parity Tests (`internal/config`, `internal/runner`, `cmd/gromit`, `test/contracts`)**: Covers old config compatibility and parity of adapter path vs legacy behavior.

**Integration Points:**
- Extend config schema with non-breaking optional sections (`project.profile`, tracker backend selector, methodology adapter selector).
- Keep existing constructors/call-sites, but switch internal resolution to read from the compatibility resolver.
- Add status/debug output lines for resolved values and default source.
- Preserve current behavior when new fields are absent.

**Data Flow:**
1. `config.Load()` parses old/new config.
2. Resolver computes effective `profile`, `tracker backend`, `methodology adapter` and source metadata.
3. Runner/CLI constructors consume resolved values (not raw fields).
4. Status/debug renders resolved values so users can verify migration behavior.

**Files to Modify:**
- `internal/config/config_types.go` - add optional compatibility schema fields.
- `internal/config/config.go` - normalization/validation for compatibility fields.
- `internal/config/config_defaults.go` and/or resolver file - compute effective compatibility values.
- `internal/config/config_accessors.go` - expose resolved getters and source metadata.
- `internal/runner/constructor.go` - consume resolved backend/adapter selections while preserving defaults.
- `internal/runner/format.go` - include compatibility diagnostics in status formatting.
- `internal/runner/print_status.go` - thread resolved diagnostics into status output.
- `cmd/gromit/debug.go` - include resolved compatibility diagnostics in debug/status-facing output.
- `internal/config/*_test.go`, `internal/runner/*_test.go`, `cmd/gromit/*_test.go`, `test/contracts/*` - migration and parity coverage.

**Files to Create:**
- `internal/config/compatibility_resolution.go` - canonical resolution logic.
- `internal/config/compatibility_resolution_test.go` - resolution precedence/source matrix coverage.
- `test/contracts/migration_compatibility_contract_test.go` (or equivalent targeted contract file) - old/new config compatibility checks.

**Tradeoffs:**
- **Central resolver over ad-hoc defaults**: prevents drift across init/runtime/status code paths.
- **Expose resolution-source metadata**: slightly larger surface area, but materially safer migration diagnostics.
- **No one-shot rewrite command**: lower migration risk and aligns with incremental rollout policy.

## Test Strategy

## Test Strategy

**Test Levels:**
1. **Unit Tests (`internal/config`)**: resolution precedence and backward compatibility (missing fields => Go/bd/go-adapter defaults; explicit overrides; source metadata correctness).
2. **Runner/CLI Integration Tests (`internal/runner`, `cmd/gromit`)**: adapter-based paths produce parity behavior for current default configuration; status/debug includes diagnostics.
3. **Contract Tests (`test/contracts`)**: old `gromit.yaml` fixtures continue to execute; new explicit config fixtures resolve backend/adapter as configured.

**Key Test Cases:**
- Old config without `project.profile` loads and runs unchanged.
- Default execution path (legacy config) and adapter path produce equivalent behavior for current defaults.
- Explicit `project.profile: go` + explicit backend/adapter resolve as explicit (not fallback).
- Debug/status output shows resolved profile/backend/adapter and source.
- Explicit invalid compatibility values fail validation with clear field-level errors.

**Mocking Strategy:**
- Mock adapter boundaries only where parity assertions need isolation.
- Keep config parsing and formatter paths real.
- Reuse existing bead/bd fakes in contract/integration tests.

**Coverage Goals:**
- Critical path: config load -> compatibility resolver -> runner wiring -> status/debug diagnostics.
- Migration safety: existing Go + bd acceptance behavior remains unchanged.
- Edge cases: partial config, mixed legacy/new fields, explicit empty/invalid values.

**Test Organization:**
- `internal/config/compatibility_resolution_test.go` for precedence/source matrices.
- `internal/runner/*_test.go` for parity and diagnostics propagation.
- `cmd/gromit/*_test.go` for command-surface output checks.
- `test/contracts/*` for old/new fixture compatibility scenarios.

## Implementation Tasks

### Task 1: Add Compatibility Schema and Resolved Context

**Files:**
- Modify: `internal/config/config_types.go`
- Modify: `internal/config/config.go`
- Create: `internal/config/compatibility_resolution.go`
- Test: `internal/config/compatibility_resolution_test.go`

**What to Do:**
Add non-breaking config fields for project profile and adapter/backend selectors, plus a resolved compatibility context type that records effective values and source metadata. Implement canonical resolution precedence (`explicit` > `profile_default` > `legacy_fallback`) with Go/bd/go-adapter compatibility defaults for missing legacy configs.

**Acceptance Criteria:**
- Config can parse old and new schema without breaking old files.
- Resolver returns effective profile/backend/adapter and source metadata deterministically.
- Missing new fields resolve to current Go-compatible behavior.

**Dependencies:**
- None.

**Notes:**
Keep normalization/validation strict only for explicitly configured values; do not reject missing fields.

### Task 2: Wire Runner to Resolved Compatibility Defaults

**Files:**
- Modify: `internal/config/config_accessors.go`
- Modify: `internal/runner/constructor.go`
- Modify: `internal/runner/methodology/*` (targeted wiring points)
- Test: `internal/runner/constructor_test.go`
- Test: `internal/runner/methodology/*_test.go`

**What to Do:**
Switch runner construction and methodology selection points to consume the resolved compatibility context rather than hard-coded assumptions. Ensure current default execution path remains behavior-equivalent (bd backend and Go methodology semantics) when new fields are absent.

**Acceptance Criteria:**
- Runner chooses tracker/methodology through resolved context.
- Existing default config behavior remains unchanged in runner tests.
- Adapter-based execution path is parity-equivalent under current defaults.

**Dependencies:**
- Task 1.

**Notes:**
Keep compatibility shims in place; do not remove legacy paths in this slice.

### Task 3: Add Status/Debug Compatibility Diagnostics

**Files:**
- Modify: `internal/runner/format.go`
- Modify: `internal/runner/print_status.go`
- Modify: `cmd/gromit/debug.go`
- Test: `internal/runner/status_test.go`
- Test: `cmd/gromit/debug_test.go`

**What to Do:**
Expose resolved profile/backend/adapter in status/debug output, including whether each value came from explicit config or defaults. Keep output lightweight and stable for existing consumers while adding migration visibility.

**Acceptance Criteria:**
- Status output shows resolved profile/backend/adapter.
- Diagnostics indicate source of defaults (explicit vs profile default vs legacy fallback).
- Existing status/debug behavior remains intact aside from additive diagnostics.

**Dependencies:**
- Task 1.
- Task 2 (for runtime wiring consistency).

**Notes:**
Prefer additive output lines to avoid destabilizing existing parsers/tests.

### Task 4: Add Migration and Parity Contract Coverage

**Files:**
- Modify/Create: `test/contracts/*` (targeted migration contract file)
- Modify: `cmd/gromit/*_test.go` (targeted compatibility command tests)
- Modify: `internal/config/config_test.go` (old/new config fixtures)

**What to Do:**
Add migration-focused tests that exercise old-config read compatibility, explicit new-config behavior, and parity between adapterized and legacy default execution paths.

**Acceptance Criteria:**
- Old config fixtures run without errors and preserve behavior.
- New explicit config fixtures resolve expected profile/backend/adapter.
- Contract tests prove parity for current default configuration.

**Dependencies:**
- Task 1.
- Task 2.
- Task 3.

**Notes:**
Reuse existing contract harness utilities to minimize scaffolding drift.

### Task 5: Guardrail Verification and Deprecation Markers

**Files:**
- Modify: `internal/config/*` (targeted deprecation annotations/messages)
- Modify: `internal/runner/*` (targeted TODO/deprecation markers)
- Test: targeted regression suites in affected packages

**What to Do:**
Add internal deprecation markers for legacy hard-coded assumptions that now route through compatibility resolution, while preserving shims. Run focused quality gates to confirm compatibility acceptance criteria and no regression in Go+bd baseline behavior.

**Acceptance Criteria:**
- Legacy assumptions are marked deprecated internally with compatibility shim retained.
- Targeted tests in config/runner/cmd/contracts pass for migration scenarios.
- No behavior regression for existing Go + bd workflows.

**Dependencies:**
- Tasks 1-4.

**Notes:**
This is the final guardrail pass before any future cleanup/removal work.

---

## Notes

- This plan intentionally avoids mandatory migration commands or automatic config rewrites.
- Compatibility visibility is a first-class deliverable: diagnostics are required to de-risk incremental rollout.
- Legacy cleanup/removal is explicitly deferred until at least one release cycle after parity is proven.
