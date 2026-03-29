# Spec 0004p — Criterion Evaluation Structured Output

## spec_id
0004p-criterion-eval-structured-output

## Vision

The criterion evaluator calls an LLM to judge pass/fail per acceptance criterion. When the LLM returns prose instead of JSON (observed in 1/10 production runs — run 143039 returned `## Criterion 1 Assessment: PASS` instead of a JSON object), the `llmadapter.ExtractJSON` helper finds no `{` bracket, returns the raw text, `json.Unmarshal` fails, and `ParseCriterionResult` raises a `ParseError` that kills the run. A clear "PASS" in prose shouldn't be a fatal error.

## Summary

Enforce structured JSON output at the API level for criterion evaluation, and add a fallback prose parser in `ParseCriterionResult` that extracts pass/fail signals from freeform LLM responses when JSON parsing fails. When both structured parsing and the prose fallback fail, include the raw LLM output in the error message so failures are self-describing.

## Goals

### Primary
- Criterion evaluation LLM calls request structured JSON output at the API level
- When JSON parsing fails, a fallback parser extracts pass/fail/unclear from prose before raising an error
- When both parsers fail, the error message includes the raw LLM output for diagnostics

### Secondary
- The fallback parser produces a `CriterionResult` with a rationale extracted from the prose when possible
- Structured output and fallback are transparent to the `AcceptStage` — no changes needed at the stage layer

## Non-goals
- Per-criterion timeout scaling (deferred to 0004p2)
- Delta-diff evaluation to reduce context size (deferred to 0004r)
- Changing the evaluation criteria format or the `CriterionResult` struct
- Modifying the `Invoker` interface — structured output is achieved by prompt-level or provider-level configuration within the acceptor package

## Architecture

All changes are confined to `internal/next/acceptor/`.

### 1. Structured output enforcement in `prompt.go`

`ProviderAcceptAgent.EvaluateCriterion` currently calls `a.invoker.Invoke(ctx, prompt)` with no output format constraint. The prompt in `prompt.go` says "Respond with a JSON object" but the LLM is free to ignore this.

Add a system-level instruction or use the invoker's JSON mode to constrain output. The exact mechanism depends on what `llmadapter.Invoker` supports:
- **Option A (prompt reinforcement):** Wrap the prompt with a system preamble that demands JSON-only output and append a closing instruction: `"Return ONLY the JSON object. No markdown, no prose, no text before or after the JSON."` This is the minimal change.
- **Option B (provider JSON mode):** If the invoker supports a `WithJSONMode` option or similar, use it. This is stronger but requires checking provider capabilities.

The implementer should start with Option A (prompt reinforcement in `prompt.go`) and upgrade to Option B if the invoker already supports JSON mode.

**File:** `internal/next/acceptor/prompt.go` — append explicit JSON-only constraint to `acceptancePromptTmpl`.

### 2. Fallback prose parser in `provider_agent.go`

In `ParseCriterionResult`, when `json.Unmarshal` fails on the extracted text, invoke a new `parseCriterionFromProse(output string) (CriterionResult, error)` function. If it succeeds, return the result immediately (skipping the JSON-path field validations). If it also fails, fall through to the error return.

`parseCriterionFromProse` logic:
1. Case-insensitive scan for status signal keywords:
   - Pass signals: `"PASS"`, `"passed"`, `"criterion is met"`, `"satisfied"`, `"Assessment: Pass"`
   - Fail signals: `"FAIL"`, `"failed"`, `"criterion is not met"`, `"not satisfied"`, `"Assessment: Fail"`
   - Unclear signals: `"UNCLEAR"`, `"insufficient evidence"`, `"cannot determine"`, `"Assessment: Unclear"`
2. If exactly one status class matches, construct a `CriterionResult` with:
   - `Status`: the matched status
   - `Rationale`: the full prose output (trimmed to first 500 chars)
   - `Criterion`: `"(parsed from prose)"` — a non-empty placeholder so the existing empty-`Criterion` validation in `ParseCriterionResult` does not reject the result (the caller in `evaluator.go` overwrites this field anyway)
   - `EvidenceRefs`: empty slice
3. If zero or multiple conflicting status classes match, return an error (ambiguous prose).

**File:** `internal/next/acceptor/provider_agent.go` — add `parseCriterionFromProse` and call it from `ParseCriterionResult`.

### 3. Raw output in error diagnostic in `provider_agent.go`

When both JSON parsing and prose fallback fail, include the raw LLM output in the `ParseError`:

