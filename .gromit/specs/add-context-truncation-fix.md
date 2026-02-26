---
id: add-context-truncation-fix
source_ideas: []
created: 2026-02-07
epic: codebase-health
---

# Fix `gromit add` Context Field Truncation

## Specification

The `gromit add` command prompts users for optional additional context after capturing the idea text. Currently, `fmt.Scanln` is used to read the context input, which stops reading at the first whitespace character. This means only the first word of the user's context is captured.

Replace `fmt.Scanln` with `bufio.Scanner` (or `bufio.NewReader`) to read the entire line of input, preserving multi-word context strings.

The fix should also audit all other `fmt.Scanln` calls in the codebase. The only other instance is the category choice prompt (line 61), which reads a single character ("1", "2", or "3") and is not affected by this bug. No changes needed there.

## Acceptance Criteria

- `gromit add` captures the full multi-word context string when the user types additional context (e.g., "this should work with the new auth system" is stored completely, not just "this")
- Empty input (pressing Enter with no text) still results in an empty context field
- The category choice prompt (1/2/3) continues to work correctly
- A test exists that verifies multi-word context is captured in full

## Decisions

1. **Use `bufio.Scanner` over `bufio.NewReader`** Both work, but `bufio.Scanner` is idiomatic Go for line-at-a-time input and has cleaner error handling. The scanner approach reads cleanly with `scanner.Scan()` + `scanner.Text()`.

2. **Leave the category choice prompt as-is** The `fmt.Scanln(&choice)` call on line 61 only reads a single character, so the whitespace truncation behavior is harmless there. Changing it would add complexity without fixing a bug.

## Research & Context

### Current State

- **Bug location**: `cmd/gromit/add.go`, line 80 — `fmt.Scanln(&context)`
- **Backlog struct**: `internal/backlog/backlog.go` — `Idea` struct has a `Context string` field that correctly handles multi-word strings
- **Storage**: `.gromit/backlog.jsonl` — JSON marshaling/unmarshaling works correctly for multi-word context
- **Evidence of truncation**: Existing backlog entries show `"context":"this"` and `"context":"TDD"` — clearly truncated first words of longer input
- **Existing tests**: `internal/backlog/backlog_test.go` has `TestIdeaFieldsRoundtrip` which verifies the Context field round-trips through storage, but no test covers the CLI input reading behavior

### Audit Results

Only two `fmt.Scanln` calls exist in the codebase, both in `cmd/gromit/add.go`:
1. Line 61: Category choice — single character, not affected
2. Line 80: Context input — **the bug**, multi-word input truncated
