# Spec 0004n — Distillation Prompt Output Format Fix

## spec_id
distillation-prompt-output-format

## Vision

Every `review record` call currently produces a non-blocking distillation error. These errors are silent in practice — they don't fail the command — but they mean doctrine proposals are never extracted and the distillation feature is effectively broken in production. The root cause is that the prompt instructs the LLM to "extract proposals" without specifying what format to return, so the LLM guesses and the parser silently fails. When failures do happen, the error message shows parser mechanics rather than the actual LLM output, so debugging requires code spelunking. This spec fixes the prompt and adds enough diagnostic signal that future failures are self-describing.

## Summary

This spec fixes the recurring distillation parse failures in `reviewdistiller` by adding explicit JSON output format instructions to all three prompt builders (`accepted`, `rework_implementation_gap`, `rework_vision_change`), and adds a diagnostic log line in `parseProposalsFromJSON` that emits the extracted JSON string when both parse attempts fail. No behavioral changes to the distillation flow — it remains non-blocking. The result is distillation that succeeds on well-formed LLM output and, when it does fail, produces an error message showing exactly what the LLM returned.

## Goals

### Primary
- Prompts explicitly specify the JSON array output format, field names, and field types
- `parseProposalsFromJSON` logs the extracted JSON string on failure so errors are self-describing

### Secondary
- Format instructions are consistent across all three outcome types

## Non-goals
- Making distillation blocking on failure
- Changing the `Proposal` struct or adding new proposal types
- Fixing the distillation error that appears when rejecting a null run (different issue)
- Parser hardening / type coercion for malformed fields

## Architecture

Two changes, both confined to `internal/next/reviewdistiller/`:

**1. Output format instructions in `prompts.go`**

Each of the three `build*Instructions()` functions gains a `## Output Format` section appended to its return string:

```
## Output Format

Return ONLY a JSON array. No prose, no markdown fences, no text before or after.
Each element must use exactly these fields:

[
  {
    "type": "...",
    "title": "...",
    "what_happened": "...",
    "what_was_missing": "...",
    "proposed_change": "...",
    "rationale": "...",
    "confidence": "high | medium | low",
    "confidence_rationale": "...",
    "evidence_references": ["...", "..."]
  }
]

evidence_references must be a JSON array (even if it contains only one item, not a string).
```

The field list matches `Proposal` exactly. The `evidence_references` type constraint addresses the most common observed failure mode.

**2. Extracted JSON in error message in `distiller.go`**

In `parseProposalsFromJSON`, when both parse attempts fail:

```go
// before
return nil, fmt.Errorf("failed to parse proposals from JSON: %w", err)

// after
return nil, fmt.Errorf("failed to parse proposals from JSON: %w; extracted: %s", err, jsonStr)
```

No behavioral change. The error is still returned to the caller and logged as non-blocking.

**Files in scope:**
- `internal/next/reviewdistiller/prompts.go` (modified)
- `internal/next/reviewdistiller/prompts_test.go` (modified)
- `internal/next/reviewdistiller/distiller.go` (modified)
- `internal/next/reviewdistiller/distiller_test.go` (modified)

All other files are out of scope. Rework tasks must not touch them.

## Acceptance Criteria

1. `buildReworkImplementationGapInstructions()` return value contains the string `"## Output Format"`.
2. `buildAcceptedInstructions()` return value contains the string `"## Output Format"`.
3. `buildReworkVisionChangeInstructions()` return value contains the string `"## Output Format"`.
4. All three output format sections contain the string `"evidence_references"` followed on the same line or the next by `[` — constraining it as a JSON array, not a string.
5. When `parseProposalsFromJSON` is called with input that fails both parse attempts, the returned error message contains the extracted JSON string that was attempted.
6. All existing tests in `internal/next/reviewdistiller/...` continue to pass.

## Scenarios

### Scenario: Prompt includes output format for rework_implementation_gap outcome
**Given:** A `DistillerInputs` with a valid `ReviewOutcome` and outcome type `"rework_implementation_gap"`
**When:** `BuildPrompt` is called
**Then:** The returned prompt string contains a section starting with `## Output Format`, which includes the text `evidence_references` and a JSON array indicator (`[`) showing that field must be an array
**Notes:** Test via `prompts_test.go`. No LLM call needed — purely testing prompt construction.

### Scenario: Same output format appears for accepted and rework_vision_change outcomes
**Given:** `BuildPrompt` called with outcome `"accepted"`, then separately with `"rework_vision_change"`
**When:** Each prompt is inspected
**Then:** Both contain `## Output Format` with `evidence_references` array syntax
**Notes:** Separate assertions per outcome type.

### Scenario: Parse failure error includes the extracted JSON
**Given:** `parseProposalsFromJSON` is called with a string that is valid JSON but cannot be unmarshaled into `[]Proposal` or `struct{Proposals []Proposal}` (e.g., `[{"evidence_references": "not-an-array"}]`)
**When:** Both parse attempts fail
**Then:** The returned error message contains the input string so the caller's log shows exactly what the LLM returned
**Notes:** Test via `distiller_test.go`. Assert `strings.Contains(err.Error(), <the input string>)`.

## Validation

```bash
go test ./internal/next/reviewdistiller/...
go vet ./internal/next/reviewdistiller/...
```
