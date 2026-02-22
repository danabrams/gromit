---
id: tdd-cycle-granularity
source_ideas: []
created: 2026-02-22
supersedes_decisions:
  - tdd-fresh-context-per-cycle#6
---

# TDD Cycle Granularity

## Specification

The fresh-context TDD orchestrator runs separate RED and GREEN Claude invocations per cycle, but in practice most cycles collapse into a single RED (write all tests) followed by a single GREEN (write all implementation). This defeats the incremental benefit of TDD — each cycle should address one discrete requirement, not the entire bead.

The root cause: beads created by decomposition have empty `expected_outputs`. When `expected_outputs` is empty, `tddExpectedOutputsOrTitle()` falls back to the bead title as a single requirement string. `resolveInitialCycleState()` then creates a `Remaining` slice with one element — the entire bead title. The orchestrator's `RunCycles` loop sees one requirement, runs one RED/GREEN cycle, and stops. Claude writes everything in that one cycle because the prompt says "implement: [entire bead title]."

Investigation report: `.gromit/reports/debug-20260222-050000.md`

The fix is three layers, each independently useful and each building on the previous. Layer 1 fixes the upstream source (decomposition). Layer 2 adds a local fallback for beads that reach the runner without structured requirements. Layer 3 adds an LLM-based extraction for beads whose descriptions resist pattern matching.

### Layer 1: Populate expected_outputs During Decomposition

The decompose pipeline creates beads via `pipeline.go:295` and `review/review.go:96`. Currently, `ExpectedOutputs` is either empty or populated with file paths — neither of which represents discrete testable requirements.

Change the decompose prompt (`PROMPT_decompose.md`) to instruct Claude to list 2-5 discrete, testable deliverables per bead. These are not file paths — they are behavioral requirements that can each be the subject of one RED/GREEN cycle. Examples:

- "Parse bulleted lists from description into individual items"
- "Return empty slice when input has no list markers"
- "Handle mixed bullet styles (-, *, numbered) in one description"

The decompose prompt already asks for acceptance criteria. The change is to also populate `expected_outputs` with the individual deliverables, distinct from the prose acceptance criteria. Acceptance criteria describe the overall definition of done; expected outputs list the incremental steps to get there.

Update the bead creation calls in `pipeline.go:295` and `review/review.go:96` to pass these deliverables as `ExpectedOutputs` instead of falling through to the `expectedOutputsOrTitle()` helper that collapses them to the title.

This is the upstream fix. Every methodology consumer — TDD, ATDD, coverage tracking — benefits from beads that carry structured requirements.

### Layer 2: Parse Bead Description as Fallback

If `ExpectedOutputs` is empty at TDD time (manually created beads, legacy beads, or a decompose run before Layer 1 is deployed), parse the bead's `Description` field to extract individual requirements.

Add a new function `extractRequirementsFromDescription(description string) []string` in `internal/runner/process_methodology.go`, near the existing `tddExpectedOutputsOrTitle()` function. This function looks for structured content patterns:

**Patterns to match (in priority order):**

1. **Numbered lists** — lines starting with `1.`, `2.`, etc.
2. **Bulleted lists** — lines starting with `-`, `*`, or `+` followed by a space
3. **"Includes:" / "Delivers:" / "Requirements:" headers** — extract the items that follow the header, whether comma-separated on the same line or as a list below
4. **Semicolon-separated items** — a single sentence with `;` separating distinct deliverables

**Rules:**

- Return `nil` if fewer than 2 items are extracted (a single item is no better than the title fallback)
- Strip list markers and leading/trailing whitespace from each item
- Skip blank lines and lines that are clearly headers (all caps, end with `:`)
- Maximum 10 items — if more are extracted, return the first 10

Update `tddExpectedOutputsOrTitle()` to try this extraction before falling back to the title:

```
func tddExpectedOutputsOrTitle(b *bead.Bead) []string {
    if len(b.ExpectedOutputs) > 0 {
        return b.ExpectedOutputs          // Layer 1: structured from decompose
    }
    if items := extractRequirementsFromDescription(b.Description); len(items) > 0 {
        return items                       // Layer 2: parsed from description
    }
    return []string{b.Title}              // Original fallback
}
```

This is a pure Go function with no LLM calls. It handles the common case where humans write descriptions with bullet points or numbered steps.

### Layer 3: LLM-Based Extraction as Last Resort

If Layer 2's pattern matching returns `nil` (unstructured prose, no lists, no separators), use a lightweight Claude call to decompose the bead title + description into discrete testable requirements.

