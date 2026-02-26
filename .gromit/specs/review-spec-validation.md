---
id: review-spec-validation
source_ideas: [idea-1770741396395]
created: 2026-02-11
epic: spec-first-atdd-execution
---

# Review --spec Flag Validation and ListWithLabel Priority Ordering

## Specification

Two improvements to the review command's `--spec` flag handling and the underlying `ListWithLabel` method:

### 1. Spec Name Validation with Helpful Errors

When `gromit review --spec <name>` is used, validate that `.gromit/specs/<name>.md` exists before querying beads. If the spec file doesn't exist, return an error that lists available spec names so the user can spot typos.

Add a `ValidateSpec(specsDir, specName string) error` function to `internal/scope/scope.go` that:
- Checks if `<specsDir>/<specName>.md` exists
- If not, reads the specs directory and lists available spec names in the error message
- Returns nil if the spec exists

Callers (`getSpecBaseCommit` in `review.go`, `main.go` retro spec handling) call `ValidateSpec` before `ResolveSpec`. This separates validation from label construction, keeping `ResolveSpec` pure.

### 2. ListWithLabel Priority Ordering

Add `--sort priority` to the `ListWithLabel` command invocation in `internal/bead/bead.go`, changing it from `bd list --json --label <label>` to `bd list --json --label <label> --sort priority`. This matches the pattern used by `List()` and ensures consistent, deterministic ordering for all callers.

Note: The related `--all` and `--limit 0` flags are covered by spec `list-with-label-all-statuses` and should not be included here.

## Acceptance Criteria

- `ValidateSpec` returns nil when the spec file exists at `<specsDir>/<specName>.md`
- `ValidateSpec` returns an error listing available spec names when the spec file doesn't exist
- `gromit review --spec nonexistent` shows "spec not found" with available names instead of "no beads found"
- `ListWithLabel` passes `--sort priority` to `bd list`
- Existing `ListWithLabel` tests are updated to expect the `--sort priority` argument

## Decisions

1. **Add ValidateSpec to scope.go rather than inline in review.go** — Multiple callers benefit from spec validation (review.go, main.go retro). Centralizing it in `scope.go` alongside `ResolveSpec` keeps related logic together and avoids duplication.

2. **Keep ResolveSpec pure (no validation)** — `ResolveSpec` is a simple name-to-label transformation with no error return. Adding file system validation there would change its signature and break callers. A separate `ValidateSpec` function preserves backward compatibility.

3. **List available specs in error, not fuzzy match** — Listing all available spec names is simpler than implementing edit-distance fuzzy matching and equally effective — users can spot their own typos in a short list.

4. **Add --sort priority independently of --all/--limit 0** — The `list-with-label-all-statuses` spec handles `--all` and `--limit 0`. Priority sorting is an orthogonal concern (ordering vs completeness) and can land separately.

## Research & Context

### Current State

**`ListWithLabel`** (`internal/bead/bead.go:616`):
```go
out, err := c.run("list", "--json", "--label", label)
```
Missing `--sort priority`. Compare with `List()` which uses `--sort priority --limit 0`.

**`ResolveSpec`** (`internal/scope/scope.go:14-16`):
```go
func ResolveSpec(specName string) []string {
    return []string{fmt.Sprintf("spec:%s", specName)}
}
```
Pure label construction — no validation, no file system access.

**`getSpecBaseCommit`** (`cmd/gromit/review.go:148-194`):
Calls `ResolveSpec` then `ListWithLabel`. When no beads are found, reports "no beads found for spec X" — indistinguishable from a typo in the spec name vs a spec with genuinely no beads.

### Callers of ResolveSpec

- `cmd/gromit/review.go:150` — review scope base commit detection
- `cmd/gromit/main.go:208` — retro bead filtering
- Tests: `review_test.go`, `retro_integration_test.go`, `retro_filter_test.go`

### Available Specs Directory

Spec files live at `.gromit/specs/<name>.md` (configurable via `cfg.Paths.Specs`). The directory typically contains 20-40 spec files.
