---
id: reduce-iteration-cost
source_ideas: []
created: 2026-02-12
---

# Reduce Gromit Run Iteration Cost

## Specification

A gromit run iteration currently averages 22.5 minutes and $1.61 per bead, with 2.8 Claude invocations per bead. Validation retries are the dominant cost — 3 out of 5 beads in recent runs needed retry, adding ~10 minutes each. This spec addresses three independent levers to reduce iteration time and token spend.

### Lever A: Reduce Validation Retry Rate

**Problem:** Validation retries are the single biggest time cost. When the build phase produces code that fails `go test` or `go vet`, gromit launches an entirely new Claude invocation (with full context reload) to fix it. Recent runs show a 60% retry rate.

**Root causes to address:**

1. **The build prompt doesn't see recent validation failures from other beads.** If the last 3 beads all failed on the same lint issue, the current bead's build prompt has no way to know. The agent makes the same mistake and pays the retry cost again.

2. **The build phase doesn't self-validate before finishing.** The agent writes code, commits, and exits — then a separate validation invocation discovers the failure. If the build agent ran `go test` and `go vet` on its own changes before finishing, it could fix issues within the same context window at near-zero marginal cost (the context is already loaded).

**Changes:**

- **Inject recent validation failures into build prompts.** Add a "Recent Validation Issues" section to the build context that includes the last N validation failure summaries (error messages, not full output) from the current run. Source this from `IterationLog` entries where validation failed. Cap at the 3 most recent failures to avoid bloating the prompt. This gives the agent foreknowledge of pitfalls.

- **Add a self-check instruction to build templates.** Append guidance to `PROMPT_build.md` (and ATDD/TDD variants) instructing the agent to run the project's validation commands (`go test ./...`, `go vet ./...`) on the packages it modified before completing. This is advisory — the agent may or may not follow it — but it's free to add and catches issues within the existing invocation. The separate validation phase still runs as a safety net.

### Lever B: Trim Static Context Per Invocation

**Problem:** Each invocation loads ~22KB of static context (RULES.md 9.4KB, CLAUDE.md 5.3KB, learnings ~6KB, template ~1.8KB). While not the dominant cost, this compounds across 2.8 invocations per bead.

**Current state:**
- RULES.md (53 lines, 9.4KB): Injected into every prompt via template context. Contains code style, safety, test quality, and process rules — many are specific to writing production code and irrelevant for validation or review.
- CLAUDE.md (101 lines, 5.3KB): Loaded automatically by Claude Code on every invocation. Contains architecture overview, dev commands, conventions.
- Learnings (~6KB): Already scoped to build-type prompts only. 5 confirmed + up to 6 recent entries.
- Archived learnings (226 entries, ~62KB): Already excluded from prompts. Not a problem.

**Changes:**

- **Split RULES.md into phase-relevant subsets.** Create a rules categorization in the renderer: `build` rules (code style, safety, process), `validate` rules (just the validation commands), `review` rules (test quality, code style). The renderer selects the subset based on the phase being rendered. Implementation: add a `Phase` field to the render context and filter rules sections by phase. Rules are tagged with comments like `<!-- phases: build, review -->` or split into separate files (`RULES_build.md`, `RULES_validate.md`) — whichever is simpler to maintain. The full RULES.md remains the source of truth for humans.

- **Trim CLAUDE.md for non-build phases.** Validation and review invocations don't need the full architecture overview, bead sizing rules, or bd integration docs. Create a minimal CLAUDE.md variant (or instruct the renderer to set `--no-project-context` for phases that provide their own full context via the prompt file) so that non-build phases load less ambient context. Investigate whether Claude Code's `--prompt-file` flag can be combined with suppressing automatic CLAUDE.md loading.

### Lever C: Surface Learnings More Selectively

**Problem:** The learnings system is already well-scoped (only build prompts, only confirmed + 24h recent). But as the project matures, confirmed learnings will grow. Currently 5 entries at ~3.9KB is fine; at 20 entries it won't be.

**Changes:**

