---
created: 2026-02-11T00:00:00Z
decomposed: true
decomposed_at: "2026-02-11T07:04:37-05:00"
id: review-spec-validation
source_spec: review-spec-validation
---

# Review --spec Flag Validation and ListWithLabel Priority Ordering Implementation Plan

**Goal:** Validate spec names before querying beads (with helpful error messages listing available specs) and add `--sort priority` to `ListWithLabel` for consistent ordering.

**Architecture:** Add `ValidateSpec` to `internal/scope/scope.go` for file-existence checking, wire it into the two callers (`review.go`, `main.go`), and add `--sort priority` to the `ListWithLabel` command invocation.

**Tech Stack:** Go, standard library (`os`, `path/filepath`, `strings`)

**Spec:** `.gromit/specs/review-spec-validation.md`

---

## Architecture

**Key Components:**
1. **`scope.ValidateSpec(specsDir, specName string) error`** — Checks if `<specsDir>/<specName>.md` exists. If not, reads the directory and returns an error listing available spec names. Returns nil if the spec exists.
2. **`ListWithLabel` sort fix** — One-line addition of `"--sort", "priority"` to the `c.run()` call in `internal/bead/bead.go`.

**Integration Points:**
- `getSpecBaseCommit` in `review.go` gains a `specsDir` parameter and calls `ValidateSpec` before `ResolveSpec`
- `determineReviewScope` passes `cfg.Paths.Specs` to `getSpecBaseCommit`
- Retro handler in `main.go` calls `ValidateSpec` before `ResolveSpec`

**Data Flow (review --spec):**
1. User runs `gromit review --spec foo`
2. `determineReviewScope` calls `getSpecBaseCommit("foo", cfg.Paths.Specs)`
3. `getSpecBaseCommit` calls `scope.ValidateSpec(specsDir, "foo")` — returns error if spec file missing
4. If valid, proceeds to `scope.ResolveSpec("foo")` → `ListWithLabel("spec:foo")` as before

## Test Strategy

**ValidateSpec tests** in `internal/scope/scope_test.go`:
- Returns nil when spec file exists
- Returns error with available spec names when spec doesn't exist
- Error lists available names (strips `.md` extension)
- Handles empty/nonexistent specs directory
- Ignores non-`.md` files when listing

Uses real temp directories (same pattern as existing `ResolveEpic` tests). No mocks needed.

**ListWithLabel**: No new parsing tests needed — `--sort priority` is in the `c.run()` call, existing tests parse JSON via helper functions.

## Implementation Tasks

### Task 1: Add ValidateSpec function and tests to scope package

**Files:**
- Modify: `internal/scope/scope.go`
- Modify: `internal/scope/scope_test.go`

**What to Do:**
Add `ValidateSpec(specsDir, specName string) error` to `scope.go`. It should:
- Check if `filepath.Join(specsDir, specName+".md")` exists via `os.Stat`
- If the file exists, return nil
- If not, read the specs directory with `os.ReadDir`, collect `.md` filenames (stripping the extension), and return an error like: `spec "foo" not found; available specs: bar, baz, qux`
- Handle edge cases: nonexistent specsDir (return error mentioning directory), empty specsDir (return error with no available specs listed)

Add tests following the existing `ResolveEpic` test pattern using `t.TempDir()`.

**Acceptance Criteria:**
- `ValidateSpec` returns nil when the spec file exists at `<specsDir>/<specName>.md`
- `ValidateSpec` returns an error listing available spec names when the spec file doesn't exist
- Tests cover: exists, not-exists with alternatives, empty dir, nonexistent dir, non-`.md` files ignored

**Dependencies:** None

### Task 2: Wire ValidateSpec into review.go and main.go callers

**Files:**
- Modify: `cmd/gromit/review.go`
- Modify: `cmd/gromit/main.go`

**What to Do:**
In `review.go`:
- Change `getSpecBaseCommit(specName string)` signature to `getSpecBaseCommit(specName, specsDir string)`
- Add `scope.ValidateSpec(specsDir, specName)` call at the top, returning error if validation fails
- Update the caller in `determineReviewScope` (line 119) to pass `cfg.Paths.Specs` as the second argument

In `main.go`:
- Before the existing `labels = scope.ResolveSpec(retroSpecFlag)` at line 208, add a `scope.ValidateSpec` call using `filepath.Join(gromitDir, "specs")` as the specsDir
- Return error if validation fails

**Acceptance Criteria:**
- `gromit review --spec nonexistent` shows "spec not found" with available names instead of "no beads found"
- Retro with `--spec nonexistent` also shows the validation error

**Dependencies:** Task 1

### Task 3: Add --sort priority to ListWithLabel

**Files:**
- Modify: `internal/bead/bead.go`

**What to Do:**
Change line 616 from:
```go
out, err := c.run("list", "--json", "--label", label)
```
to:
```go
out, err := c.run("list", "--json", "--label", label, "--sort", "priority")
```
This matches the pattern used by `List()` at line 519.

**Acceptance Criteria:**
- `ListWithLabel` passes `--sort priority` to `bd list`

**Dependencies:** None (independent of Tasks 1-2)

---

## Notes

- Task 3 is fully independent and can be implemented in parallel with Tasks 1-2.
- The existing `ListWithLabel` tests in `list_with_label_test.go` use `parseListWithLabelOutput` helper functions that test JSON parsing, not the `c.run()` args — they won't need updates for the `--sort priority` change.
- The `runner/interfaces.go` `BeadClient` interface has `ListWithLabel(label string)` — no signature change, so no mock updates needed.
