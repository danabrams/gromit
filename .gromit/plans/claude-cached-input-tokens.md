---
created: 2026-02-26T00:00:00Z
decomposed: true
decomposed_at: "2026-02-26T02:38:36Z"
id: claude-cached-input-tokens
source_spec: claude-cached-input-tokens
---

# Claude Cached Input Tokens Propagation Implementation Plan

**Goal:** Preserve Claude cache-hit token accounting by propagating `cache_read_input_tokens` from stream JSON usage payloads into `provider.Result.CachedInputTokens`.

**Architecture:** Extend Claude stream usage parsing to capture cached input tokens, store that value on `claude.Result`, and map it through the provider conversion layer without changing external provider interfaces.

**Tech Stack:** Go, Claude CLI stream-json parsing, provider abstraction and unit tests.

**Spec:** `.gromit/specs/claude-cached-input-tokens.md`

---

## Architecture

## Architecture Proposal

**Overview:**
Add cached-token support at the source (`claude` stream parser), store it on `claude.Result`, and propagate it through `provider/claude` conversion so downstream cost accounting uses real cache-hit data.

**Key Components:**
1. **`internal/claude.Result`**: Add `CachedInputTokens int` as the transport field for Claude usage parsing output.
2. **`processStreamJSONWithCost` in `internal/claude/claude.go`**: Extend usage/result parsing to read `cache_read_input_tokens` from stream JSON usage blocks.
3. **`convertResult()` in `internal/provider/claude.go`**: Map `claude.Result.CachedInputTokens` to `provider.Result.CachedInputTokens`.
4. **Provider regression test (`internal/provider/claude_test.go` and/or `claude_cost_tracking_test.go`)**: Verify end-to-end propagation from Claude stream payload to provider result.

**Integration Points:**
- No API contract changes for provider consumers; field already exists in `provider.Result`.
- Claude stream parsing is the only source-of-truth change.
- Provider conversion layer gets a one-field mapping extension.

**Data Flow:**
`Claude stream JSON line ("result" with "usage.cache_read_input_tokens")`
-> parsed in `processStreamJSONWithCost`
-> returned from `StreamRun` into `claude.Result.CachedInputTokens`
-> `convertResult()` copies into `provider.Result.CachedInputTokens`
-> downstream reporting sees non-zero cached tokens.

**Files to Modify:**
- `internal/claude/claude.go` - add `CachedInputTokens` to `Result`, parse `cache_read_input_tokens` in usage/result event structs, return it from stream parser, wire into `StreamRun` result.
- `internal/provider/claude.go` - include `CachedInputTokens` in `convertResult()`.
- `internal/provider/claude_test.go` (or `internal/provider/claude_cost_tracking_test.go`) - add propagation test using stream-style JSON payload with `cache_read_input_tokens`.
- `internal/claude/cost_tracking_test.go` - parser-level assertion for `cache_read_input_tokens` extraction.

**Files to Create:**
- Optional: `test/fixtures/claude_stream_cache_usage.jsonl` if fixture-backed payload reuse is preferred over inline JSON in tests.

**Tradeoffs:**
- **Parse at Claude layer vs provider layer**: Chose Claude layer so token semantics stay provider-client specific and `provider` remains a pure mapper.
- **Usage-only key vs alias support**: Start with `cache_read_input_tokens` per spec; add aliases later only if real CLI variants require them.
- **Inline test JSON vs fixture file**: Inline is simpler; fixture file is better if multiple tests share payloads.

## Test Strategy

**Test Levels:**
1. **Unit tests (Claude parser):**
- Extend `internal/claude/cost_tracking_test.go` to cover `cache_read_input_tokens` extraction from a `result.usage` block.
- Assert existing fields still parse correctly alongside cached tokens.

2. **Unit tests (provider conversion):**
- Extend provider Claude conversion tests to assert `convertResult()` maps `claude.Result.CachedInputTokens` to `provider.Result.CachedInputTokens`.
- Add a full-path test that starts from stream JSON parsing behavior (mock client result shaped from parsed values) and verifies final provider result carries non-zero cached tokens.

3. **Manual testing:**
- Not required; behavior is deterministic and fully unit-testable with fixture/inline JSON lines.

