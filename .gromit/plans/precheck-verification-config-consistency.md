---
id: precheck-verification-config-consistency
source_spec: precheck-verification-config-consistency
created: 2026-02-28
decomposed: false
---

# Precheck Verification Config Consistency Implementation Plan

**Goal:** Enforce deterministic precheck/verification semantics by rejecting contradictory configuration where verification is enabled while precheck is disabled.

**Architecture:** Add a dedicated validation rule in the config load path that treats `precheck.enabled=false` plus `precheck.verification.enabled=true` as a hard conflict, then align tests and inline config comments with this parent-child contract.

**Tech Stack:** Go, YAML config parsing (`gopkg.in/yaml.v3`), Go testing package.

**Spec:** `.gromit/specs/precheck-verification-config-consistency.md`

---

## Architecture

## Architecture Proposal

**Overview:**  
Enforce a strict parent-child contract at config validation time: if `precheck.enabled=false` and `precheck.verification.enabled=true`, return a hard validation error so startup/config load fails deterministically.

**Key Components:**
1. **`Config.Validate()` precheck rule**: add a dedicated validation branch for parent/child consistency.
2. **Precheck accessors (read behavior only)**: keep current accessor defaults but make conflict impossible at runtime by blocking invalid config earlier.
3. **Config docs/default comments**: update `gromit.yaml` comments to clarify verification is subordinate to precheck.

**Integration Points:**
- Extend existing validation flow in `internal/config/config.go` so failure occurs during `Load()`.
- Update tests in `internal/config/config_test.go` near current precheck verification coverage.
- Align operator-facing inline comments in `gromit.yaml`.

**Data Flow:**  
YAML parse -> defaults applied -> validation checks parent/child precheck consistency -> conflict returns explicit error mentioning `precheck.enabled` and `precheck.verification.enabled` -> caller gets `validating config: ...` and startup halts.

**Files to Modify:**
- `internal/config/config.go` - add conflict validation and clear error text.
- `internal/config/config_test.go` - add/adjust tests for conflict rejection plus valid combos.
- `gromit.yaml` - update comments/default intent messaging.

**Files to Create:**
- None.

**Tradeoffs:**
- **Validate-time failure vs implicit disabling**: chose hard failure to avoid silent operator confusion.
- **Keep default verification=true vs flip default**: keep current default for compatibility, but enforce parent gate so contradictory explicit config is invalid.

## Test Strategy

**Test Levels:**
1. **Unit/config-validation tests**: cover `Load()`/`Validate()` behavior for precheck + verification combinations.
2. **Integration-style config parsing tests**: verify YAML inputs produce expected load success/failure and effective values.
3. **Manual sanity check**: run config load with the repo’s `gromit.yaml` after comment/default updates to ensure no ambiguity remains.

**Key Test Cases:**
- Default config (`precheck.enabled` default false, verification default true) should remain valid unless both are explicitly contradictory after defaults.
- `precheck.enabled: true` + `precheck.verification.enabled: true` loads successfully.
- `precheck.enabled: true` + `precheck.verification.enabled: false` loads successfully.
- `precheck.enabled: false` + `precheck.verification.enabled: false` loads successfully.
- `precheck.enabled: false` + `precheck.verification.enabled: true` fails with error mentioning both field paths: `precheck.enabled` and `precheck.verification.enabled`.

**Mocking Strategy:**
- No mocks needed; use real YAML parsing and config loading helpers already used in `config_test.go`.

**Coverage Goals:**
- Ensure conflict detection is strict and fail-fast in config load path.
- Ensure non-conflicting combinations preserve current behavior.
- Ensure error text is actionable and explicitly references parent-child relationship.

**Test Organization:**
- Add tests in `internal/config/config_test.go`, alongside existing precheck/verification tests.
- Prefer table-driven cases matching existing style in that file.

## Implementation Tasks

### Task 1: Add Precheck/Verification Conflict Validation

**Files:**
- Modify: `internal/config/config.go`

**What to Do:**
Add a focused validation helper (or inline validate branch) called from `Config.Validate()` that enforces the parent-child rule: when `precheck.enabled` resolves false and `precheck.verification.enabled` resolves true, return a validation error that clearly names both fields and states verification cannot run when precheck is disabled.

**Acceptance Criteria:**
- `Config.Validate()` rejects the conflicting combination with a deterministic error.
- Error message includes exact field paths `precheck.enabled` and `precheck.verification.enabled`.
- Non-conflicting combinations continue to validate successfully.

**Dependencies:**
- None.

**Notes:**
- Keep validation in the existing load-time path so startup fails fast before runtime phases begin.

### Task 2: Expand Config Tests for Parent/Child Semantics

**Files:**
- Modify: `internal/config/config_test.go`

**What to Do:**
Add/adjust precheck verification tests to cover all explicit enable/disable combinations and assert conflict handling for the contradictory case. Include a failure assertion that checks for both field paths in the error text.

**Acceptance Criteria:**
- Tests verify success for the three non-conflicting explicit combinations.
- Tests verify failure for `precheck.enabled: false` + `precheck.verification.enabled: true`.
- Test assertions confirm conflict error text references both full field paths.

**Dependencies:**
- Task 1.

**Notes:**
- Place tests near existing `TestPrecheckVerificationFromYAML` coverage to preserve discoverability.

### Task 3: Align Inline Config Documentation with Enforced Semantics

**Files:**
- Modify: `gromit.yaml`

**What to Do:**
Update inline comments in the `precheck` and `precheck.verification` section so they do not imply verification is independently effective. Clarify verification is subordinate and only meaningful when precheck is enabled.

**Acceptance Criteria:**
- Comments explicitly describe parent-child relationship between `precheck.enabled` and `precheck.verification.enabled`.
- Inline wording does not imply verification can run independently.
- Resulting example config remains syntactically valid YAML.

**Dependencies:**
- Task 1 (semantic source of truth).

**Notes:**
- Keep wording concise and operator-focused; avoid duplicating full validation text in comments.

### Task 4: Verify End-to-End Config Loading Behavior

**Files:**
- Modify: `internal/config/config_test.go` (if additional load-path assertion is needed)

**What to Do:**
Ensure the load path (`Load`) surfaces conflict errors with the `validating config:` prefix and retains existing behavior for valid configs. Add one targeted load-path assertion if current tests only check helper behavior.

**Acceptance Criteria:**
- A load-path test confirms startup/config-load failure for the conflict case.
- Error propagation remains user-facing and actionable.
- Existing config tests remain green.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
- Skip additional test file creation unless necessary; prefer extending current table-driven patterns.

---

## Notes

- Keep scope limited to config semantics, validation, and tests; no runtime execution-path changes should be needed once invalid config is blocked at load.
- During implementation, watch for default-value interactions (`SetDefaults`) so expectations in tests explicitly distinguish default behavior from explicit contradictory overrides.
- This plan is ready for `gromit decompose precheck-verification-config-consistency`.
