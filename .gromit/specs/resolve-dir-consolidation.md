---
id: resolve-dir-consolidation
source_ideas: [idea-1770406823736]
created: 2026-02-06
---

# Consolidate resolve*Dir Helpers

## Specification

Move the three `resolve*Dir` helper functions into a single shared file `cmd/gromit/resolve.go`. These functions all follow the same pattern (check config for a path override, fall back to a default) but are currently scattered across three files:

- `resolveGromitDir` in `main.go:111-117`
- `resolveSpecsDir` in `refine.go:271-276`
- `resolvePlansDir` in `plan.go:205-210`

The new file groups them together, making it obvious where to find and add path resolution logic. No behavior changes — this is a pure code-organization move within the `main` package.

## Acceptance Criteria

- All three `resolve*Dir` functions live in `cmd/gromit/resolve.go` and are removed from their original files
- All existing callers (`main.go`, `refine.go`, `plan.go`, `decompose.go`, `backlog.go`, `add.go`, `review.go`) continue to compile and work unchanged
- `go build ./cmd/gromit` and `go test ./...` pass with no regressions

## Decisions

1. **Consolidate, don't eliminate.** A `config.Default()` approach that eliminates the nil-config pattern would be cleaner but touches more code and changes calling conventions. The consolidation approach is lower-risk and matches the original intent.

2. **New file, not config package.** These functions live in `cmd/gromit/` (the `main` package) because they bridge CLI concerns (nil config when `gromit.yaml` is missing) with config defaults. The `config` package shouldn't need to know about CLI fallback behavior.

## Research & Context

### Current State

The three functions exist because several commands (`refine`, `plan`, `add`, `backlog`) gracefully handle missing `gromit.yaml` by setting `cfg = nil`. When config is loaded successfully, `config.setDefaults()` (`internal/config/config.go:137-230`) already populates all path defaults — so the resolve functions only matter for the nil-config path.

Call sites:
- `resolveGromitDir`: `main.go` (retro), `refine.go`, `backlog.go`, `add.go`, `review.go` (3 times) — 6 total
- `resolveSpecsDir`: `refine.go`, `plan.go` — 2 total
- `resolvePlansDir`: `plan.go`, `decompose.go` — 2 total
