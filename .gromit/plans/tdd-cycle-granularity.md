---
id: tdd-cycle-granularity
source_spec: tdd-cycle-granularity
created: 2026-02-22
decomposed: false
---

# TDD Cycle Granularity Implementation Plan

**Goal:** Ensure TDD beads produce one RED/GREEN cycle per discrete requirement instead of collapsing to a single cycle per bead.

**Architecture:** Three extraction layers — each independently useful — populate `ExpectedOutputs` with 2-10 discrete requirements. Layer 1 fixes upstream (decompose prompt), Layer 2 adds local pattern-matching on bead descriptions, Layer 3 uses a haiku LLM call as last resort.

**Tech Stack:** Go, prompt templates (Go text/template)

**Spec:** `.gromit/specs/tdd-cycle-granularity.md`

---

## Architecture

**Overview:**
The root cause is that `tddExpectedOutputsOrTitle()` falls back to the bead title when `ExpectedOutputs` is empty, producing a single-element `Remaining` slice and thus one TDD cycle. Three layers ensure beads carry structured requirements before reaching the cycle loop.

**Key Components:**

1. **Description Parser (Layer 2)**: New `extractRequirementsFromDescription()` pure Go function in `process_methodology.go`. Matches numbered lists, bulleted lists, header-prefixed lists, and semicolon-separated items. Returns nil if fewer than 2 items found, caps at 10.

2. **LLM Extraction (Layer 3)**: New `extractRequirementsViaLLM()` in `process_methodology.go`. Sends a short prompt to haiku tier, parses newline-separated response. Called only when Layers 1-2 produce <=1 item. 30-second timeout, 2000-char description truncation.

3. **Decompose Prompt (Layer 1)**: Updated `PROMPT_decompose.md` instructs Claude to include `expected_outputs` array in JSON output — 2-5 discrete, testable behavioral requirements per bead. New `ExpectedOutputs` field on `beadDef` struct in `decompose.go`.

**Integration Points:**

- `tddExpectedOutputsOrTitle()` (process_methodology.go:226-238) — modified to try Layer 2 before title fallback
- `runTDDFreshContextCycles()` (process_methodology.go:80) — Layer 3 call added when effectiveOutputs has <=1 item
- `resolveInitialCycleState()` (callbacks_tdd.go:265) — no change, correctly consumes ExpectedOutputs
- `RunCycles` (orchestrator.go:107) — no change, correctly iterates Remaining
- `beadDef` (decompose.go:29) — add ExpectedOutputs field
- `Decompose()` (decompose.go:170) — pass ExpectedOutputs if populated

**Data Flow:**

```
Bead arrives at runTDDFreshContextCycles()
  → tddExpectedOutputsOrTitle(bead)
    → Layer 1: bead.ExpectedOutputs populated?  → use them (2-5 items)
    → Layer 2: extractRequirementsFromDescription(bead.Description) → parsed items (2-10)
    → Fallback: bead.Title as single item
  → Layer 3: if effectiveOutputs <= 1, extractRequirementsViaLLM()
    → Success with 2+ items → replace effectiveOutputs
    → Failure → keep title fallback (no regression)
  → bc.Bead.ExpectedOutputs = effectiveOutputs
  → RunCycles iterates one RED/GREEN cycle per item
```

**Files to Modify:**

- `internal/runner/process_methodology.go` — Layers 2 and 3 functions, updated fallback, entry point wiring, observability logs
- `internal/pipeline/decompose.go` — `beadDef` struct update, bead creation update
- `.gromit/templates/PROMPT_decompose.md` — Layer 1 prompt instructions

**Files to Create:**

- `internal/runner/process_methodology_test.go` — tests for Layers 2 and 3

**Tradeoffs:**

- **Layered vs single LLM**: Layered avoids haiku cost/latency when structured data already exists. Most beads should be handled by Layer 1 after deployment.
- **Parser in runner, not bead**: `bead` is a data model. Extraction heuristics belong near the TDD consumer.
- **Haiku for Layer 3**: Parsing task, not creative. Fast (~2-3s) and cheap (~$0.001).
- **No downstream changes**: Orchestrator and cycle state machinery already handle any-length Remaining slice.

