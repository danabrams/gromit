---
id: profile-aware-init-bootstrap
source_ideas:
  - idea-1771591754849
created: 2026-02-20
---

# Profile-Aware Init Bootstrap

## Specification

Update `gromit init` to scaffold profile-appropriate project artifacts so first-run experience works for non-Go repositories without manual rewrites.

Current bootstrap is strongly Go/self-hosting oriented (default command set, guidance text, generated expectations). This spec makes initialization aware of `project.profile` and stack signals, while preserving current behavior for Go projects.

### Profile Selection

`gromit init` supports:
- auto-detect profile from repo files
- explicit `--profile` override

Detection precedence:
1. explicit `--profile`
2. existing `gromit.yaml` profile (if present)
3. repo signals (`go.mod`, `package.json`, `pyproject.toml`, `requirements.txt`)
4. fallback to `go`

### Generated Config

Init writes `gromit.yaml` with:
- `project.profile`
- profile-matched starter values for validation commands
- profile-matched preflight compile command defaults (or empty when not applicable)
- comments that explain how to customize per project

### Generated Rules and Guidance

Init-generated `.gromit/RULES.md` and next-step output should:
- keep universal safety/test/process rules
- avoid Go-only language for non-Go profiles
- include short profile-specific notes on validation conventions

### Template Seeding

Base prompt templates remain shared, but init can seed profile notes in sections where command examples are shown. Template structure remains unchanged unless profile-specific variation is required.

## Acceptance Criteria

- `gromit init` accepts `--profile` with `go|node|python|custom`
- auto-detection produces expected profile for representative repo markers
- generated `gromit.yaml` always includes `project.profile`
- Go profile output remains backward-compatible with current behavior
- non-Go profiles do not ship Go-only validation guidance by default
- init tests cover profile detection, explicit override, and generated file content

## Execution Order

- Sequence position: 4
- Dependencies: `project-profiles-core`
- Unblocks: smoother onboarding for non-Go repositories using finalized profile defaults

## Decisions

1. **Profile is chosen at init but editable later** -- init provides a sane starting point; users can still tune config manually.

2. **Shared templates with minimal variation** -- avoid template sprawl; keep profile differences focused on commands and guidance text.

3. **No hard failure on ambiguous detection** -- ambiguity falls back to `go` unless explicitly overridden, preserving existing UX.

## Research & Context

### Current Coupling Points

- `cmd/gromit/init.go` hard-codes Go-oriented config and setup messaging.
- default scaffold references beads + Go flow patterns that are not universally valid.
- existing tests assert generated artifacts and must be extended for profile variants.

### Files to Change

| File | Change |
|------|--------|
| `cmd/gromit/init.go` | Add profile flag, detection, and profile-aware generation |
| `cmd/gromit/init*_test.go` | Add profile matrix tests for config/rules/messages |
| `internal/config/*` | Add helper defaults used by init generation |
| `.gromit/templates/*` (targeted) | Add profile-aware command examples only where needed |

### Out of Scope

- Full per-profile prompt template families.
- Tracker backend selection during init (handled by tracker follow-up specs).
