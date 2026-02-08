---
created: 2026-02-07T00:00:00Z
decomposed: true
decomposed_at: "2026-02-07T23:53:16-05:00"
id: learning-quality-filter
source_spec: learning-quality-filter
---

# Learning Quality Filter Implementation Plan

**Goal:** Add a two-layer defense against generic learnings: an LLM-based quality filter at creation time and tightened retro prompt guidance.

**Architecture:** Dependency-injected `FilterFunc` callback on `learnings.File` that calls haiku to classify learnings as project-specific or generic. Generic learnings are archived instead of added to provisional. Batch filtering in retro uses the same function with hash-based tracking in `state.json`.

**Tech Stack:** Go, Claude CLI (haiku model), existing learnings/retro/state packages

**Spec:** `.gromit/specs/learning-quality-filter.md`

---

## Architecture

**Overview:**
Add a `FilterFunc` callback to the learnings package for dependency-injected quality filtering. The LLM classification function lives in a new `filter.go` file within the learnings package, and callers wire it up when creating `learnings.File` instances. Batch filtering for retro uses the same function with hash-based tracking in `state.json`.

**Key Components:**

1. **`FilterFunc` type + `SetFilter()` on `learnings.File`**: A callback `func(content string) (isGeneric bool, err error)` that `Add()` calls before dedup logic. If generic, archives directly instead of adding to provisional. Keeps the claude dependency out of core learnings logic.

2. **`NewLLMFilter()` factory in `internal/learnings/filter.go`**: Creates a `FilterFunc` that calls haiku via `claude.Client.Run()` with a concise classification prompt. The prompt includes project name and criteria for specific vs. generic.

3. **`FilterProvisional()` method on `learnings.File`**: Batch-filters all provisional learnings not yet evaluated. Takes a `FilterFunc` and a set of already-filtered hashes, returns newly-evaluated hashes for caller to persist.

4. **`FilteredLearningHashes` field in `state.State`**: A `[]string` persisted in `state.json` to track which learnings have been filter-evaluated.

**Integration Points:**
- `internal/runner/runner.go` — wire filter when creating learnings file
- `cmd/gromit/review.go` — wire filter at both learnings persistence sites
- `internal/retro/retro.go` — call batch filter before rendering retro prompt
- `.gromit/templates/PROMPT_retro.md` — add anti-generic archival rules

**Data Flow (creation-time):**
```
caller → learnings.Add(beadID, content, category)
  → filterFunc(content) → haiku classifies as "specific" or "generic"
  → if generic: archive with reason "filtered: generic engineering advice"
  → if specific: proceed with normal dedup/fuzzy/provisional logic
```

**Data Flow (retro-time batch):**
```
retro.Run() → load state.FilteredLearningHashes
  → learningsFile.FilterProvisional(filterFunc, filteredHashes)
  → for each unfiltered provisional: classify via haiku
  → archive generics, track all evaluated hashes
  → save updated hashes to state.json
  → proceed with retro prompt rendering
```

**Tradeoffs:**
- **`FilterFunc` callback over direct claude import**: Keeps learnings package testable without mocking exec.Command. Tests inject a deterministic function.
- **Hash set in state.json over metadata in LEARNINGS.md**: Avoids changing the human-readable LEARNINGS.md format. Aligns with existing state tracking patterns.

## Test Strategy

**Test Levels:**
1. **Unit Tests**: Filter integration in `Add()`, batch `FilterProvisional()`, state hash tracking, prompt construction, response parsing
2. **Integration Tests**: Retro flow with mock filter, runner wiring verification
3. **Manual Testing**: Run `gromit run` with real haiku to verify generic learnings get archived

**Key Test Cases:**
- `Add()` with generic filter → archived with reason
- `Add()` with specific filter → normal provisional placement
- `Add()` with filter error → fall through to normal logic
- `Add()` with no filter set → backward compatible
- `Add()` filter + exact duplicate → dedup short-circuits before filter
- `FilterProvisional()` basic → archives generics, keeps specifics
- `FilterProvisional()` skip already-filtered hashes
- `FilterProvisional()` filter error → skip, continue others
- `FilterProvisional()` empty provisionals → no filter calls
- State `FilteredLearningHashes` persistence round-trip
- LLM filter prompt includes project name and classification criteria
- LLM filter response parsing: "specific" → false, "generic" → true, unexpected → error

**Mocking Strategy:**
- Inject deterministic `FilterFunc` in all learnings tests
- Use `t.TempDir()` for real file I/O (existing pattern)
- Test prompt construction and response parsing separately from actual LLM calls

