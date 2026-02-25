---
id: project-profiles-core
source_spec: project-profiles-core
created: 2026-02-25
decomposed: false
---

# Project Profiles Core Implementation Plan

**Goal:** Introduce a first-class `project.profile` system that drives stack-aware defaults for runtime behavior and `gromit init` while preserving backward compatibility.

**Architecture:** Add a centralized profile resolver in `internal/config` that computes effective defaults using explicit config first, then profile defaults, then legacy fallback where needed; wire runtime and init to consume this resolver.

**Tech Stack:** Go, Cobra CLI, YAML config (`gopkg.in/yaml.v3`), table-driven Go tests.

**Spec:** `.gromit/specs/project-profiles-core.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Add a profile-resolution layer in `internal/config` that computes effective defaults (`validation`, `preflight`, `methodology`) from `project.profile`, then make runtime and `init` consume that resolver instead of Go-specific assumptions.

**Key Components:**
1. **Profile catalog (`internal/config/profile_defaults.go`)**: defines built-in defaults for `go|node|python|custom` (validation fast/full/mandatory, preflight compile command, methodology adapter).
2. **Resolver API (`internal/config/compatibility_resolution.go` + accessors)**: extends current compatibility resolution to return profile-derived defaults with source metadata (`explicit`, `profile_default`, `legacy_fallback`).
3. **Validation/preflight accessors (`internal/config/config_accessors.go`)**: add profile-aware getters so runtime paths do `explicit config -> profile default -> legacy fallback`.
4. **Init profile detection (`cmd/gromit/init.go`)**: detect stack (`go.mod`, `package.json`, `pyproject.toml`/`requirements.txt`), support `--profile` override, and render profile-specific `gromit.yaml`.
5. **Template generation (`cmd/gromit/templates.go` or new init template builder)**: replace single hardcoded Node starter commands with profile-aware config generation.

**Integration Points:**
- Keep `project.profile` as config-first selector (already in `ProjectConfig`).
- Reuse existing compatibility abstraction (`ResolveCompatibilityContext`) as the single runtime resolver entrypoint.
- Update runner/validation/methodology call sites to use profile-aware accessors instead of direct `FastCommandsOrDefault`/raw adapter checks where needed.
- Preserve existing behavior for missing `project.profile` by resolving to Go via legacy fallback source.

**Data Flow:**
1. Load config -> normalize -> defaults.
2. Runtime asks resolver for effective profile and derived defaults.
3. For each command field:
   - if user explicitly set in `gromit.yaml`, use it;
   - else use built-in profile default;
   - else (migration safety) use existing hardcoded fallback.
4. `init` detects profile (or uses `--profile`), writes `project.profile`, and emits starter commands from profile catalog.

**Files to Modify:**
- `internal/config/config.go` - validate profile enum (`go|node|python|custom`) and relax old go-only compatibility check.
- `internal/config/config_accessors.go` - add profile-aware effective command/compile/adapter accessors.
- `internal/config/compatibility_resolution.go` - centralize profile default resolution and source tracking.
- `internal/config/config_defaults.go` - minimal wiring if needed to keep legacy defaults behavior intact.
- `cmd/gromit/init.go` - add `--profile`, repo signal detection, and profile-aware config emission.
- `cmd/gromit/templates.go` - move from single static default config to profile-aware config rendering.
- `internal/runner/policy/methodology.go` and targeted runtime callers - consume resolved methodology adapter from new resolver path.

**Files to Create:**
- `internal/config/profile_defaults.go` - built-in profile definitions and lookup helpers.
- `internal/config/profile_resolution_test.go` - precedence and compatibility unit coverage.
- `cmd/gromit/init_profile_test.go` - auto-detect and explicit override behavior tests.

**Tradeoffs:**
- **Central resolver vs scattered fallbacks**: chose central resolver to avoid drift and keep acceptance criterion “single profile resolver” true.
- **Keep legacy fallback temporarily**: chose to preserve existing hardcoded Go defaults behind source metadata for backward compatibility during migration.
- **`custom` as strict no-default commands**: chose explicit empties (no implicit toolchain injection) to enforce user intent and avoid hidden behavior.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: verify profile resolution precedence and source metadata in `internal/config` (explicit > profile default > legacy fallback), including `custom` no-injection behavior.
2. **Integration Tests**: verify `init` profile auto-detection and `--profile` override produce expected `gromit.yaml` output and `project.profile`.
3. **Manual Testing**: quick CLI smoke checks for `gromit init` in temp dirs representing Go/Node/Python repos.

**Key Test Cases:**
- Missing `project.profile` resolves to `go` with legacy-fallback source and preserves current runtime behavior.
- `project.profile: node` with no explicit validation commands resolves to node defaults.
- Explicit `validation.fast_commands/full_commands/mandatory_commands` always override profile defaults.
- Explicit `preflight.compile_command` overrides profile default compile command.
- `project.profile: custom` yields no implicit validation/preflight/methodology command defaults.
- Invalid profile value fails validation with clear enum error.
- `gromit init` detects:
  - `go.mod` => `go`
  - `package.json` (without go.mod) => `node`
  - `pyproject.toml` or `requirements.txt` (without go.mod/package.json) => `python`
  - none => `go`
- `gromit init --profile <x>` overrides auto-detection and writes selected profile.
- Generated `gromit.yaml` includes `project.profile` and profile-matched starter command sets.

**Mocking Strategy:**
- Keep config tests pure (no mocks) by instantiating `config.Config` structs directly.
- For `init` tests, use real temp directories/files and execute `runInit` logic with injected cwd/flags where possible.
- Avoid provider/runner mocks for this spec except minimal targeted assertions on adapter-gated behavior.

**Coverage Goals:**
- All profile variants (`go`, `node`, `python`, `custom`) covered in resolver tests.
- Precedence matrix covered for validation/preflight/methodology fields.
- Backward compatibility paths covered for omitted profile and legacy command fields.
- Detection priority order and override behavior covered for init.

**Test Organization:**
- Add `internal/config/profile_resolution_test.go` (and extend `compatibility_resolution_test.go` if cleaner).
- Add `cmd/gromit/init_profile_test.go`.
- Keep table-driven tests with explicit expected source/value pairs for maintainability.

## Implementation Tasks

### Task 1: Add Profile Catalog and Enum Validation

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/profile_defaults.go`
- Test: `internal/config/profile_resolution_test.go`

