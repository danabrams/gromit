---
created: 2026-02-19T00:00:00Z
decomposed: true
decomposed_at: "2026-02-19T21:37:26Z"
id: dynamic-code-map
source_spec: dynamic-code-map
---

# Dynamic Code Map Implementation Plan

**Goal:** Replace the static CLAUDE.md architecture listing in build prompts with a bead-scoped code map while always preserving key principles and falling back safely when scope cannot be inferred.

**Architecture:** Prompt assembly will compute relevant packages from spec/description text and sibling completed-work history, render a minimal package map with one-line descriptions, and swap only the CLAUDE architecture section when at least one package is found.

**Tech Stack:** Go (`internal/prompt`, `internal/runner`, `internal/logger`), Markdown section parsing, JSONL iteration logs.

**Spec:** `.gromit/specs/dynamic-code-map.md`

---

## Architecture

**Overview:**  
Add a deterministic CLAUDE-scoping layer inside prompt context assembly. It extracts relevant package paths (Layer 1 from spec/bead text, Layer 2 from sibling touched-package history), builds a minimal code map, and replaces only CLAUDE's Architecture section. If no packages are identified, BuildContext keeps full CLAUDE.md unchanged.

**Key Components:**
1. **Prompt code-map resolver (`internal/prompt`)**: Extract package paths, normalize/dedupe/sort, and decide fallback vs scoped rendering.
2. **CLAUDE section parser (`internal/prompt`)**: Split Architecture vs Key Principles and rebuild content with Architecture replaced only.
3. **Sibling package enrichment wiring (`internal/runner` + `internal/logger`)**: Persist touched packages per iteration and expose sibling package union lookup by parent/spec context.
4. **Package description resolver (`internal/prompt`)**: Resolve one-line package descriptions from CLAUDE architecture bullets first, then Go package doc comments, then safe generic fallback text.

**Integration Points:**
- `internal/prompt/prompt.go` `BuildContext()` remains the single point where CLAUDE context is loaded and transformed.
- Runner initialization injects a sibling-package resolver callback into the prompt renderer so `internal/prompt` remains independent from direct bd command execution.
- Iteration logging writes touched packages to JSONL so sibling enrichment can use historical completed work without re-diffing old commits.

**Data Flow:**
1. `BuildContext()` loads CLAUDE, RULES, learnings, workdir, and linked spec as today.
2. Layer 1 extracts `internal/...` and `cmd/...` package paths from spec content plus bead/parent descriptions.
3. Layer 2 queries sibling touched packages via injected resolver when parent epic context exists.
4. Package sets are unioned, normalized, deduped, and sorted.
5. If non-empty: render minimal scoped code map and rebuild `ctx.ClaudeMD` as `Architecture(scoped)` + original `Key Principles` section.
6. If empty: preserve `ctx.ClaudeMD` unchanged.

**Files to Modify:**
- `internal/prompt/prompt.go` - add BuildContext scoping flow and resolver hooks.
- `internal/logger/logger.go` - add touched-packages field to `IterationLog`.
- `internal/runner/logging.go` - populate touched-packages in iteration log writes.
- `internal/runner/constructor.go` - wire prompt renderer with sibling package lookup callback.
- `internal/runner/interfaces.go` and relevant mocks/tests - maintain compatibility for any new renderer methods.

**Files to Create:**
- `internal/prompt/claude_scoping.go` - section parsing, package extraction, package description selection, scoped CLAUDE rendering.
- `internal/prompt/claude_scoping_test.go` - unit tests for scoping logic.
- `internal/logger/sibling_packages.go` - helper(s) for sibling touched-package aggregation from logs.
- `internal/logger/sibling_packages_test.go` - unit tests for aggregation semantics.

**Tradeoffs:**
- **Injected resolver callback over direct bead client calls in prompt**: keeps prompt package focused and unit-testable.
- **Reuse JSONL touched-package data over re-deriving from git history**: lower complexity and no extra command cost.
- **Regex/path pattern extraction for Layer 1 over heavier parsing**: aligns with spec language and remains cheap; future scope gate enhancement can extend recall later.
- **CLAUDE descriptions first, go-doc fallback second**: keeps output stable and human-friendly while still covering new packages.

