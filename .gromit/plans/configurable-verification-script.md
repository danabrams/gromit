---
created: 2026-02-18T00:00:00Z
decomposed: true
decomposed_at: "2026-02-18T17:50:17Z"
id: configurable-verification-script
source_spec: configurable-verification-script
---

# Configurable Verification Script Implementation Plan

**Goal:** Replace hard-coded Go-specific commands in mandatory quality gates and pre-build compilation checks with config-driven fields, enabling non-Go projects to use Gromit.

**Architecture:** Add `validation.mandatory_commands` (list of prefixes) and `preflight.compile_command` (shell command string) to `gromit.yaml`. Remove the package-level `mandatoryQualityGateCommandPrefixes` variable and the `CompileCheck *bool` field. Empty/absent values disable the respective features.

**Tech Stack:** Go, YAML config

**Spec:** `.gromit/specs/configurable-verification-script.md`

---

## Architecture

**Key Components:**
1. **`ValidationConfig.MandatoryCommands []string`** — replaces the package-level `mandatoryQualityGateCommandPrefixes` variable in `lifecycle.go`
2. **`PreflightConfig.CompileCommand string`** — replaces `CompileCheck *bool` + hard-coded `"go build ./..."` string in `process.go`

**Integration Points:**
- `enforceMandatoryQualityGateCoverage` reads `r.cfg.Validation.MandatoryCommands` instead of the package-level var
- `missingMandatoryQualityCommands` takes a `mandatoryPrefixes []string` parameter
- `runCompilationCheck` reads `r.cfg.Preflight.CompileCommand` — empty string means skip
- `SetDefaults` no longer sets `CompileCheck`; no language-specific defaults for either new field
- `NormalizeNilFields` normalizes `MandatoryCommands` nil → empty slice

**Data Flow:**
- YAML → `Config.Validation.MandatoryCommands` → `enforceMandatoryQualityGateCoverage` → `missingMandatoryQualityCommands`
- YAML → `Config.Preflight.CompileCommand` → `runCompilationCheck` → inject errors into build prompt

**Tradeoffs:**
- No defaults: users must explicitly configure. Avoids surprising non-Go users.
- String replaces bool+hardcode: `compile_command` is self-documenting.
- Keep Go-specific regex cases in `mandatoryCommandPattern`: optimized paths that coexist with the generic `default` case.

## Test Strategy

**Unit Tests:**
- Config defaults: `MandatoryCommands` absent → empty slice; `CompileCommand` absent → empty string
- Config YAML deserialization for both fields
- `NormalizeNilFields`: `MandatoryCommands` nil → empty slice
- `missingMandatoryQualityCommands`: parameterized with arbitrary prefixes
- `runCompilationCheck`: uses configured command, skips when empty

**Integration Tests:**
- Mandatory gate enforcement with config-driven prefixes (existing tests updated)
- Empty `mandatory_commands` disables enforcement
- Compilation check with custom command string and disabled when empty

**Key Test Cases:**
- Config with explicit mandatory commands enforces only those
- Empty/absent mandatory commands skips enforcement entirely
- Config with compile command runs that command
- Empty/absent compile command skips pre-build check
- Existing wrapped-command regex tests still pass
- Backward compat: Go project behavior preserved when fields are populated

## Implementation Tasks

### Task 1: Add config fields and remove CompileCheck boolean

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**What to Do:**
Add `MandatoryCommands []string` with yaml tag `mandatory_commands` to `ValidationConfig`. Add `CompileCommand string` with yaml tag `compile_command` to `PreflightConfig`. Remove `CompileCheck *bool` field and its yaml tag from `PreflightConfig`. In `SetDefaults()`, remove the block that defaults `CompileCheck` to true. In `NormalizeNilFields()`, add normalization for `Validation.MandatoryCommands` (nil → empty slice). Update `config_test.go`: remove `TestSetDefaults_CompileCheckDefaultsTrue`, add tests for `MandatoryCommands` default (empty slice), `CompileCommand` default (empty string), and YAML deserialization of both fields.

