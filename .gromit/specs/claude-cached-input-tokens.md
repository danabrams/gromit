---
id: claude-cached-input-tokens
source_ideas: []
created: 2026-02-26
epic: provider-ecosystem
---

# Claude Provider CachedInputTokens Propagation

## Problem

`provider.Result` has a `CachedInputTokens` field, but `claude.Result` does not. The `convertResult()` function in the Claude provider always emits zero cached tokens, silently dropping any cache hit data from the stream JSON usage block and skewing cost reporting.

## Approach

- Add `CachedInputTokens int` field to `claude.Result` struct in `internal/claude/claude.go` (or wherever the struct is defined)
- Parse `cache_read_input_tokens` from the Claude stream JSON usage block in the stream processing path and populate `claude.Result.CachedInputTokens`
- Update `convertResult()` to map `claude.Result.CachedInputTokens` to `provider.Result.CachedInputTokens`
- Add a unit test using a fixture stream JSON payload that includes `cache_read_input_tokens` and assert the value propagates through `convertResult()` to `provider.Result`

## Files to Change

- `internal/claude/claude.go` — add `CachedInputTokens` to `Result`, parse from stream JSON usage block
- `internal/provider/claude.go` — update `convertResult()` to propagate `CachedInputTokens`
- `internal/provider/claude_test.go` — add test asserting non-zero `CachedInputTokens` when stream JSON contains `cache_read_input_tokens`

## Acceptance Criteria

- `claude.Result` defines a `CachedInputTokens` field
- Stream JSON usage blocks containing `cache_read_input_tokens` populate `claude.Result.CachedInputTokens`
- `convertResult()` maps `claude.Result.CachedInputTokens` to `provider.Result.CachedInputTokens`
- A test verifies the full propagation path from stream JSON to `provider.Result`
- Cost reporting for Claude invocations with cache hits reflects actual cached token counts