```go
// current
return CriterionResult{}, &ParseError{Msg: "parsing criterion result: " + err.Error()}

// new
return CriterionResult{}, &ParseError{
    Msg: fmt.Sprintf("parsing criterion result: %s; raw output: %.500s", err.Error(), output),
}
```

This truncates the raw output to 500 chars to avoid log bloat while providing enough signal to diagnose the format.

**File:** `internal/next/acceptor/provider_agent.go` — modify the error return in `ParseCriterionResult`.

### 4. Files in scope

- `internal/next/acceptor/prompt.go` (modified — JSON-only output constraint)
- `internal/next/acceptor/prompt_test.go` (modified — assert constraint present)
- `internal/next/acceptor/provider_agent.go` (modified — fallback parser + error diagnostic)
- `internal/next/acceptor/provider_agent_test.go` (modified — fallback parser tests + error message tests)

All other files are out of scope. Rework tasks must not touch them.

## Acceptance Criteria

1. The rendered acceptance prompt (from `RenderAcceptancePrompt`) contains the string `"Return ONLY the JSON object"` (or equivalent JSON-only constraint), ensuring the LLM is explicitly told not to return prose.

2. When `ParseCriterionResult` is called with a string that contains no JSON but includes a clear pass signal (e.g., `"## Assessment\n\nThe criterion is clearly PASS. The implementation satisfies all requirements."`), it returns a `CriterionResult` with `Status == "pass"` instead of an error.

3. When `ParseCriterionResult` is called with a string that contains no JSON but includes a clear fail signal (e.g., `"Criterion Assessment: FAIL — the feature is not implemented"`), it returns a `CriterionResult` with `Status == "fail"` and a non-empty `Rationale`.

4. When `ParseCriterionResult` is called with a string that contains no JSON and has ambiguous or no status signals (e.g., `"I'm not sure about this criterion"`), it returns a `ParseError` whose `.Error()` string contains a substring of the raw input.

5. When `ParseCriterionResult` is called with valid JSON that unmarshals into a valid `CriterionResult`, the fallback parser is NOT invoked — the existing JSON path is used unchanged.

6. When `ParseCriterionResult` is called with prose containing conflicting signals (both "PASS" and "FAIL"), it returns a `ParseError` (ambiguous — does not guess).

7. The `ParseError` returned when both parsers fail includes the raw LLM output (truncated to at most 500 characters) in the error message.

8. All existing tests in `internal/next/acceptor/...` continue to pass.

## Scenarios

### Scenario: LLM returns valid JSON — happy path unchanged
**Given:** The LLM returns `{"criterion": "AC1", "status": "pass", "rationale": "All tests pass", "evidence_refs": ["foo.go"]}`
**When:** `ParseCriterionResult` is called with this output
**Then:** Returns a `CriterionResult` with `Status == "pass"`, `Rationale == "All tests pass"`, `EvidenceRefs == ["foo.go"]`; the prose fallback is never reached
**Notes:** This is a regression guard — existing behavior must not change.

### Scenario: LLM returns prose with clear PASS signal
**Given:** The LLM returns `"## Criterion 1 Assessment: PASS\n\nThe implementation adds the required field and all tests confirm it works correctly."`
**When:** `ParseCriterionResult` is called with this output
**Then:** Returns `CriterionResult{Status: "pass", Rationale: "## Criterion 1 Assessment: PASS..."}` with no error; the run continues instead of dying on a parse failure
**Notes:** Reproduces the actual production failure from run 143039.

### Scenario: Prose with conflicting signals returns error with raw output
**Given:** The LLM returns `"The criterion PASSED for file A but FAILED for file B"`
**When:** `ParseCriterionResult` is called
**Then:** Returns a `ParseError` because both pass and fail signals are present; the error message includes (a substring of) the raw LLM output for debugging

### Scenario: End-to-end through ProviderAcceptAgent — prose fallback prevents evaluation failure
**Given:** A `ProviderAcceptAgent` backed by a mock `Invoker` that returns prose-only output (`"## Assessment: PASS\nAll requirements satisfied."`) for a criterion
**When:** `EvaluateCriterion` is called
**Then:** The call returns a `CriterionResult` with `Status == "pass"` and a non-empty `Rationale` instead of a `ParseError`; the prose fallback fires transparently within the provider agent
**Notes:** Test in `provider_agent_test.go`. The `AcceptStage` is out of scope — it delegates to `AcceptAgent` and is unaffected by the fallback.

## Validation

```bash
go test ./internal/next/acceptor/...
go test ./internal/next/specloop/stages/... -run TestAccept
go vet ./internal/next/acceptor/...
```
