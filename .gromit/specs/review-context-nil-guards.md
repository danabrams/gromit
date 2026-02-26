---
id: review-context-nil-guards
source_ideas: []
created: 2026-02-26
epic: codebase-health
---

# ReviewContext and ThoroughReviewContext Nil Guards

## Problem

`ReviewContext.ValidationCommands` and `ThoroughReviewContext.CompletedBeads` slices have no nil normalization, unlike `Context` and `ScopeEstimate` which already define `normalizeNilFields()`. Template rendering panics or produces unexpected output when callers pass nil slices.

## Approach

- Add `normalizeNilFields()` method to `ReviewContext` in `internal/prompt/context_types.go` that initializes `ValidationCommands` to an empty slice when nil
- Add `normalizeNilFields()` method to `ThoroughReviewContext` that initializes `CompletedBeads` to an empty slice when nil
- Audit `TDDRedContext.TestFileContents` and `TDDGreenContext.ImplFileContents` map fields for similar nil guards; add initialization to empty map if nil
- Call `normalizeNilFields()` at the start of any `Build()` or render function that accepts these context types, matching the pattern used by `Context` and `ScopeEstimate`
- Add table-driven tests for nil-input and empty-input cases to verify templates render without panic

## Files to Change

- `internal/prompt/context_types.go` — add `normalizeNilFields()` to `ReviewContext`, `ThoroughReviewContext`, `TDDRedContext`, `TDDGreenContext`
- `internal/prompt/prompt.go` — verify call sites invoke normalization before rendering
- `internal/prompt/context_types_test.go` — add nil-guard tests

## Acceptance Criteria

- `ReviewContext.normalizeNilFields()` converts nil `ValidationCommands` to `[]string{}`
- `ThoroughReviewContext.normalizeNilFields()` converts nil `CompletedBeads` to `[]string{}`
- `TDDRedContext` and `TDDGreenContext` nil map fields are initialized before template rendering
- All template render functions that accept these types call `normalizeNilFields()` before use
- Existing tests continue to pass; new nil-input tests pass