**What to Do:**
Define built-in profile defaults for `go`, `node`, `python`, and `custom` in a dedicated config package file. Update config validation to accept only this enum for explicit profile values and remove the current go-only compatibility restriction.

**Acceptance Criteria:**
- Explicit `project.profile` accepts only `go|node|python|custom`.
- Profile catalog exposes defaults for validation commands, mandatory command prefixes, preflight compile command, and methodology adapter.
- Invalid explicit profile values return a clear validation error mentioning accepted values.

**Dependencies:**
- None

**Notes:**
Keep missing `project.profile` behavior unchanged for compatibility; do not force profile to be explicitly set.

### Task 2: Implement Central Profile Resolver with Source Metadata

**Files:**
- Modify: `internal/config/compatibility_resolution.go`
- Modify: `internal/config/config_accessors.go`
- Test: `internal/config/profile_resolution_test.go`

**What to Do:**
Extend compatibility resolution to derive profile-dependent defaults through one resolver path. Add profile-aware accessors for effective validation fast/full/mandatory commands, preflight compile command, and methodology adapter.

**Acceptance Criteria:**
- Resolver applies precedence: explicit config > profile default > legacy fallback.
- Accessors expose both effective value and source metadata where needed.
- `custom` profile resolves to no implicit toolchain command defaults.

**Dependencies:**
- Task 1

**Notes:**
Avoid duplicating fallback logic in call sites; accessors should be the single entry for runtime defaulting.

### Task 3: Wire Runtime Callers to Profile-Aware Accessors

**Files:**
- Modify: `internal/runner/validation/runner.go`
- Modify: `internal/runner/policy/validation.go`
- Modify: `internal/runner/policy/methodology.go`
- Test: `internal/runner/policy/methodology_test.go` (targeted additions)

**What to Do:**
Switch validation/methodology runtime paths to consume profile-aware resolved accessors instead of direct raw config fields where defaults are currently assumed.

**Acceptance Criteria:**
- Runtime validation command selection uses resolved fast/full commands and mandatory prefixes.
- Methodology adapter gating uses resolved adapter consistently.
- Behavior for repositories without explicit profile remains equivalent to current go-profile behavior.

**Dependencies:**
- Task 2

**Notes:**
Keep edits targeted to command selection points; avoid broad refactors unrelated to profile resolution.

### Task 4: Add Profile-Aware Init Detection and Override Flag

**Files:**
- Modify: `cmd/gromit/init.go`
- Modify: `cmd/gromit/templates.go` (or new helper file for config rendering)
- Test: `cmd/gromit/init_profile_test.go`

**What to Do:**
Add profile detection in `gromit init` based on repo files and support explicit `--profile` override. Generate `gromit.yaml` with `project.profile` plus profile-specific starter commands.

**Acceptance Criteria:**
- Detection order works as specified: `go.mod` > `package.json` > `pyproject.toml|requirements.txt` > default `go`.
- `--profile` forces selected profile regardless of detected files.
- Generated config includes `project.profile` and profile-matched starter commands.

**Dependencies:**
- Task 1

**Notes:**
Maintain existing init side effects (template creation, RULES/LEARNINGS generation, gitignore handling).

### Task 5: Backward Compatibility and Precedence Regression Coverage

**Files:**
- Modify: `internal/config/compatibility_resolution_test.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/gromit/init_profile_test.go`

**What to Do:**
Add and update tests that lock compatibility behavior for missing profile and verify precedence across explicit command settings and profile defaults.

**Acceptance Criteria:**
- Legacy configs without `project.profile` continue resolving as go profile with no breaking errors.
- Explicit command fields in `gromit.yaml` override profile defaults in all tested paths.
- `custom` profile tests confirm no implicit injection of validation/preflight commands.

**Dependencies:**
- Task 2
- Task 4

**Notes:**
Use table-driven tests to keep precedence matrix readable and extensible.

### Task 6: End-to-End Verification and Fixture Alignment

**Files:**
- Modify: `cmd/gromit/run_spec_flag_test.go` (only if fixture expectations change)
- Modify: `internal/config/migration_compatibility_fixtures_test.go` (as needed)
- Test: existing config/init/runner suites

**What to Do:**
Run targeted and broad test suites, adjust compatibility fixtures only when expected behavior intentionally changes, and confirm no regression in existing go-default workflows.

**Acceptance Criteria:**
- `go test ./internal/config/... ./cmd/gromit/... ./internal/runner/...` passes.
- No unintended behavior changes in existing go-profile runtime paths.
- Any fixture update is intentional, documented in test names/messages, and limited to profile-related deltas.

**Dependencies:**
- Task 3
- Task 5

**Notes:**
Keep fixture churn minimal to preserve signal for future migration work.

---

## Notes

- Keep the profile resolver as the only place that knows built-in toolchain defaults.
- Preserve compatibility markers/source metadata so future strict cutover work remains measurable.
- `custom` profile should be intentionally sparse: no automatic validation/preflight command injection.
- This plan is foundation work and is expected to unblock tracker adapter, methodology adapter, and profile-aware bootstrap follow-ons.