Add a new function `extractRequirementsViaLLM(ctx context.Context, title, description string, invokeFn) ([]string, error)` in `internal/runner/process_methodology.go`. This function:

1. Builds a short prompt: "Given this task title and description, list 2-5 discrete, independently testable requirements. Output one requirement per line, no numbering, no bullets."
2. Invokes Claude at the haiku tier (cheapest, fastest)
3. Parses the response: split on newlines, trim whitespace, skip empty lines
4. Returns the extracted requirements, or falls back to `[]string{title}` if the call fails or returns fewer than 2 items

**Constraints:**

- Haiku tier only — this is a classification/parsing task, not a creative one
- Prompt must be under 500 tokens (title + description + instructions)
- If the bead description exceeds 2000 characters, truncate it before sending
- Timeout: 30 seconds — if it takes longer, fall back to the title
- This invocation runs before the first RED cycle, not inside the cycle loop

Update the TDD fresh-context entry point in `process_methodology.go` (around line 80-90) to call this extraction when the first two layers produce only a single-item result:

```
effectiveOutputs := tddExpectedOutputsOrTitle(bc.Bead)    // Layers 1 & 2
if len(effectiveOutputs) <= 1 {
    if extracted, err := extractRequirementsViaLLM(ctx, bc.Bead.Title, bc.Bead.Description, invokeFn); err == nil && len(extracted) > 1 {
        effectiveOutputs = extracted                        // Layer 3
    }
}
```

### How the Three Layers Interact

```
Bead arrives at TDD runner
    |
    v
Layer 1: ExpectedOutputs populated?  ---yes--> Use ExpectedOutputs (2-5 items)
    |no
    v
Layer 2: Description has lists?  ---yes--> Use parsed items (2-10 items)
    |no
    v
Layer 3: LLM extraction  ---success--> Use extracted items (2-5 items)
    |failure
    v
Title fallback (1 item = 1 big cycle, current behavior)
```

Each layer is a refinement. Layer 1 alone solves the problem for all newly decomposed beads. Layer 2 catches manually created beads with structured descriptions. Layer 3 handles the remaining edge cases. If all three fail, behavior is unchanged from today.

### Observability

Add a log line when each layer activates, so debugging can identify which extraction path was used:

- `"TDD cycle granularity: using %d expected_outputs from decomposition"` (Layer 1)
- `"TDD cycle granularity: parsed %d requirements from description"` (Layer 2)
- `"TDD cycle granularity: extracted %d requirements via LLM"` (Layer 3)
- `"TDD cycle granularity: title fallback (single cycle)"` (no extraction succeeded)

## Acceptance Criteria

### Layer 1: Decomposition

- Decompose prompt instructs Claude to list 2-5 discrete testable deliverables per bead in the `expected_outputs` field
- Bead creation in `pipeline.go` and `review/review.go` passes these deliverables as `ExpectedOutputs`
- Beads created by decomposition have 2-5 items in `ExpectedOutputs`, not file paths, not the title repeated
- Unit tests verify that decompose output parsing populates `ExpectedOutputs` with behavioral requirements

### Layer 2: Description Parsing

- `extractRequirementsFromDescription()` extracts items from numbered lists, bulleted lists, header-prefixed lists, and semicolon-separated items
- Returns `nil` when fewer than 2 items are found
- Returns at most 10 items
- `tddExpectedOutputsOrTitle()` tries description parsing before title fallback
- Table-driven unit tests cover each pattern (numbered, bulleted, header, semicolon) and edge cases (empty description, single item, more than 10 items, mixed formats)

### Layer 3: LLM Extraction

- `extractRequirementsViaLLM()` sends a short prompt to haiku and parses the response into requirements
- Uses haiku tier only
- Times out after 30 seconds, falling back to title
- Truncates descriptions over 2000 characters
- Returns `nil` on error or fewer than 2 extracted items
- Called only when Layers 1 and 2 produce a single-item result
- Runs before the first RED cycle, not inside the cycle loop
- Unit tests verify prompt construction, response parsing, timeout handling, and fallback behavior (using a fake invocation function)

### Integration

- A bead with populated `ExpectedOutputs` produces one TDD cycle per requirement (Layer 1 path)
- A bead with empty `ExpectedOutputs` but a bulleted description produces one TDD cycle per bullet (Layer 2 path)
- A bead with empty `ExpectedOutputs` and an unstructured description produces multiple TDD cycles via LLM extraction (Layer 3 path)
- A bead with empty everything still works (title fallback, single cycle — no regression)
- Log output identifies which layer was used