## Test Strategy

**Test Levels:**
1. **Unit Tests (`internal/prompt`)**: package extraction, CLAUDE section replacement, format rendering, and fallback behavior.
2. **Unit Tests (`internal/logger`)**: sibling touched-package aggregation from iteration logs.
3. **Integration Tests (`internal/prompt` + runner wiring)**: `BuildContext()` output with scoped vs full CLAUDE behavior.
4. **Regression Tests (`internal/runner`)**: touched-packages logging/wiring does not break existing iteration logging behavior.

**Key Test Cases:**
- Spec text mentions package paths and they appear in scoped code map.
- Bead or parent description mentions package paths and they appear in scoped code map.
- Sibling completed iterations contribute touched packages to relevant set.
- Scoped mode replaces Architecture only and preserves Key Principles section text.
- No discovered packages triggers full CLAUDE fallback unchanged.
- Code-map output is minimal bullet list with one-line descriptions.
- Description sourcing order: CLAUDE architecture mapping, then package doc comment, then fallback label.
- Deterministic ordering and deduplication across both discovery layers.

**Mocking Strategy:**
- Inject fake sibling-package resolver into prompt renderer for targeted tests.
- Use temp CLAUDE/spec fixtures for file-backed parsing tests.
- Use synthetic JSONL logs for logger aggregation tests.
- Keep template rendering real in integration tests; mock only external lookups.

**Coverage Goals:**
- Critical path in `BuildContext()` for scoped CLAUDE generation.
- Edge handling for missing Architecture/Key Principles headers, empty spec, nil parent bead, and duplicate package signals.
- Backward compatibility guarantee when discovery layers produce no relevant packages.

**Test Organization:**
- `internal/prompt/claude_scoping_test.go` for helper-level logic.
- `internal/prompt/prompt_test.go` additions for BuildContext end-to-end behavior.
- `internal/logger/sibling_packages_test.go` for aggregation and filtering.
- Runner wiring tests in existing runner test files for resolver hookup and touched-packages log propagation.

## Implementation Tasks

### Task 1: Add CLAUDE section parsing and scoped rendering primitives

**Files:**
- Create: `internal/prompt/claude_scoping.go`
- Test: `internal/prompt/claude_scoping_test.go`

**What to Do:**
Implement helpers to parse CLAUDE.md into major sections, extract the Architecture block, preserve Key Principles block, and render a replacement Architecture section using minimal bullets (`- \`path/\` — description`). Add deterministic formatting helpers and graceful handling when expected headers are missing.

**Acceptance Criteria:**
- Can parse CLAUDE content and isolate Architecture and Key Principles sections.
- Can render scoped CLAUDE content that replaces only Architecture and preserves Key Principles text.
- Missing/partial section structure falls back safely without panic.

**Dependencies:**
- None.

**Notes:**
- Keep helpers pure and file-system agnostic for unit testing.

### Task 2: Implement package discovery Layer 1 from spec and bead text

**Files:**
- Modify: `internal/prompt/claude_scoping.go`
- Test: `internal/prompt/claude_scoping_test.go`

**What to Do:**
Add package extraction logic that scans spec content, bead description, and parent description for Go package path patterns (`internal/...`, `cmd/...`). Normalize trailing slashes, dedupe, and sort.

**Acceptance Criteria:**
- Package paths referenced in spec text are extracted.
- Package paths referenced in bead/parent descriptions are extracted.
- Output is deduplicated, normalized, and deterministic.

**Dependencies:**
- Task 1.

**Notes:**
- Avoid overmatching file paths that do not map cleanly to package roots.

### Task 3: Add description resolution for scoped packages

**Files:**
- Modify: `internal/prompt/claude_scoping.go`
- Test: `internal/prompt/claude_scoping_test.go`

**What to Do:**
Implement one-line description lookup order: first from CLAUDE architecture bullets, then Go package doc comment under repo root, then fallback generic description. Ensure rendering never emits empty descriptions.

**Acceptance Criteria:**
- Known packages in existing CLAUDE architecture retain familiar descriptions.
- Packages missing in CLAUDE can use go-doc fallback when available.
- All rendered package lines include non-empty one-line descriptions.

