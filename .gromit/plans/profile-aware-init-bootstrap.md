---
id: profile-aware-init-bootstrap
source_spec: profile-aware-init-bootstrap
created: 2026-02-26
decomposed: false
---

# Profile-Aware Init Bootstrap Implementation Plan

**Goal:** Make `gromit init` scaffold profile-appropriate config and guidance so non-Go repositories get correct first-run defaults without manual rewrites.

**Architecture:** Keep the existing init flow and add thin profile-aware rendering for generated guidance (RULES/next steps/template notes), while preserving current Go behavior and reusing centralized profile defaults.

**Tech Stack:** Go, Cobra CLI, YAML config generation, table-driven Go tests.

**Spec:** `.gromit/specs/profile-aware-init-bootstrap.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Keep the current init architecture and add a thin profile-aware generation layer for guidance text so config behavior stays backward-compatible while non-Go scaffolds stop inheriting Go-oriented wording.

**Key Components:**
1. **`cmd/gromit/init.go` profile-aware scaffolding hooks**: Pass selected profile into rule/template/message generation.
2. **`cmd/gromit/templates.go` profile-aware renderers**: Replace static `defaultRules` and static next-step messaging with profile-parameterized helpers while keeping shared templates.
3. **`internal/config/profile_defaults.go` extension (if needed)**: Keep command defaults centralized; optionally add small guidance metadata if it reduces duplication.
4. **`cmd/gromit/init*_test.go` matrix coverage**: Add assertions for generated RULES/next-step content by profile and preserve Go backward compatibility.

**Integration Points:**
- Reuse existing `selectInitProfile` and `configForProfile` flow in `runInit`.
- Keep existing template constants shared; inject profile-specific snippets only in command-example sections.
- Preserve existing file creation paths and force/skip semantics.

**Data Flow:**
`runInit` resolves profile -> writes `gromit.yaml` via `configForProfile(profile)` -> writes profile-aware RULES content and profile-aware “next steps” output -> writes shared templates with minimal profile-specific command-example substitutions.

**Files to Modify:**
- `cmd/gromit/init.go` - wire profile into generated RULES and CLI next-step output helpers.
- `cmd/gromit/templates.go` - add renderer helpers for profile-aware RULES/guidance and optional template note seeding.
- `cmd/gromit/init_profile_test.go` - extend assertions for RULES/next-step content by profile.
- `cmd/gromit/init_templates_test.go` (or new focused init test file) - add profile-specific template/guidance tests.

**Files to Create:**
- Optional: `cmd/gromit/init_guidance_test.go` for focused output/guidance assertions if existing tests become noisy.

**Tradeoffs:**
- **Helper render functions over per-profile full templates**: avoids template sprawl while keeping behavior explicit.
- **Minimal targeted variation over broad profile branching**: keeps Go behavior stable and lowers regression risk.
- **Extend current tests over isolated new suite only**: keeps profile behavior checks near existing init tests and reduces duplication.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Profile selection and content rendering helpers (RULES/guidance/template note helpers).
2. **Integration Tests**: `runInit` end-to-end scaffold generation per profile in temp dirs.
3. **Manual Testing**: Quick smoke of `gromit init --profile <x>` in representative repos (Go/Node/Python/custom markers) to validate UX text.

**Key Test Cases:**
- Explicit `--profile` overrides existing config and repo signals.
- Existing `gromit.yaml project.profile` is used when no explicit override is set.
- Repo signal detection maps to expected profile (go > node > python > custom fallback).
- Generated `gromit.yaml` includes `project.profile` for all profiles.
- Go profile output remains backward-compatible (existing commands/compile defaults and wording).
- Node/Python/custom generated RULES and next-step text do not contain Go-only validation guidance.
- Template command-example sections include appropriate profile notes where variation is required.
- Invalid profile in config and invalid YAML continue to fail with clear errors.

**Mocking Strategy:**
- No heavy mocks needed; use temp directories and real file writes for scaffold verification.
- Capture stdout from `runInit` for next-step guidance assertions.
- Keep profile default resolution real via `internal/config` helpers.

**Coverage Goals:**
- Critical paths: profile resolution precedence and profile-aware generated content.
- Edge cases: ambiguous signals, missing markers, invalid overrides/config, empty compile command (python/custom).
- Regression guard: unchanged Go defaults and guidance semantics unless explicitly intended.

**Test Organization:**
- Keep profile resolution/scaffold assertions in `cmd/gromit/init_profile_test.go`.
- Add focused tests in `cmd/gromit/init_templates_test.go` or `cmd/gromit/init_guidance_test.go` for helper rendering behavior.
- Use table-driven tests keyed by profile for maintainability and low duplication.

## Implementation Tasks

### Task 1: Add Profile-Aware Guidance Renderers

**Files:**
- Modify: `cmd/gromit/templates.go`
- Modify: `cmd/gromit/init.go`
- Test: `cmd/gromit/init_profile_test.go`

**What to Do:**
Implement helper functions that render profile-specific RULES text and init “next steps” guidance. Update `runInit` to use these helpers based on the selected profile instead of static strings.

**Acceptance Criteria:**
- `runInit` renders `.gromit/RULES.md` from a profile-aware helper.
- CLI next-step output is generated from a profile-aware helper.
- Go profile preserves current wording and command guidance unless intentionally updated.

**Dependencies:**
- None.

**Notes:**
- Keep language-neutral sections unchanged.
- Restrict variation to validation/tooling guidance where profile differences matter.

### Task 2: Seed Profile Notes in Shared Templates

**Files:**
- Modify: `cmd/gromit/templates.go`
- Modify: `cmd/gromit/init.go`
- Test: `cmd/gromit/init_templates_test.go`

**What to Do:**
Keep shared base prompt templates but add targeted profile-aware note seeding in sections that show command examples (only where needed).

**Acceptance Criteria:**
- Template structure remains shared and unchanged outside targeted note sections.
- Non-Go profiles no longer get Go-only command examples in seeded guidance.
- Go profile template output remains backward-compatible.

**Dependencies:**
- Task 1.

**Notes:**
- Prefer small helper substitutions over branching entire template constants.

### Task 3: Extend Init Profile Matrix Tests

**Files:**
- Modify: `cmd/gromit/init_profile_test.go`
- Modify or Create: `cmd/gromit/init_guidance_test.go`

**What to Do:**
Expand table-driven tests to assert generated RULES and next-step text per profile, including explicit override behavior and non-Go guidance correctness.

**Acceptance Criteria:**
- Tests verify `go|node|python|custom` outputs for profile-sensitive guidance.
- Tests assert explicit `--profile` precedence over config/signals for generated content.
- Tests fail if non-Go profiles include Go-only validation guidance by default.

**Dependencies:**
- Task 1.

**Notes:**
- Capture stdout in tests for next-step text validation.

### Task 4: Validate Config Defaults and Regression Safety

**Files:**
- Modify: `internal/config/profile_defaults.go` (only if additional metadata is required)
- Modify: `internal/config/*_test.go` (targeted additions)
- Test: `cmd/gromit/init_profile_test.go`

**What to Do:**
Confirm profile default data remains canonical and sufficient for init generation. Add/adjust tests for any added metadata and verify Go default behavior remains compatible.

**Acceptance Criteria:**
- `project.profile` and per-profile validation/compile defaults remain correctly generated.
- Any new profile metadata is covered by unit tests.
- Go init config output and existing profile tests remain green.

**Dependencies:**
- Task 1.
- Task 2.

**Notes:**
- Only extend `internal/config` types if init guidance cannot be implemented cleanly in `cmd/gromit` helpers.

### Task 5: Run Verification and Document Manual Smoke Checks

**Files:**
- Modify: `cmd/gromit/init_profile_test.go` (if gaps found)
- Optional notes update: `docs/` (if project keeps init behavior notes)

**What to Do:**
Run targeted tests for init/profile generation and perform quick manual smoke checks in marker-based temp repos to validate profile detection and override UX.

**Acceptance Criteria:**
- Targeted init/profile tests pass reliably.
- Manual checks confirm detection precedence and profile-aware generated guidance.
- Any discovered gaps are either fixed in-scope or tracked as follow-up beads.

**Dependencies:**
- Task 2.
- Task 3.
- Task 4.

**Notes:**
- Keep this task focused on validation and stabilization, not feature expansion.

---

## Notes

- Existing code already satisfies substantial parts of the spec (profile flag, precedence, and config defaults). This plan focuses on remaining profile-aware guidance/template behavior and regression-proof testing.
- Keep profile-specific differences intentionally narrow to avoid future template divergence.
- If additional profile families are introduced later, extend helper renderers and test matrices rather than cloning templates.
