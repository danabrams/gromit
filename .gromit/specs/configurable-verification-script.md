---
id: configurable-verification-script
source_ideas: []
created: 2026-02-18
epic: codebase-health
---

# Configurable Verification Script

## Specification

Gromit hard-codes Go-specific commands in two places: a mandatory quality gate that requires `go test`, `go vet`, and `go build` prefixes in validation command sets, and a pre-build compilation check that runs `go build ./...` before each Claude invocation. These hard-codings prevent non-Go projects from using Gromit.

This spec makes both configurable through `gromit.yaml`, with no language-specific defaults. Users must explicitly declare their mandatory commands and compilation check. Existing Go projects add two fields to their config; non-Go projects configure their own toolchain.

### Mandatory Quality Gate Commands

A new `validation.mandatory_commands` field replaces the hard-coded `mandatoryQualityGateCommandPrefixes` variable in `lifecycle.go`. The runner checks that each prefix appears in the configured fast or full command sets. When the list is empty or absent, no enforcement runs.

```yaml
validation:
  mandatory_commands:
    - "go test"
    - "go vet"
    - "go build"
```

The existing `enforceMandatoryQualityGateCoverage` function reads from `r.cfg.Validation.MandatoryCommands` instead of the package-level variable. The `mandatoryCommandPattern` function already handles arbitrary prefixes through its `default` case, so it needs no changes.

### Pre-Build Compilation Check

A new `preflight.compile_command` field replaces the hard-coded `"go build ./..."` in `runCompilationCheck`. When set, the command runs before each Claude invocation; compilation errors are injected into the build prompt so the agent can fix them. When empty or absent, no pre-check runs.

```yaml
preflight:
  compile_command: "go build ./..."
```

The existing `preflight.compile_check` boolean field is removed. The empty-vs-set string check replaces it.

### No Language-Specific Defaults

When `mandatory_commands` is absent, the runner enforces nothing. When `compile_command` is absent, no pre-check runs. Gromit ships no Go defaults. Users declare what their project needs.

## Acceptance Criteria

- `validation.mandatory_commands` accepts a list of command prefixes to enforce in fast/full command sets
- An empty or absent `mandatory_commands` disables enforcement entirely
- `preflight.compile_command` accepts a shell command string for pre-build checks
- An empty or absent `compile_command` disables the pre-build check entirely
- The hard-coded `mandatoryQualityGateCommandPrefixes` variable is removed from `lifecycle.go`
- The hard-coded `"go build ./..."` string is removed from `process.go`
- The `preflight.compile_check` boolean field is removed from `PreflightConfig`
- This project's `gromit.yaml` is updated with explicit `mandatory_commands` and `compile_command` values to preserve current behavior

## Decisions

1. **No defaults** -- Users must explicitly configure their mandatory commands and compile check. This avoids surprising non-Go users with Go-specific enforcement and makes the config self-documenting.

2. **Config fields, not removal of enforcement** -- The mandatory quality gate concept is useful as a footgun guard. Rather than deleting it, we make it configurable. Users who want no enforcement set an empty list.

3. **String field replaces boolean for compile check** -- `compile_command: "go build ./..."` is clearer than a separate boolean toggle plus a hard-coded command. One field controls both "whether" and "what."

4. **Existing pattern matching works generically** -- `mandatoryCommandPattern` in `lifecycle.go` already has a `default` case that handles arbitrary prefixes via `regexp.QuoteMeta`. The Go-specific `case` branches for `"go test"`, `"go vet"`, `"go build"` can remain as optimized paths or be removed; the default path covers all languages.

## Research & Context

### Current State

**Config** (`internal/config/config.go`):
- `ValidationConfig` has `FastCommands`, `FullCommands`, and `Commands` (legacy fallback) -- all user-configurable lists
- `PreflightConfig` has `CompileCheck *bool` that gates the hard-coded `"go build ./..."` check

**Mandatory enforcement** (`internal/runner/lifecycle.go`):
- Line 22: `var mandatoryQualityGateCommandPrefixes = []string{"go test", "go vet", "go build"}`
- `enforceMandatoryQualityGateCoverage` checks that configured commands contain these prefixes
- Fast gates get a fallback: if fast commands lack a prefix but full commands have it, enforcement passes

**Compilation pre-check** (`internal/runner/process.go`):
- `runCompilationCheck` runs `"go build ./..."` with a 30-second timeout
- Injects stderr into the build prompt on failure
- Non-blocking: never prevents the bead from proceeding

### Files to Change

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `MandatoryCommands []string` to `ValidationConfig`, add `CompileCommand string` to `PreflightConfig`, remove `CompileCheck *bool` |
| `internal/runner/lifecycle.go` | Delete `mandatoryQualityGateCommandPrefixes` var, read from config in `enforceMandatoryQualityGateCoverage` |
| `internal/runner/process.go` | Read `cfg.Preflight.CompileCommand` in `runCompilationCheck` instead of hard-coded string |
| `gromit.yaml` | Add `mandatory_commands` and `compile_command` fields |
| Test files | Update tests for config-driven behavior, add tests for empty/absent fields |
