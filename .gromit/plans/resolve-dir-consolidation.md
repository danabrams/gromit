---
created: 2026-02-06T00:00:00Z
decomposed: true
decomposed_at: "2026-02-06T21:37:05-05:00"
id: resolve-dir-consolidation
source_spec: resolve-dir-consolidation
---

# Consolidate resolve*Dir Helpers — Implementation Plan

**Goal:** Move three scattered `resolve*Dir` functions into a single `cmd/gromit/resolve.go` file.

**Architecture:** Create one new file, delete functions from three existing files. Pure code-organization move within `package main` — no caller changes, no behavior changes.

**Tech Stack:** Go

**Spec:** `.gromit/specs/resolve-dir-consolidation.md`

---

## Architecture

Create `cmd/gromit/resolve.go` containing `resolveGromitDir`, `resolveSpecsDir`, and `resolvePlansDir`. Remove the originals from `main.go`, `refine.go`, and `plan.go`. All files are `package main`, so callers work unchanged with no import updates.

The only import needed in the new file is `github.com/danabrams/gromit/internal/config`.

After removal, check whether `refine.go` and `plan.go` still use the `config` import for other purposes — if not, remove it to avoid unused-import compile errors. (`refine.go` uses `config.Config` in `resolveSpecsDir` only but also references it nowhere else directly — however it imports `config` via the `backlog` and `skills` packages indirectly. Need to verify.) (`plan.go` uses `config.Config` in `resolvePlansDir` and also in `runPlan` via `loadConfig` return type — so `config` import stays.)

Verified: `refine.go` does not reference `config.Config` or any `config.*` symbol outside `resolveSpecsDir`, so the `config` import must be removed. `plan.go` uses `config.Config` in the `frontmatter` import and `bead` import lines but not directly — actually `plan.go` imports `config` and uses it only in `resolveSpecsDir`/`resolvePlansDir`. After removing both functions, the `config` import becomes unused and must be removed.

## Test Strategy

- `go build ./cmd/gromit` — primary compilation check
- `go test ./...` — no regressions
- `golangci-lint run ./...` — no unused imports or lint issues
- No new tests needed — these are trivial nil-check helpers covered by existing caller integration

## Implementation Tasks

### Task 1: Move resolve*Dir functions to cmd/gromit/resolve.go

**Files:**
- Create: `cmd/gromit/resolve.go`
- Modify: `cmd/gromit/main.go` (remove `resolveGromitDir`)
- Modify: `cmd/gromit/refine.go` (remove `resolveSpecsDir`, remove `config` import)
- Modify: `cmd/gromit/plan.go` (remove `resolvePlansDir`, remove `config` import if unused)

**What to Do:**
1. Create `cmd/gromit/resolve.go` with `package main`, import `config`, and all three functions copied verbatim (including their doc comments)
2. Delete `resolveGromitDir` from `main.go` (lines 111-117)
3. Delete `resolveSpecsDir` from `refine.go` (lines 270-276) and remove the `config` import
4. Delete `resolvePlansDir` from `plan.go` (lines 204-210) and remove the `config` import if no other references remain
5. Run `go build ./cmd/gromit` and `go test ./...`

**Acceptance Criteria:**
- All three functions exist only in `cmd/gromit/resolve.go`
- `go build ./cmd/gromit` and `go test ./...` pass

**Dependencies:** None

---

## Notes

- This is an atomic change — Go won't compile with duplicate function definitions in the same package, so create + delete must happen in one step.
- The `config` import in `refine.go` is used only by `resolveSpecsDir`. After removal, it becomes unused and must be deleted or the build fails.
- Verify `plan.go`'s `config` import usage before removing — `loadConfig()` returns `*config.Config` but that's defined in `main.go`, so `plan.go` may or may not need the import depending on whether it references `config.*` directly.
