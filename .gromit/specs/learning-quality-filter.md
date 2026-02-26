---
id: learning-quality-filter
source_ideas: []
created: 2026-02-07
epic: codebase-health
---

# Learning Quality Filter

## Specification

Learnings should capture project-specific patterns, not restate generic engineering principles. Today, generic learnings (e.g., "always verify tests pass", "follow DRY principles", "use single responsibility") enter LEARNINGS.md unchecked and accumulate as noise until the next retro cycle archives them. This feature adds a two-layer defense: an LLM-based quality filter at creation time and tightened retro prompt guidance.

### Creation-Time LLM Filter

When `learnings.Add()` is called with a new learning, before writing it to LEARNINGS.md:

1. Send the learning text to a fast model (haiku) with a prompt asking whether the learning is project-specific or generic engineering advice.
2. The prompt should instruct the model to classify the learning as **"specific"** or **"generic"** based on whether it references project-specific patterns, files, packages, conventions, or behaviors — versus restating universally-known engineering principles.
3. If classified as **generic**: add the learning to the **archived** section of LEARNINGS.md with the reason `"filtered: generic engineering advice"`. This preserves the audit trail.
4. If classified as **specific**: proceed with normal `Add()` logic (deduplication, fuzzy matching, provisional placement).

The filter prompt should be concise and deterministic. It should include the project name and a brief description of what qualifies as project-specific (references to files, packages, bead IDs, specific error patterns, project conventions) versus generic (DRY, SRP, "always test", "handle errors", basic language features).

### Retro-Time Batch Filter

During `retro.Run()`, after loading learnings and before generating the retro prompt:

1. Run the same LLM filter against all **provisional** learnings that haven't been previously filtered.
2. Any provisional learning classified as generic gets archived with the same `"filtered: generic engineering advice"` reason.
3. This catches learnings that entered before this feature existed.
4. Track which learnings have been filter-evaluated (e.g., via a metadata marker or hash set) so they aren't re-evaluated on every retro.

### Retro Prompt Tightening

Add explicit anti-generic rules to `PROMPT_retro.md`. The retro prompt should instruct Claude to:

1. Archive any learning that restates standard engineering principles (DRY, SRP, SOLID, test coverage, error handling, code review) unless it references a project-specific pattern, file, or convention.
2. Archive any learning that describes basic programming language features or standard library behavior (e.g., "Go interfaces are satisfied implicitly", "use cmd.StdinPipe for input").
3. Archive any learning that could apply to any software project without modification.
4. When in doubt, archive — project-specific learnings reference concrete files, packages, bead patterns, or failure modes unique to this codebase.

These rules supplement the existing retro guidance and serve as a safety net for anything the creation-time filter misses.

## Acceptance Criteria

- When `learnings.Add()` is called with a generic learning (e.g., "Always verify tests pass before marking a bead complete"), it is added to the archived section with reason `"filtered: generic engineering advice"` instead of the provisional section.
- When `learnings.Add()` is called with a project-specific learning (e.g., "The runner's escalation chain skips haiku when the bead has complexity:high label"), it is added to the provisional section normally.
- During `retro.Run()`, existing provisional learnings that are generic get batch-archived before the retro prompt is generated.
- The retro prompt template includes explicit rules instructing Claude to archive generic learnings.
- The LLM filter uses the haiku model for cost efficiency.
- Previously-filtered learnings are not re-evaluated on subsequent retro runs.

## Decisions

1. **LLM-based filter over keyword matching.** Keyword/pattern blocklists are brittle and require maintenance. An LLM can understand nuance — e.g., "always run tests" is generic, but "run tests with `-count=1` to avoid cached results in the runner's CI step" is specific. The cost of a haiku call per learning is negligible.

2. **Archive rather than drop.** Filtered learnings go to the archived section rather than being silently discarded. This preserves a full audit trail, lets users review filter decisions, and allows recovery if the filter is too aggressive. The archived section already serves this purpose for retro-archived learnings.

3. **Filter at both creation and retro time.** Creation-time filtering prevents noise accumulation between retros. Retro-time batch filtering cleans up existing generics and catches anything the creation filter missed. Defense in depth.

4. **Explicit rules over examples in retro prompt.** The retro prompt already has a structured, rule-based format. The archived learnings in LEARNINGS.md (with their archival reasons) serve as implicit examples. Adding explicit rules is more maintainable and fits the existing template style.

5. **Track filter evaluation state.** To avoid re-evaluating learnings on every retro, track which ones have been filter-checked. This could be a hash set persisted in state.json or a metadata marker in the learning entry itself.

## Research & Context

### Current State

The learnings system lives in two packages:

- **`internal/learnings/learnings.go`** — Core CRUD: `Add()`, `Archive()`, `Replace()`, `Save()`, parsing of LEARNINGS.md. Has exact-hash deduplication and fuzzy matching (0.7 Jaccard similarity on trigrams) but no quality filtering.
- **`internal/retro/retro.go`** — Retro orchestration: loads learnings/rules, filters stuck beads, renders prompt, calls Claude (opus), parses proposals. The retro prompt template is at `.gromit/templates/PROMPT_retro.md`.

Learnings are created during `gromit run` when Claude's analysis output is parsed. The creation path has no quality gate — any text is accepted as a learning.

The retro prompt already instructs Claude to "be conservative" and "only promote patterns seen multiple times", but lacks explicit guidance about archiving generic learnings. The archived section of LEARNINGS.md contains ~47 entries, many manually flagged as "standard Go programming knowledge" or "generic software engineering advice".

### Integration Points

- **`learnings.Add()`** — Entry point for new learnings. The LLM filter hooks in here, before the deduplication/fuzzy-matching logic.
- **`retro.Run()`** — Entry point for retro analysis. The batch filter runs after loading learnings but before rendering the prompt.
- **`claude` package** — Used to invoke haiku for the filter. The existing `claude.RunHeadless()` or similar function can be reused.
- **`PROMPT_retro.md`** — Template file that needs the anti-generic rules added.
- **`state.json`** — Candidate for persisting the set of already-filtered learning hashes.