**Dependencies:**
- Task 1.

**Notes:**
- Keep go-doc reads bounded and best-effort to avoid BuildContext failures.

### Task 4: Add sibling touched-package aggregation in logger layer

**Files:**
- Create: `internal/logger/sibling_packages.go`
- Test: `internal/logger/sibling_packages_test.go`

**What to Do:**
Implement helper(s) that scan iteration logs and return unioned touched packages for sibling beads under the same parent epic/spec context, constrained to completed iterations.

**Acceptance Criteria:**
- Aggregation returns union of sibling touched packages from qualifying entries.
- Non-qualifying records (different spec/parent/incomplete) are excluded.
- Result ordering is deterministic.

**Dependencies:**
- None.

**Notes:**
- Keep API small so runner can inject it into prompt renderer cleanly.

### Task 5: Persist touched packages in iteration logs

**Files:**
- Modify: `internal/logger/logger.go`
- Modify: `internal/runner/logging.go`
- Test: existing logger/runner logging tests

**What to Do:**
Extend `logger.IterationLog` with `touched_packages` and populate it from per-bead touched package state when writing logs.

**Acceptance Criteria:**
- Iteration JSONL records include touched packages when available.
- Existing log decoding/processing remains backward compatible with older logs.
- Existing logging tests updated to assert new field where appropriate.

**Dependencies:**
- None.

**Notes:**
- Keep zero-value omission behavior aligned with current JSON conventions.

### Task 6: Wire sibling enrichment callback into prompt renderer

**Files:**
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/runner/constructor.go`
- Modify: `internal/runner/interfaces.go` and test mocks as needed

**What to Do:**
Add a renderer-level setter or injected function for sibling touched-package lookup and wire it in runner construction so prompt context building can request Layer 2 enrichment without importing runner logic.

**Acceptance Criteria:**
- Renderer accepts optional sibling resolver callback.
- Runner sets callback in production construction path.
- Existing tests compile with updated interfaces/mocks.

**Dependencies:**
- Task 4.

**Notes:**
- Resolver failures should degrade gracefully (no enrichment) rather than failing BuildContext.

### Task 7: Integrate dynamic code map into BuildContext

**Files:**
- Modify: `internal/prompt/prompt.go`
- Modify: `internal/prompt/claude_scoping.go`
- Test: `internal/prompt/prompt_test.go` and `internal/prompt/claude_scoping_test.go`

**What to Do:**
Update `BuildContext()` to run both discovery layers, merge results, and conditionally replace Architecture section with scoped code map while always retaining Key Principles. Preserve full CLAUDE fallback when no packages are discovered.

**Acceptance Criteria:**
- With discovered packages, `ctx.ClaudeMD` includes scoped package map + key principles and omits full static architecture listing.
- With no discovered packages, `ctx.ClaudeMD` is unchanged from `LoadClaudeMD()` output.
- Behavior is deterministic across repeated calls with cached content.

**Dependencies:**
- Task 1
- Task 2
- Task 3
- Task 6

**Notes:**
- Maintain existing BuildContext error handling and caching behavior.

### Task 8: Add end-to-end acceptance coverage and token-size guard

**Files:**
- Modify: `internal/prompt/prompt_test.go`
- Modify: `internal/prompt/budget_test.go` (or dedicated scoping test file)

**What to Do:**
Add integration-style tests covering the full acceptance matrix from the spec: spec parsing, bead description parsing, sibling enrichment, scoped replacement, full fallback, and a size comparison test showing scoped map with small package set is smaller than full architecture section.

**Acceptance Criteria:**
- Acceptance criteria scenarios are represented in tests.
- Scoped output for a two-package case is measurably smaller than full architecture section.
- Regression tests protect against accidental removal of key principles.

**Dependencies:**
- Task 5
- Task 7

**Notes:**
- Keep token-size assertion robust by comparing character counts in controlled fixtures.

---

## Notes

- This rollout is strictly additive and should preserve current behavior whenever discovery confidence is low (no packages found).
- Task decomposition is designed so `gromit decompose dynamic-code-map` can split each task into 1-3 beads with clear boundaries.
- If section parsing or sibling lookup proves noisy in practice, prioritize deterministic fallback over aggressive inference.
