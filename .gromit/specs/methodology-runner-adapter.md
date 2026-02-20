---
id: methodology-runner-adapter
source_ideas:
  - idea-1771591754844
created: 2026-02-20
---

# Methodology Runner Adapter

## Specification

Generalize methodology command adaptation so ATDD/TDD flows are not hard-coded to Go command rewriting.

Current logic mutates `go test` commands (for example adding `-tags acceptance`) and assumes Go-specific test execution semantics. This blocks portability to Node/Python and forces non-Go projects to fight default behavior.

This spec introduces a methodology runner adapter interface selected by project profile.

### Adapter Contract

Add a profile-selected adapter interface (for example in `internal/runner/methodology/adapter.go`):

```go
type RunnerAdapter interface {
    // Build verification command set for "tests should fail before impl" checks.
    AcceptanceCommands(baseCommands []string, scope []string) []string

    // Build TDD red/green commands for the selected scope.
    TDDCommands(baseCommands []string, scope []string, phase string) []string

    // Validate command compatibility for methodology mode.
    Validate(commands []string) error
}
```

### Built-In Adapters

Provide:
- `go` adapter (preserves current behavior, including acceptance-tag injection semantics)
- `passthrough` adapter (no command mutation; used by `custom` until stack-specific adapters exist)

Follow-up specs may add `node` and `python` adapters with stack-specific transformations.

### Profile Integration

Adapter selection is driven by `project.profile`:
- `go` -> Go adapter
- `node|python|custom` -> passthrough adapter in this phase

No silent Go-specific rewrites occur for non-Go profiles.

### Behavioral Guarantees

- Existing Go methodology behavior is preserved for this repository.
- Non-Go profiles run configured commands as-is unless their adapter explicitly modifies them.
- Adapter selection and resulting command set are visible in debug output/logging for diagnosability.

## Acceptance Criteria

- Methodology command transformation logic is encapsulated behind `RunnerAdapter`.
- Go adapter retains current ATDD/TDD command behavior in existing tests.
- Non-Go profiles avoid Go-specific command rewrites.
- Unit tests cover adapter selection by profile and command-generation behavior.
- Integration tests verify no regressions in current Go ATDD/TDD flow.

## Execution Order

- Sequence position: 3
- Dependencies: `project-profiles-core`
- Unblocks: future Node/Python methodology adapters without changing orchestration core

## Decisions

1. **Adapter before per-language implementation** -- establish seam first, then add language-specific behavior incrementally.

2. **Passthrough for non-Go in first slice** -- avoids introducing incorrect assumptions for Node/Python before explicit design.

3. **Profile-driven selection** -- keeps behavior consistent with project profile and avoids ad-hoc command sniffing heuristics.

## Research & Context

### Current Coupling Points

- Methodology executor currently rewrites `go test` commands directly.
- Related tests assert Go-specific command mutation behavior.
- Validation and methodology flows share command sets that are currently interpreted with Go-first assumptions.

### Files to Change

| File | Change |
|------|--------|
| `internal/runner/methodology/*` | Add adapter interface + default implementations + wiring |
| `internal/config/*` | Expose profile-aware adapter selection input |
| `internal/runner/process_methodology*.go` | Replace direct rewrite logic with adapter calls |
| `internal/runner/methodology/*_test.go` | Preserve Go behavior tests and add profile-selection coverage |

### Out of Scope

- Defining complete Node/Python ATDD/TDD heuristics.
- Changing high-level methodology state machine or cycle limits.