## Test Strategy

**Unit Tests (Layer 2 — description parser):**
- Table-driven tests for `extractRequirementsFromDescription()` covering each pattern type
- Pure function, no mocks needed

**Unit Tests (Layer 3 — LLM extraction):**
- `extractRequirementsViaLLM()` with a fake invoke function closure
- Tests prompt construction, response parsing, timeout, fallback behavior

**Unit Tests (layer integration):**
- Updated `tddExpectedOutputsOrTitle()` tests verifying Layer 1 → Layer 2 → fallback priority
- Entry point tests verifying Layer 3 triggers only when Layers 1-2 produce <=1 item

**Key Test Cases:**

Layer 2:
- Numbered list → extracts items
- Bulleted list (-, *, +) → extracts items
- Header-prefixed list (Requirements:, Includes:, Delivers:) → extracts items after header
- Semicolon-separated → splits on semicolons
- Empty description → nil
- Single item only → nil
- More than 10 items → first 10
- Header-only lines → skipped
- Blank lines → skipped

Layer 3:
- Normal response (3 lines) → 3 requirements
- Blank lines in response → skipped
- Fewer than 2 items → nil
- Invoke error → nil
- Description over 2000 chars → truncated
- Prompt uses TierLow (haiku)

Integration:
- Bead with populated ExpectedOutputs → Layer 1 (no parsing)
- Bead with empty ExpectedOutputs + bulleted description → Layer 2
- Bead with empty ExpectedOutputs + unstructured description → Layer 3
- Bead with empty everything → title fallback (no regression)
- Log output identifies which layer was used

**Mocking:** Fake invoke function closure for Layer 3. No mocks needed for Layer 2. Constructed `bead.Bead` structs for integration tests.

**Test Organization:** All tests in `internal/runner/process_methodology_test.go`. Table-driven format for Layer 2 patterns. Subtests for Layer 3 scenarios.

## Implementation Tasks

### Task 1: Layer 2 — Description parser and fallback update

**Files:**
- Modify: `internal/runner/process_methodology.go`
- Create: `internal/runner/process_methodology_test.go`

**What to Do:**

Add `extractRequirementsFromDescription(description string) []string` near the existing `tddExpectedOutputsOrTitle()` function (around line 238). This function matches structured content patterns in priority order:

1. Numbered lists — lines starting with `1.`, `2.`, etc.
2. Bulleted lists — lines starting with `-`, `*`, or `+` followed by a space
3. Header-prefixed lists — lines with `Includes:`, `Delivers:`, `Requirements:` headers, extracting items that follow (comma-separated on same line or as a list below)
4. Semicolon-separated items — a single sentence with `;` separating distinct deliverables

Rules: return nil if <2 items, strip markers and whitespace, skip blank lines and header-only lines (all caps or ending with `:`), cap at 10 items.

Update `tddExpectedOutputsOrTitle()` to try description parsing before title fallback:

```go
func tddExpectedOutputsOrTitle(b *bead.Bead) []string {
    if b == nil {
        return []string{}
    }
    if len(b.ExpectedOutputs) > 0 {
        return append([]string(nil), b.ExpectedOutputs...)
    }
    if items := extractRequirementsFromDescription(b.Description); len(items) > 0 {
        return items
    }
    trimmedTitle := strings.TrimSpace(b.Title)
    if trimmedTitle == "" {
        return []string{}
    }
    return []string{trimmedTitle}
}
```

Add observability log lines in `runTDDFreshContextCycles()` identifying which layer was used (Layer 1 decomposition, Layer 2 parsed, or title fallback).

**Expected Outputs:**
- Extract items from numbered lists (lines starting with `1.`, `2.`, etc.)
- Extract items from bulleted lists (lines starting with `-`, `*`, or `+` followed by space)
- Extract items from header-prefixed content (`Requirements:`, `Includes:`, `Delivers:` followed by list or comma-separated items)
- Extract items from semicolon-separated strings
- Return nil when fewer than 2 items found, and cap at 10 items
- `tddExpectedOutputsOrTitle()` tries Layer 2 parsing before title fallback