**Acceptance Criteria:**
- `MandatoryCommands` defaults to empty slice when absent
- `CompileCommand` defaults to empty string when absent
- `CompileCheck *bool` no longer exists on `PreflightConfig`

**Dependencies:** None (foundational)

### Task 2: Make mandatory quality gate enforcement config-driven

**Files:**
- Modify: `internal/runner/lifecycle.go`
- Test: `internal/runner/process_test.go`

**What to Do:**
Delete the package-level `var mandatoryQualityGateCommandPrefixes` on line 22. Change `missingMandatoryQualityCommands` to accept a `mandatoryPrefixes []string` parameter instead of reading the global. Update `enforceMandatoryQualityGateCoverage` to pass `r.cfg.Validation.MandatoryCommands` to `missingMandatoryQualityCommands`. Add early return when `MandatoryCommands` is empty (no enforcement). Update existing quality gate tests in `process_test.go` to set `MandatoryCommands` on the config. Add a new test: config with empty `MandatoryCommands` allows any commands (no enforcement).

**Acceptance Criteria:**
- `mandatoryQualityGateCommandPrefixes` variable is deleted
- Empty `MandatoryCommands` disables enforcement entirely
- Existing quality gate tests pass with config-driven prefixes

**Dependencies:** Task 1

### Task 3: Make compilation check use configured command

**Files:**
- Modify: `internal/runner/process.go`
- Test: `internal/runner/compilation_check_test.go`

**What to Do:**
In `runCompilationCheck`, replace the `CompileCheck` boolean check with: if `r.cfg.Preflight.CompileCommand == ""`, return early. Replace the hard-coded `"go build ./..."` string in the `r.runCmd` call with `r.cfg.Preflight.CompileCommand`. Update all three tests in `compilation_check_test.go`: change `CompileCheck: &enabled` to `CompileCommand: "go build ./..."`, change `CompileCheck: &disabled` to `CompileCommand: ""` (or absent), and verify the configured command string is what gets executed.

**Acceptance Criteria:**
- Hard-coded `"go build ./..."` string removed from `process.go`
- Empty `CompileCommand` skips the pre-build check
- Non-empty `CompileCommand` runs that exact command

**Dependencies:** Task 1

### Task 4: Update gromit.yaml with explicit field values

**Files:**
- Modify: `gromit.yaml`

**What to Do:**
Add `mandatory_commands` list under the `validation:` section with the three Go commands: `"go test"`, `"go vet"`, `"go build"`. Add `compile_command: "go build ./..."` under the `preflight:` section. This preserves current behavior for this Go project while making it explicit.

**Acceptance Criteria:**
- `gromit.yaml` contains `mandatory_commands` with Go prefixes
- `gromit.yaml` contains `compile_command: "go build ./..."`
- Existing project behavior is unchanged

**Dependencies:** Task 1

### Task 5: Final verification

**Files:**
- None (verification only)

**What to Do:**
Run `go test ./...`, `go vet ./...`, and `go build ./...` to confirm all quality gates pass. Verify no remaining references to the deleted `mandatoryQualityGateCommandPrefixes` variable or `CompileCheck` field outside of test files.

**Acceptance Criteria:**
- All tests pass
- All vet checks pass
- Build succeeds

**Dependencies:** Tasks 1-4

---

## Notes

- The `mandatoryCommandPattern` function with its Go-specific `case` branches is intentionally kept — those are optimized regex paths that don't conflict with the generic `default` case. The spec notes this explicitly.
- The `CompileCheck *bool` removal may break any external code referencing that field. Within this repo, only `config.go`, `config_test.go`, `compilation_check_test.go`, `process.go`, `runner_test.go`, and `interfaces_test.go` reference it — all updated in Tasks 1-3.
- The spec says "no language-specific defaults" — `SetDefaults()` must NOT populate `MandatoryCommands` or `CompileCommand` with Go values. Only `gromit.yaml` carries the Go-specific values.