**Key Test Cases:**
- `result.usage.cache_read_input_tokens` present: parsed value is non-zero and preserved.
- Top-level token fields present plus nested usage cached field: cached field still extracted correctly.
- `convertResult()` with non-zero cached tokens: output result preserves value.
- Backward compatibility: no cached field in payload yields zero value without breaking other usage fields.

**Mocking Strategy:**
- Use existing stream JSON parsing tests with in-memory `strings.NewReader` payloads (no external process).
- Use existing provider tests with direct `claude.Result` construction or mock client returns (no real Claude CLI).

**Coverage Goals:**
- Critical path: stream JSON usage parsing -> `claude.Result` -> `provider.Result`.
- Edge behavior: missing cached field remains zero and does not alter cost/input/output parsing.

**Test Organization:**
- Keep parser tests in `internal/claude/cost_tracking_test.go`.
- Keep mapping tests in `internal/provider/claude_cost_tracking_test.go` (preferred) or `internal/provider/claude_test.go` if staying with current conversion tests there.
- Follow existing `TestXxx...` naming and style used in nearby cost-tracking tests.

## Implementation Tasks

### Task 1: Extend Claude Result and Stream Usage Parsing

**Files:**
- Modify: `internal/claude/claude.go`
- Test: `internal/claude/cost_tracking_test.go`

**What to Do:**
Add `CachedInputTokens` to `claude.Result`. Update stream event usage parsing in `processStreamJSONWithCost` to read `usage.cache_read_input_tokens` (and any direct result field needed for compatibility), return it from the parser helper, and assign it in the `StreamRun` result object.

**Acceptance Criteria:**
- `claude.Result` includes `CachedInputTokens int`.
- `processStreamJSONWithCost` extracts non-zero `cache_read_input_tokens` values from stream JSON usage blocks.
- Existing cost/input/output extraction behavior remains unchanged.

**Dependencies:**
- None.

**Notes:**
Mirror existing nested-usage precedence logic (top-level fields first, then usage fallback) so behavior is consistent with current token/cost parsing.

### Task 2: Propagate Cached Tokens Through Provider Conversion

**Files:**
- Modify: `internal/provider/claude.go`
- Test: `internal/provider/claude_cost_tracking_test.go`

**What to Do:**
Update `convertResult()` to map `claude.Result.CachedInputTokens` onto `provider.Result.CachedInputTokens`. Add a unit test that asserts non-zero cached token values survive conversion.

**Acceptance Criteria:**
- `convertResult()` sets `CachedInputTokens` in `provider.Result`.
- Conversion test fails before mapping change and passes after.
- Existing mapped fields (`CostUSD`, `InputTokens`, `OutputTokens`, etc.) continue to pass tests.

**Dependencies:**
- Task 1 (provides source field on `claude.Result`).

**Notes:**
Keep this task limited to conversion and conversion-focused tests to avoid mixing parser behavior and mapping behavior in one test scope.

### Task 3: Add End-to-End Propagation Regression Test

**Files:**
- Modify: `internal/provider/claude_test.go` (or `internal/provider/claude_cost_tracking_test.go`)
- Optional Create: `test/fixtures/claude_stream_cache_usage.jsonl`

**What to Do:**
Add a regression test that exercises the full intended propagation path and asserts cached token accounting is non-zero at provider level when stream JSON includes `cache_read_input_tokens`.

**Acceptance Criteria:**
- Test asserts stream usage payload with `cache_read_input_tokens` leads to non-zero `provider.Result.CachedInputTokens`.
- Test protects against regressions where cached token field is silently dropped.
- Test is deterministic and runs in default unit-test lane.

**Dependencies:**
- Task 1 (parser support).
- Task 2 (conversion support).

**Notes:**
Prefer inline JSON payload unless reuse across multiple tests justifies fixture extraction.

---

## Notes

- Keep changes narrowly scoped to Claude provider accounting; avoid unrelated parser refactors.
- Run targeted tests at minimum: `go test ./internal/claude ./internal/provider`.
- If Claude CLI schema evolves, add compatibility parsing in a follow-up bead and link it as `discovered-from` this plan's implementation work.