## Decisions

1. **Three layers, not one.** A single LLM-based extraction (Layer 3 only) would work for all cases but adds latency and cost to every TDD bead, even those that already have structured requirements. The layered approach uses the cheapest signal first (data already present), then pattern matching (free, fast), then LLM (cheap but not free). Most beads should be handled by Layer 1 after deployment.

2. **Expected outputs are behavioral requirements, not file paths.** The `ExpectedOutputs` field currently sometimes contains file paths (e.g., `"internal/foo/bar.go"`). For TDD cycle granularity, each output should be a testable behavior (e.g., "Parse bulleted lists from description"). File paths are not testable requirements. This is a semantic change to how decomposition populates the field.

3. **Description parsing lives in `internal/runner/`, not `internal/bead/`.** The `bead` package is a data model — it should not contain extraction heuristics. The parsing function lives near the TDD orchestration code that consumes it, in `process_methodology.go`. If other consumers need it later, it can be extracted to a shared package.

4. **LLM extraction uses haiku, not the bead's assigned tier.** This is a parsing task — decomposing prose into a list. It does not require the creativity or reasoning of sonnet/opus. Haiku is fast (~2-3 seconds) and cheap (~$0.001 per call). Using the bead's assigned tier would waste tokens on a trivial task.

5. **Maximum 10 requirements from parsing, 5 from LLM.** Description parsing caps at 10 to avoid pathological cases (a description with 50 bullet points). LLM extraction caps at 5 because the prompt asks for 2-5 and more would produce cycles too fine-grained. Both caps prevent runaway cycle counts.

6. **Layer 1 changes the decompose prompt, not the decompose Go code.** The decomposition pipeline already passes `ExpectedOutputs` through to bead creation. The issue is that Claude does not populate the field during decomposition because the prompt does not ask for it. Changing the prompt is sufficient — no pipeline code changes needed for Layer 1 beyond ensuring the field flows through correctly.

## Research & Context

### Current State

The TDD fresh-context orchestrator (`tdd-fresh-context-per-cycle` spec, implemented Feb 19-20) runs separate Claude invocations per RED/GREEN/REFACTOR phase. The cycle loop in `orchestrator.go:108-115` iterates over `CycleState.Remaining`, processing one requirement per cycle.

The problem is upstream: `Remaining` is populated from `ExpectedOutputs`, and `ExpectedOutputs` is almost always empty for beads created by decomposition. The fallback in `tddExpectedOutputsOrTitle()` (`process_methodology.go:226-238`) collapses to the bead title — one requirement, one cycle.

### Key Code Paths

| Location | Role | Change |
|---|---|---|
| `internal/runner/process_methodology.go:226-238` | `tddExpectedOutputsOrTitle()` — title fallback | Add Layer 2 (description parsing) before title fallback |
| `internal/runner/process_methodology.go:80-90` | TDD entry point — sets `ExpectedOutputs` | Add Layer 3 (LLM extraction) when Layers 1-2 produce single item |
| `internal/runner/callbacks_tdd.go:265-283` | `resolveInitialCycleState()` — creates `Remaining` | No change — correctly consumes whatever `ExpectedOutputs` provides |
| `internal/runner/tdd/orchestrator.go:108-115` | `RunCycles` — iterates over `Remaining` | No change — correctly loops over available requirements |
| `cmd/gromit/decompose.go` | Decompose command entry point | No Go code changes (Layer 1 is prompt-only) |
| `internal/pipeline/pipeline.go:295` | Bead creation during decompose | Verify `ExpectedOutputs` flows through |
| `.gromit/templates/PROMPT_decompose.md` | Decompose prompt template | Layer 1: add expected_outputs instructions |

### Related Specs

- **tdd-fresh-context-per-cycle** — Defines the per-phase invocation model and cycle loop that this spec feeds into. That spec assumes `ExpectedOutputs` is populated; this spec ensures it is.
- **decomposition-granularity** — Changed bead sizing from file-count to behavioral units. This spec extends that philosophy: each behavioral unit should be further decomposed into testable requirements for TDD cycles.
- **decompose-overlap-guard** — Prevents sibling beads from overlapping. Complementary — finer-grained expected outputs within a bead do not affect inter-bead overlap.
- **tdd-methodology** — The original TDD spec. Decision #6 (prescriptive prompt, not mechanically enforced cycles) was superseded by `tdd-fresh-context-per-cycle`. This spec does not change the cycle mechanics — it changes what feeds into them.
