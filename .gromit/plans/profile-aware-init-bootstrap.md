---
created: 2026-02-26T00:00:00Z
decomposed: true
decomposed_at: "2026-02-26T12:50:09Z"
id: profile-aware-init-bootstrap
source_spec: profile-aware-init-bootstrap
---

# Profile-Aware Init Bootstrap Implementation Plan

**Goal:** Make `gromit init` scaffold profile-appropriate configuration and guidance so non-Go repositories work out-of-the-box without manual rewrites, while preserving current Go behavior.

**Architecture:** Extend existing profile-aware init plumbing (already present for profile selection and config defaults) with profile-aware guidance rendering for RULES/next-step output and minimal template note variation in command-example sections.

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
4. **`cmd/gromit/init*_test.go` matrix coverage**: Add assertions for generated RULES/next-step text and preserve Go backward compatibility.

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
- Chose **helper render functions** over per-profile full templates to avoid template sprawl.
- Chose **minimal targeted variation** over broad profile branching to keep Go behavior stable and reduce regression risk.
- Chose **extending current tests** over only adding new files to keep behavioral checks close to existing init profile tests.

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

### Task 1: Add profile-aware guidance render helpers

**Files:**
- Modify: `cmd/gromit/templates.go`
- Modify: `internal/config/profile_defaults.go`
- Test: `internal/config/profile_defaults_test.go` (or existing profile defaults test file)

**What to Do:**
Add small helper APIs for init-time guidance text, reusing profile catalog defaults as source of truth. Keep shared template structure intact; only inject profile-specific guidance where command examples are shown.

**Acceptance Criteria:**
- A renderer/helper returns profile-tailored validation guidance snippets for `go|node|python|custom`.
- Helper output avoids Go-specific command guidance for non-Go profiles.
- Existing config default behavior for Go remains unchanged.

**Dependencies:**
- None

**Notes:**
Prefer minimal metadata additions in `internal/config` if it prevents duplicating profile switch logic in multiple files.

### Task 2: Wire profile-aware init output generation

**Files:**
- Modify: `cmd/gromit/init.go`
- Modify: `cmd/gromit/templates.go`
- Test: `cmd/gromit/init_profile_test.go`

**What to Do:**
Use the selected profile to generate `.gromit/RULES.md` content and next-step terminal guidance. Keep universal process/safety rules, but add short profile-specific notes and remove Go-only wording for non-Go profiles.

**Acceptance Criteria:**
- `runInit` writes profile-appropriate RULES content by selected profile.
- init next-step output reflects profile conventions and is not Go-only for non-Go profiles.
- `gromit init --profile go` remains backward-compatible with existing guidance semantics.

**Dependencies:**
- Task 1

**Notes:**
Keep changes isolated to scaffolding text; avoid changing unrelated init behavior.

### Task 3: Seed profile-aware template command examples

**Files:**
- Modify: `cmd/gromit/templates.go`
- Test: `cmd/gromit/init_templates_test.go`

**What to Do:**
In shared base templates, introduce minimal profile-aware note insertion only in command-example sections where language-specific commands are shown. Do not create per-profile template families.

**Acceptance Criteria:**
- Shared templates remain structurally identical across profiles except targeted command-example notes.
- Non-Go profiles do not receive Go-only command examples by default in seeded sections.
- Existing template rendering/tests remain green except intentional profile-aware changes.

**Dependencies:**
- Task 1

**Notes:**
Avoid broad template churn; keep diffs narrow and predictable.

### Task 4: Expand init profile matrix coverage

**Files:**
- Modify: `cmd/gromit/init_profile_test.go`
- Modify: `cmd/gromit/init_templates_test.go` (or create `cmd/gromit/init_guidance_test.go`)

**What to Do:**
Add table-driven tests covering profile detection/override precedence and generated content assertions for `gromit.yaml`, RULES, templates, and next-step output.

**Acceptance Criteria:**
- Tests verify precedence order: explicit flag > existing config profile > repo markers > fallback.
- Tests assert `project.profile` always exists in generated config.
- Tests assert non-Go scaffolds exclude Go-only validation guidance by default.

**Dependencies:**
- Task 2
- Task 3

**Notes:**
Capture stdout where needed for next-step assertions; keep tests deterministic with temp dirs.

### Task 5: Regression and compatibility validation

**Files:**
- Modify: `cmd/gromit/init_profile_test.go`
- Modify: `cmd/gromit/run_spec_flag_test.go` (only if profile text assumptions require updates)

**What to Do:**
Run focused regression checks to ensure profile-aware init changes do not break existing Go/default profile assumptions elsewhere in CLI tests.

**Acceptance Criteria:**
- Go-profile init tests pass with backward-compatible defaults and guidance.
- Existing tests that depend on seeded `project.profile: go` continue to pass.
- Any changed assertions are limited to intentional guidance/profile behavior updates.

**Dependencies:**
- Task 4

**Notes:**
Keep compatibility updates minimal and justified by spec behavior.

---

## Notes

- Current repo already implements key parts of this spec (profile flag, precedence, and config defaults), so implementation should focus on remaining guidance/template deltas and tightening tests.
- If a pre-existing plan file appears in other branches, avoid duplicate plan IDs unless intentionally regenerating with `--force`.
- During implementation, keep profile logic centralized to reduce future divergence between config defaults and text guidance.
