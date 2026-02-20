---
id: project-profiles-core
source_ideas:
  - idea-1771591754833
  - idea-1771591754849
created: 2026-02-20
---

# Project Profiles Core

## Specification

Introduce a first-class project profile system so Gromit can run outside this repository and outside Go without requiring manual config surgery.

Today, defaults and process guidance are biased toward this codebase (Go quality gates, bead-centric language, and repo-specific assumptions). This spec defines a profile layer that controls stack-specific defaults while preserving backward compatibility for existing users.

### Profile Model

Add a new top-level config section:

```yaml
project:
  profile: "go" # go | node | python | custom
```

`project.profile` selects profile behavior for initialization and runtime defaults.

### Built-In Profiles

Provide built-in profiles:
- `go` (default, current behavior)
- `node`
- `python`
- `custom`

Built-ins define defaults for:
- validation command templates (`fast_commands`, `full_commands`, `mandatory_commands`)
- preflight compile command
- methodology command adapter selection
- initialization scaffolding text/templates

### Runtime Resolution Rules

Profile defaults are only fallback values. User-configured fields in `gromit.yaml` always win.

Resolution order:
1. Explicit value in `gromit.yaml`
2. Built-in profile default
3. Existing hard-coded fallback (only where needed for compatibility during migration)

`custom` profile applies no toolchain defaults and requires explicit command configuration.

### Init Behavior

`gromit init` becomes profile-aware:
- defaults to `go` when it detects `go.mod` (or no clear signal)
- picks `node` for `package.json` without `go.mod`
- picks `python` for `pyproject.toml`/`requirements.txt` without `go.mod`/`package.json`
- supports explicit override flag (for example `--profile`)

Generated `gromit.yaml` should include `project.profile` and profile-appropriate starter commands.

### Compatibility

For existing repositories without `project.profile`:
- treat as `go` profile
- preserve behavior of current validation/preflight defaults
- emit no breaking errors

## Acceptance Criteria

- Config schema supports `project.profile` with values `go|node|python|custom`
- Runtime has a single profile resolver used by run/init/validation setup paths
- User-specified command fields in `gromit.yaml` override profile defaults
- `custom` profile performs no implicit toolchain command injection
- `gromit init` writes `project.profile` and profile-matched starter config
- Repositories with no `project.profile` continue behaving as `go` profile by default
- Unit tests verify profile resolution precedence and backward compatibility
- Init tests verify profile auto-detection and explicit override behavior

## Execution Order

- Sequence position: 1 (foundation spec)
- Dependencies: none
- Unblocks: `tracker-adapter-interface`, `methodology-runner-adapter`, `profile-aware-init-bootstrap`

## Decisions

1. **Profile is config-first, not repo-global state** -- profile selection lives in `gromit.yaml` so behavior is explicit and versioned with the project.

2. **Built-ins plus custom** -- built-ins reduce setup friction for common stacks; `custom` avoids forcing opinionated defaults.

3. **No silent override of explicit user config** -- profile defaults only fill missing fields; they never rewrite user intent.

4. **Backward compatibility first** -- missing profile maps to `go` to avoid breaking current installations and existing tests.

## Research & Context

### Current Coupling Points

- `gromit.yaml` in this repository assumes Go validation and compile commands.
- `.gromit/RULES.md` includes project-specific language such as "For this project: go test/go vet/go build only."
- `cmd/gromit/init.go` emits Go-leaning setup guidance and defaults.
- Methodology command adaptation today is centered on `go test`-style transformation.

### Files to Change

| File | Change |
|------|--------|
| `internal/config/config_types.go` | Add `ProjectConfig` with `Profile` field |
| `internal/config/config.go` | Set default profile and normalize/validate profile values |
| `internal/config/defaults.go` (or equivalent) | Add profile default resolution helpers |
| `cmd/gromit/init.go` | Add profile detection/override and profile-aware generated config |
| `cmd/gromit/init*_test.go` | Add profile auto-detect and override tests |
| `internal/runner/*` (targeted) | Read commands through profile resolver where defaults are applied |

### Out of Scope

- Tracker abstraction (`bd` replacement) is handled in `tracker-adapter-interface`.
- Methodology execution abstraction beyond wiring profile selection is handled in `methodology-runner-adapter`.
- Full template/rules profile packs beyond init baseline are handled in `profile-aware-init-bootstrap`.