**Test Organization:**
- `internal/learnings/learnings_test.go` — `Add()` with filter, `FilterProvisional()`
- `internal/learnings/filter_test.go` — prompt construction, response parsing
- `internal/state/state_test.go` — hash persistence
- `internal/retro/retro_test.go` — batch filter wiring

## Implementation Tasks

### Task 1: Add FilterFunc type and integrate into Add()

**Files:**
- Modify: `internal/learnings/learnings.go`
- Test: `internal/learnings/learnings_test.go`

**What to Do:**
Define `FilterFunc` type as `func(content string) (isGeneric bool, err error)`. Add `filterFunc FilterFunc` field to `File` struct and `SetFilter(fn FilterFunc)` method. Modify `Add()` to call `filterFunc` after exact-duplicate checks but before fuzzy matching. If filter returns `isGeneric=true`, add the learning directly to the archived section with reason `"filtered: generic engineering advice"` and return. If filter returns an error, log a warning and fall through to normal logic (don't block on filter failure). If `filterFunc` is nil, skip filtering entirely (backward compatible).

**Acceptance Criteria:**
- `Add()` with a filter that returns generic → learning appears in archived section with correct reason, not in provisional
- `Add()` with no filter set → existing behavior unchanged, all current tests pass
- `Add()` with filter error → falls through to normal provisional placement

**Dependencies:**
- None (foundational task)

**Notes:**
The filter should be called after exact-duplicate checks (lines 106-115) to avoid wasting LLM calls on duplicates. The archived learning should use the same format as `Archive()` but can skip the "Archived from provisional" prefix since it was never in provisional — use a direct append to `f.archived` with the reason embedded in content.

### Task 2: Implement LLM classification function

**Files:**
- Create: `internal/learnings/filter.go`
- Create: `internal/learnings/filter_test.go`

**What to Do:**
Create `NewLLMFilter(claudeClient ClaudeRunner, projectName string) FilterFunc` factory function. Define a `ClaudeRunner` interface with `Run(ctx context.Context, prompt string, model string) (*claude.Result, error)` to enable testing without exec.Command. The filter prompt should ask haiku to classify a learning as "specific" or "generic" based on whether it references project-specific patterns (files, packages, bead IDs, error patterns, conventions) vs. generic engineering principles (DRY, SRP, SOLID, "always test", basic language features). Parse the response: contains "generic" → `isGeneric=true`, contains "specific" → `isGeneric=false`, otherwise → return error. The prompt should be concise to minimize token usage.

**Acceptance Criteria:**
- `NewLLMFilter()` returns a `FilterFunc` that calls the claude client with haiku model
- Response parsing correctly maps "specific" → false, "generic" → true
- Unexpected responses return an error (not a false classification)

**Dependencies:**
- Task 1 (defines `FilterFunc` type)

**Notes:**
Use `context.Background()` for the haiku call since individual learning classification doesn't need the bead timeout. The `ClaudeRunner` interface avoids importing claude directly into the filter — the actual `claude.Client` satisfies the interface. Keep the prompt under ~200 tokens for cost efficiency.

### Task 3: Add FilteredLearningHashes to state

**Files:**
- Modify: `internal/state/state.go`
- Test: `internal/state/state_test.go`

**What to Do:**
Add `FilteredLearningHashes []string` field to `State` struct with JSON tag `filtered_learning_hashes,omitempty`. Add `GetFilteredHashes() map[string]bool` method that converts the slice to a set for efficient lookup. Add `AddFilteredHashes(hashes []string)` method that merges new hashes into the existing list, deduplicating. These are simple accessor/mutator methods following the existing pattern (e.g., `RecordRetro()`).

**Acceptance Criteria:**
- `FilteredLearningHashes` persists across Save/Load cycle
- `GetFilteredHashes()` returns a map for O(1) lookup
- `AddFilteredHashes()` merges without duplicates

**Dependencies:**
- None (foundational task, independent of Task 1)

### Task 4: Add FilterProvisional batch method

**Files:**
- Modify: `internal/learnings/learnings.go`
- Test: `internal/learnings/learnings_test.go`

**What to Do:**
Add `FilterProvisional(fn FilterFunc, alreadyFiltered map[string]bool) ([]string, error)` method to `File`. Iterate over provisional learnings. Skip any whose hash is in `alreadyFiltered`. Call `fn(content)` for each unfiltered learning. If generic, call `Archive(hash, "filtered: generic engineering advice")`. Collect all evaluated hashes (both generic and specific) in the return slice. If `fn` returns an error for a specific learning, skip that learning and continue. Iterate in reverse order since `Archive()` modifies the provisional slice.

**Acceptance Criteria:**
- Generic provisionals get archived with correct reason
- Specific provisionals remain in provisional
- Already-filtered hashes are skipped (no redundant LLM calls)
- Filter errors on individual learnings don't stop the batch

**Dependencies:**
- Task 1 (defines `FilterFunc` type)

**Notes:**
Iterate in reverse (or collect hashes to archive and process after iteration) because `Archive()` modifies `f.provisional` in place. The safer approach is to collect hashes to archive first, then archive them in a separate loop.

### Task 5: Wire filter into runner and review callers

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `cmd/gromit/review.go`

**What to Do:**
In `internal/runner/runner.go`, in `checkRetroSuggestion()` (line 879) or wherever the learnings file is created for the run loop, the filter is not needed here since learnings are added via `process.go` which uses `r.renderer.GetLearningsFile()`. Find where that learnings file is created/initialized and call `SetFilter(NewLLMFilter(claudeClient, projectName))` on it. In `cmd/gromit/review.go`, at both call sites (lines 440 and 520) where `learnings.NewFile()` is created, set the filter using the claude client available in the review context.

**Acceptance Criteria:**
- Learnings added during `gromit run` are filtered through haiku before placement
- Learnings added during `gromit review` are filtered through haiku before placement
- Existing behavior preserved when claude client is unavailable (filter not set)

**Dependencies:**
- Task 1 (FilterFunc type and SetFilter)
- Task 2 (NewLLMFilter factory)

**Notes:**
The runner's learnings file is created in the prompt renderer (`internal/prompt/prompt.go:152`). The filter needs to be set after the learnings file is created there. Check whether the prompt package's `NewRenderer` or similar function is the right place, or if the runner should set the filter after getting the learnings file from the renderer.

### Task 6: Wire batch filter into retro.Run()

**Files:**
- Modify: `internal/retro/retro.go`
- Test: `internal/retro/retro_test.go`

**What to Do:**
In `retro.Run()`, after `r.learningsFile.Load()` (line 77) and before `r.formatLearnings()` (line 88): load state to get filtered hashes, create the LLM filter using `r.claude` and the project name, call `r.learningsFile.FilterProvisional(filterFunc, filteredHashes)`, save newly-evaluated hashes back to state. This ensures generic provisionals are archived before the retro prompt sees them.

**Acceptance Criteria:**
- Generic provisional learnings are archived before the retro prompt is rendered
- Already-filtered learnings are not re-evaluated
- Newly-filtered hashes are persisted to state.json
- Filter failures don't block the retro from running

**Dependencies:**
- Task 2 (NewLLMFilter factory)
- Task 3 (FilteredLearningHashes in state)
- Task 4 (FilterProvisional method)

### Task 7: Update retro prompt template with anti-generic rules

**Files:**
- Modify: `.gromit/templates/PROMPT_retro.md`

**What to Do:**
Add a new section to the `## Guidelines` area of the retro prompt template with explicit anti-generic archival rules. The rules should instruct Claude to: (1) archive any learning that restates standard engineering principles (DRY, SRP, SOLID, test coverage, error handling) unless it references a project-specific pattern/file/convention, (2) archive any learning that describes basic language features or stdlib behavior, (3) archive any learning that could apply to any software project without modification, (4) when in doubt, archive — project-specific learnings reference concrete files, packages, bead patterns, or failure modes unique to this codebase.

**Acceptance Criteria:**
- The retro prompt template includes explicit anti-generic archival rules
- Rules are in the Guidelines section, supplementing existing guidance
- Rules are clear enough for Claude to apply consistently

**Dependencies:**
- None (independent template change)

**Notes:**
This is the safety net layer. Even if the LLM filter misses something at creation time, the retro prompt should catch it. Keep the rules concise and actionable — Claude will see them alongside the existing "be conservative" guidance.

---

## Notes

- **Backward compatibility**: All existing callers that don't set a filter continue to work unchanged. The filter is opt-in via `SetFilter()`.
- **Cost**: Each learning classification costs one haiku call (~0.001 USD). With typically 1-5 learnings per run, this is negligible.
- **Error handling**: Filter failures never block learning creation or retro execution. They fall through to existing behavior.
- **The `ClaudeRunner` interface in Task 2 needs to match `claude.Client.Run()`'s signature**. The actual claude.Client satisfies it implicitly (Go structural typing).
- **Task 5 requires investigation** of the prompt renderer's learnings file lifecycle to find the right wiring point. The runner accesses learnings via `r.renderer.GetLearningsFile()`, so the filter should be set on that instance.
