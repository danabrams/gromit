---
id: normalize-nil-fields-convention
source_spec: normalize-nil-fields-convention
created: 2026-02-26
decomposed: false
---

# Normalize Nil Fields Naming Convention Policy Implementation Plan

**Goal:** Document and enforce a clear visibility convention for nil-field normalization methods so future additions consistently use exported or unexported method names based on call scope.

**Architecture:** Add a policy to `CLAUDE.md`, audit existing normalize methods and call sites for visibility consistency, and annotate one unexported and one exported example method with references to the policy.

**Tech Stack:** Go, Markdown, ripgrep (`rg`) for repository audit

**Spec:** `.gromit/specs/normalize-nil-fields-convention.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Add a documented policy in `CLAUDE.md`, validate current usage aligns with it, and add short policy-reference comments on one unexported and one exported example method.

**Key Components:**
1. **`CLAUDE.md` policy section**: Defines visibility rule for nil-normalization methods.
2. **`specgate.GateVerdict.normalizeNilFields`**: Example of unexported/package-local usage with inline policy reference.
3. **`runtypes.SubTask.NormalizeNilFields`**: Example of exported/cross-package usage with inline policy reference.
4. **Repository-wide audit pass**: Confirms existing naming/visibility usage matches the policy and flags/fixes mismatches.

**Integration Points:**
- Uses existing coding conventions doc (`CLAUDE.md`)
- Touches current method definitions only (no behavior change)
- Keeps API surface unchanged unless audit finds a real mismatch

**Data Flow:**
No runtime flow changes; this is a documentation + convention-enforcement pass. The audit inspects call sites to determine whether methods are used only intra-package or inter-package, then aligns visibility if needed.

**Files to Modify:**
- `CLAUDE.md` - add "Code Patterns"/convention note
- `internal/specgate/verdict.go` - add comment referencing policy for unexported method example
- `internal/runner/runtypes/types.go` - add comment referencing policy for exported method example
- Potential additional files only if audit reveals mismatches

**Files to Create:**
- None expected

**Tradeoffs:**
- Chose minimal edits over broad refactors because spec asks for policy clarity, not behavior changes
- Chose `GateVerdict` as concrete example despite spec saying `GateResult`, because `GateResult` does not exist in current code

## Test Strategy

## Test Strategy

**Test Levels:**
1. **Unit Tests:** No new unit tests required if only comments/docs are added and audit confirms no behavior/API changes.
2. **Integration Tests:** Not needed unless audit forces visibility changes; if so, run affected package tests plus dependents.
3. **Manual Verification:** Validate convention text and comment references are clear and consistent with actual usages.

**Key Test Cases:**
- `normalizeNilFields` methods used only within defining package are unexported.
- `NormalizeNilFields` methods called from other packages are exported.
- `GateVerdict.normalizeNilFields` and `SubTask.NormalizeNilFields` include brief comments pointing to policy in `CLAUDE.md`.
- No new exported method introduced where package-local usage is sufficient.

**Mocking Strategy:**
- No mocks needed; this is static code/documentation validation.

**Coverage Goals:**
- Full coverage of all `normalizeNilFields`/`NormalizeNilFields` definitions and cross-package call sites.
- If any visibility mismatch is fixed, ensure compile/test coverage for impacted package boundaries.

**Test Organization:**
- Prefer command-line validation:
  - `rg` audit for method definitions and call sites
  - `go test` on impacted packages only if code visibility changes occur

## Implementation Tasks

### Task 1: Audit normalize method visibility and usage

**Files:**
- Modify: none expected
- Test/Verify: repository-wide search results (`internal/**`)

**What to Do:**
Run a complete audit of `normalizeNilFields` and `NormalizeNilFields` definitions and their call sites to confirm visibility matches usage scope (package-local vs cross-package). Record any mismatches and decide whether a rename/export change is required.

**Acceptance Criteria:**
- Every normalize method definition is classified as package-local or cross-package based on actual callers.
- Any mismatch between visibility and caller scope is identified with exact file/function references.
- If no mismatches exist, audit results explicitly state that no API/code changes are required.

**Dependencies:**
- None

**Notes:**
- The spec references `GateResult`; map this to `GateVerdict` in current codebase and note the mapping in implementation notes or review output.

### Task 2: Document visibility convention in CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**What to Do:**
Add a concise convention entry under a "Code Patterns" or equivalent section defining when to use unexported `normalizeNilFields()` versus exported `NormalizeNilFields()`.

**Acceptance Criteria:**
- Policy clearly states: unexported for package-local calls, exported for cross-package calls.
- Wording aligns with Go visibility conventions and minimal exported surface principle.
- Policy location is easy to find from top-level architecture/conventions guidance.

**Dependencies:**
- Task 1

**Notes:**
- Keep policy generic enough to apply beyond current examples.

### Task 3: Add policy-reference comments to representative methods

**Files:**
- Modify: `internal/specgate/verdict.go`
- Modify: `internal/runner/runtypes/types.go`

**What to Do:**
Add brief inline comments near `GateVerdict.normalizeNilFields()` and `SubTask.NormalizeNilFields()` referencing the documented policy in `CLAUDE.md`, showing one unexported and one exported example.

**Acceptance Criteria:**
- `GateVerdict.normalizeNilFields()` comment explains package-local/unexported choice.
- `SubTask.NormalizeNilFields()` comment explains cross-package/exported choice.
- Comments are concise, non-redundant, and consistent with the policy text.

**Dependencies:**
- Task 1
- Task 2

**Notes:**
- If Task 1 found mismatches requiring code changes, apply those first and adjust representative examples accordingly.

### Task 4: Validate and finalize with minimal regression checks

**Files:**
- Modify: only if mismatch fixes were required
- Test: impacted package tests (conditional)

**What to Do:**
Perform final validation that policy, comments, and method visibility are in sync. If visibility changes were made, run targeted `go test` for impacted packages; otherwise, run lightweight verification commands only.

**Acceptance Criteria:**
- Documented policy and method comments are internally consistent.
- Post-change audit shows no visibility/usage mismatches.
- Any code/API change from mismatch fixes is covered by passing targeted tests.

**Dependencies:**
- Task 3

**Notes:**
- For documentation-only updates, avoid unnecessary full-suite test runs.

---

## Notes

- This plan intentionally treats the work as policy and consistency enforcement, not feature development.
- The spec’s `GateResult` example appears stale relative to current code; implementation should use `GateVerdict` as the canonical example.
- Keep the resulting edits small and focused to avoid accidental API-surface expansion.