**Acceptance Criteria:**
- `extractRequirementsFromDescription()` extracts items from numbered, bulleted, header-prefixed, and semicolon-separated patterns
- Returns nil for empty input, single items, and unstructured prose
- Caps output at 10 items
- `tddExpectedOutputsOrTitle()` tries description parsing before title fallback
- Table-driven tests cover each pattern and edge case

**Dependencies:** None

**Notes:** This is a pure Go function with no external dependencies. The function lives in `process_methodology.go` near the existing fallback function it enhances. The bead's `Description` field is always available — populated either from decompose or from manual bead creation.

### Task 2: Layer 3 — LLM extraction and entry point wiring

**Files:**
- Modify: `internal/runner/process_methodology.go`
- Modify: `internal/runner/process_methodology_test.go`

**What to Do:**

Add `extractRequirementsViaLLM(ctx context.Context, title, description string, invokeFn func(ctx context.Context, prompt, tier string) error) ([]string, error)` in `process_methodology.go`. This function:

1. Builds a short prompt: "Given this task title and description, list 2-5 discrete, independently testable requirements. Output one requirement per line, no numbering, no bullets."
2. Truncates description to 2000 characters if longer
3. Invokes via the provided `invokeFn` at `provider.TierLow` (haiku)
4. The invoke function needs to capture the LLM output — use the existing TDD `InvokeFn` pattern from `tdd/orchestrator.go` or create a lightweight closure that captures the response
5. Parses the response: split on newlines, trim whitespace, skip empty lines
6. Returns the extracted requirements, or nil if <2 items or on error
7. Applies a 30-second context timeout

Wire into `runTDDFreshContextCycles()` after the `tddExpectedOutputsOrTitle()` call (around line 80-88):

```go
effectiveOutputs := tddExpectedOutputsOrTitle(bc.Bead)
if len(effectiveOutputs) <= 1 {
    if extracted, err := extractRequirementsViaLLM(ctx, bc.Bead.Title, bc.Bead.Description, invokeFn); err == nil && len(extracted) > 1 {
        effectiveOutputs = extracted
    }
}
```

The `invokeFn` parameter requires access to the runner's invoke machinery. The TDD orchestrator's `InvokeFn` (type `func(ctx context.Context, prompt, tier string) error` from `tdd/orchestrator.go:18`) is the right signature, but it captures output in `bc.Result`. For Layer 3, we need the raw text output. Options:
- Use `r.makeInvokeFn()` with a temporary BeadContext and read `bc.Result.Output` after invocation
- Create a simpler closure that calls the router directly at TierLow

The simpler approach: build a closure in `runTDDFreshContextCycles` that invokes at TierLow and returns the output text, passing that to `extractRequirementsViaLLM`.

Add observability log line when Layer 3 activates.

**Expected Outputs:**
- Build prompt from title + description and invoke at TierLow (haiku)
- Parse newline-separated LLM response into requirement strings, skipping blank lines
- Truncate descriptions over 2000 characters before sending to LLM
- Return nil on invoke error or when fewer than 2 items extracted
- Wire into `runTDDFreshContextCycles()` so Layer 3 runs only when Layers 1-2 produce <=1 item

**Acceptance Criteria:**
- `extractRequirementsViaLLM()` sends a short prompt to haiku and parses the response
- Uses `provider.TierLow` only
- Truncates descriptions over 2000 characters
- Returns nil on error or <2 extracted items
- Called only when Layers 1-2 produce <=1 item
- Runs before the first RED cycle, not inside the cycle loop
- Tests verify prompt construction, response parsing, timeout handling, and fallback behavior using a fake invoke function

**Dependencies:** Task 1 (needs updated `tddExpectedOutputsOrTitle()`)

**Notes:** The main design decision is how to invoke haiku. The TDD orchestrator's `InvokeFn` type captures output via side effects on BeadContext. For Layer 3 we need direct access to the text output. A lightweight closure that calls the execution/invocation path and returns the output string is cleaner than reusing the side-effect-based InvokeFn. Look at how `r.makeInvokeFn()` in `callbacks.go:78-150` builds its closure for reference.

