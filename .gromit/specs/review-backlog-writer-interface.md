---
id: review-backlog-writer-interface
source_ideas: []
created: 2026-02-26
epic: codebase-health
---

# Narrow BacklogWriter Interface for Review Pipeline

## Problem

`cmd/gromit/review.go` defines a `cliBacklogClient` with stub `List` and `Get` methods that return "not implemented" errors. The review pipeline only ever calls `Add` and `Update`, making the stubs dead code that silently swallows accidental calls instead of surfacing them as bugs.

## Approach

- Define a narrower `BacklogWriter` interface in `cmd/gromit/review.go` (or a shared internal file) with only the two methods the review pipeline actually uses: `Add` and `Update`
- Change the review pipeline's dependency type from the full `BacklogClient` interface to `BacklogWriter`
- Delete the `List` and `Get` stub methods from `cliBacklogClient` since they are no longer required by any interface
- Verify that no review pipeline code paths call `List` or `Get` before deletion
- Add a compile-time interface check (`var _ BacklogWriter = (*cliBacklogClient)(nil)`) to lock in the narrower surface

## Files to Change

- `cmd/gromit/review.go` — define `BacklogWriter` interface, update pipeline dependency, remove stub methods from `cliBacklogClient`
- `cmd/gromit/review_test.go` — update any test fixtures that construct `cliBacklogClient` with the removed methods

## Acceptance Criteria

- `BacklogWriter` interface is defined with only `Add` and `Update` methods
- Review pipeline uses `BacklogWriter` as its dependency type, not the full `BacklogClient`
- `cliBacklogClient` has no `List` or `Get` methods
- Compile-time interface check is present
- All existing review pipeline tests pass
- No code paths in review pipeline call `List` or `Get`