- **Add category-based filtering to learnings injection.** Each learning has a category (`patterns`, `conventions`, `gotchas`). When rendering a build prompt, the renderer can optionally filter to categories relevant to the packages being touched. Implementation: the `BuildContext` method already receives the bead details including which packages are mentioned in the title/description. Add an optional `categories` or `packages` filter to `GetConfirmed()` that uses keyword matching (not LLM calls) to select relevant learnings. This is a lightweight optimization that becomes more valuable as learnings accumulate.

- **Cap confirmed learnings at a token budget.** Add a `max_learning_chars` config field (default: 8000 chars, ~2K tokens). When confirmed learnings exceed this budget, include only the N most recently confirmed entries that fit. This prevents unbounded growth from ever becoming a problem, without requiring manual curation.

## Acceptance Criteria

### Lever A
- Build prompt context includes a "Recent Validation Issues" section populated from the current run's failed validation summaries (max 3)
- When no validation failures have occurred in the current run, the section is omitted
- Build templates (standard, ATDD, TDD) include self-check guidance instructing the agent to run validation commands before completing
- Validation retry rate measurably decreases in subsequent runs (measured via retro efficiency analysis)

### Lever B
- Non-build phases (validate, review, analyze) receive a reduced subset of RULES.md relevant to their phase
- The full RULES.md remains the single source of truth; phase subsets are derived, not manually maintained
- Non-build Claude invocations load less ambient project context than build invocations

### Lever C
- `GetConfirmed()` accepts an optional filter parameter for category or keyword-based selection
- A `max_learning_chars` config field caps the total size of injected learnings with a sensible default
- When learnings exceed the cap, the most recently confirmed entries are preferred

## Decisions

1. **Self-check is advisory, not enforced.** The build template tells the agent to run tests before finishing, but we don't verify it did. The separate validation phase is the enforcement mechanism. This avoids complex coordination between phases while still catching issues early in the common case.

2. **Validation failure summaries, not full output.** Injecting the full stderr of a failed `go test` run would be huge. Instead, extract the key failure lines (test name + error message) and cap total size. The goal is pattern recognition ("this project fails on unused imports"), not debugging context.

3. **Rules splitting by annotation, not separate files.** Maintaining multiple rules files invites drift. A single RULES.md with phase annotations (parsed by the renderer) keeps one source of truth. If annotations prove too noisy for human readability, we can split into files and concatenate for display.

4. **Learnings cap is character-based, not entry-based.** A cap of "max 10 entries" penalizes detailed learnings. A character budget treats all entries equally and directly maps to token cost.

5. **No LLM calls for filtering.** Learnings filtering uses keyword/category matching, not Claude calls. Adding an LLM call to reduce token spend would be counterproductive. Keep it mechanical.

## Research & Context

### Current Metrics (2026-02-12)
- Average iteration: 22.5 min, $1.61, 42 tool calls
- Validation retry rate: 60% (3/5 beads in recent run)
- Validation retry cost: +10 min per retry
- Static context: ~22KB per build invocation
- Claude invocations per bead: 2.8 average
- LEARNINGS.md: 1,088 lines total (5 confirmed, 6 provisional, 226 archived)
- RULES.md: 53 lines, 9.4KB
- CLAUDE.md: 101 lines, 5.3KB

### Relevant Code Locations
- Prompt rendering: `internal/prompt/prompt.go` — `BuildContext()` (line 326), `NewRenderer()` (line 153)
- Learnings loading: `internal/learnings/learnings.go` — `GetConfirmed()`, `GetRecent()`
- Build templates: `.gromit/templates/PROMPT_build.md`, `PROMPT_atdd_build.md`, `PROMPT_tdd_build.md`
- Validation flow: `internal/runner/process.go` — `runValidation()`, `runValidationWithRecovery()`
- Iteration logging: `internal/logger/logger.go` — `IterationLog` struct
- Rules loading: `internal/prompt/prompt.go` — loaded in `NewRenderer()` from `.gromit/RULES.md`
- Config defaults: `internal/config/config.go` — `setDefaults()`

### Dependencies
- Lever A depends on `IterationLog` having validation failure data (already present: `ValidationOutput` field or reconstructable from `Success` + `FailureOutput`)
- Lever B requires understanding how Claude Code loads CLAUDE.md (automatic on startup, not controllable per-invocation without `--no-project-context` or similar flag)
- Lever C builds on existing `learnings.File` API
