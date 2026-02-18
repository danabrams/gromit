---
id: result-output-semantics
source_ideas: [idea-1771413407538]
created: 2026-02-18
---

# Clean Result.Output Semantics

## Problem

`provider.Result` has both `Output` and `Stdout` fields with inconsistent semantics across providers:

- **Claude:** sets `Output` and `Stdout` to the same value (duplicated).
- **Codex (failure paths):** sets `Output` to stdout+stderr combined, `Stdout` to stdout only.
- **Codex (success paths):** sets both to the same value.

No consumer code reads `Stdout`. Every consumer reads `Output` to parse markers (`VALIDATION_PASSED`, `PRECHECK_PASSED`), JSON responses, and structured content. The `Stdout` field is dead.

The inconsistency means `Output` sometimes contains stderr noise that could corrupt marker detection or JSON parsing in Codex failure paths.

## Design

### Semantic Contract

| Field | Meaning |
|-------|---------|
| `Output` | The provider's semantic response text. Consumers parse this for markers, JSON, and structured content. Never includes process-level stderr. |
| `Stderr` | Raw stderr from the subprocess. Used by ATDD logging and diagnostics. |
| `Diagnostics` | Structured diagnostic info built from args, environment, and stderr. |

Remove the `Stdout` field entirely.

### Changes

**`internal/provider/provider.go`** — Remove `Stdout` from the `Result` struct.

**`internal/provider/claude.go`** — Remove the `Stdout: claudeResult.Output` line from `convertResult`.

**`internal/provider/codex.go`** — Three sites:
1. `runOnce`: Change `Output` from `stdout.String() + stderr.String()` to `stdout.String()`. Remove `Stdout` field.
2. `streamRunOnce` failure path (cmd.Wait error): Change `Output` from `resultText + stderr.String()` to `resultText`. Remove `Stdout` field.
3. `streamRunOnce` success and stream-error paths: Remove `Stdout` field (already identical to `Output`).

**`internal/provider/codex_test.go`** — Update all `result.Stdout` assertions to use `result.Output`. In the stderr-only test, `result.Output` becomes empty (the provider produced no semantic response), and `result.Stderr` still contains the error.

### What Stays the Same

- No consumer code changes. Every consumer already reads `Output`.
- `Stderr` stays. ATDD logging reads it at `callbacks.go:437`.
- `Diagnostics` stays. It holds structured diagnostic content.
- Shared helpers in `helpers.go` stay. They read `Output`.