### Task 3: Layer 1 — Decompose prompt and beadDef struct update

**Files:**
- Modify: `.gromit/templates/PROMPT_decompose.md`
- Modify: `internal/pipeline/decompose.go`
- Modify: `internal/pipeline/decompose_test.go`

**What to Do:**

**Prompt update:** Add instructions to `PROMPT_decompose.md` for Claude to include an `expected_outputs` array in each bead's JSON output. These are 2-5 discrete, independently testable behavioral requirements — not file paths, not the title repeated. They represent the incremental steps to satisfy the acceptance criteria. Add `expected_outputs` to the JSON example format:

```json
{
  "title": "...",
  "description": "...",
  "depends_on_index": [],
  "acceptance_criteria": ["overall done criterion 1", "overall done criterion 2"],
  "expected_outputs": [
    "Parse bulleted lists from description into individual items",
    "Return empty slice when input has no list markers",
    "Handle mixed bullet styles in one description"
  ]
}
```

Add guidance distinguishing the two fields: acceptance_criteria define overall definition of done; expected_outputs list the incremental testable steps to get there (each becomes one TDD RED/GREEN cycle).

**Struct update:** Add `ExpectedOutputs []string` field with `json:"expected_outputs,omitempty"` tag to `beadDef` struct in `decompose.go:29-36`.

**Bead creation update:** In `decompose.go:170`, change the 4th argument to `CreateWithDepsAndDescription` from `def.AcceptanceCriteria` to `def.ExpectedOutputs` when non-empty, falling back to `def.AcceptanceCriteria`:

```go
outputs := def.ExpectedOutputs
if len(outputs) == 0 {
    outputs = def.AcceptanceCriteria
}
beadResult, err := p.deps.BeadClient.CreateWithDepsAndDescription(
    def.Title, priority, labels, outputs, dependencies, def.Description,
)
```

**Expected Outputs:**
- Add `expected_outputs` instructions and JSON example to `PROMPT_decompose.md`
- Add `ExpectedOutputs []string` field with JSON tag to `beadDef` struct
- Pass `def.ExpectedOutputs` to bead creation when populated, fall back to `def.AcceptanceCriteria`
- Deserialize `expected_outputs` from JSON output correctly in decompose parsing

**Acceptance Criteria:**
- Decompose prompt instructs Claude to include 2-5 discrete testable deliverables in `expected_outputs`
- `beadDef` struct has `ExpectedOutputs` field with JSON tags
- Bead creation passes ExpectedOutputs when populated, falls back to AcceptanceCriteria
- JSON parsing test verifies `expected_outputs` field is correctly deserialized

**Dependencies:** None (independent of Tasks 1-2)

**Notes:** This task is primarily a prompt change. The Go code changes are minimal — one struct field and a conditional in bead creation. The existing `resolveExpectedOutputsFromAcceptanceCriteria()` in `bead.go` will continue to work for legacy beads that only have acceptance_criteria. The decompose prompt template at `PROMPT_decompose.md` is the canonical template — the hardcoded `decomposePromptTemplate` in `decompose.go:203-221` wraps the plan content and skill content, not the per-bead instructions.

---

## Notes

- **No downstream changes needed.** The orchestrator (`RunCycles`), cycle state (`resolveInitialCycleState`), and cycle advancement (`AssembleCycleState`) all correctly handle any-length `Remaining` slice. The fix is entirely about what feeds into `ExpectedOutputs`.
- **Existing `resolveExpectedOutputsFromAcceptanceCriteria()`** in `bead.go:88-102` maps acceptance_criteria lines into ExpectedOutputs for legacy beads. This continues to work — Layer 2 only activates when ExpectedOutputs is empty after all bead-level resolution.
- **The `beadDef` struct currently passes `AcceptanceCriteria` as expected outputs** at `decompose.go:170-174`. Task 3 separates these concepts by adding a dedicated `ExpectedOutputs` field.
- **Layer 3 invoke pattern:** The TDD `InvokeFn` type (`func(ctx, prompt, tier string) error`) captures output via side effects. For Layer 3, consider a simpler closure that returns `(string, error)` to get the raw text output directly.
