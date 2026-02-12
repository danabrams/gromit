# Learnings

Accumulated operational knowledge from Gromit iterations.
This file is automatically updated. Review periodically with `gromit retro`.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

### 2026-02-07 | Mock Implementation Patterns | patterns
*Related to: ge6j, atmb*

Mock implementations use optional function pointer fields (FnField pattern) with nil-safe defaults. Tests set up only the behavior needed via tracking flags or injected callbacks, enabling focused verification of specific code paths without requiring full mock setup.

### 2026-02-07 | Status File Management | patterns
*Related to: nalr, k8c2, kydj, ead1, xpfn, lm34, 2y2d, yj2h, vpyl, kim2*

Status struct fields require backward-compatible changes (omitempty for new optional fields). Use ReadStatus()/IsProcessAlive() for state + liveness checks. Return nil,nil for missing optional files (not an error). StatusWriter handles both lifecycle states and preserves completed iteration count on shutdown. Round-trip tests verify serialization fidelity. Stale resource cleanup integrates into status reporting via process liveness checks. Process utilities (IsProcessAlive) are co-located with Status in status.go. Test file I/O uses t.TempDir() for isolation.

### 2026-02-07 | Methodology Label Activation | patterns
*Related to: ralph-runner-4a3f, ralph-runner-nzue*

Methodologies use label-based activation ("methodology:true"/"false") with global config fallback via bead.IsMethodologyActive(). When active, replace the build prompt with a specialized RenderXXXBuild method. Check parent labels before adding globally-active methodology labels to sub-beads to avoid duplicates. Order methodology checks carefully for precedence when multiple methodologies are active.

### 2026-02-08 | Runner Method Pattern | patterns
*Related to: gromit-5pvp, gromit-82qx, gromit-vabo*

Runner methods follow a consistent pattern: nil-safe receiver/config checks, feature-flag gating (e.g., IsAutoPushEnabled()), context.WithTimeout for subprocess calls, and failure handling via mode field (warn vs stop) or skippedBeads map rather than inline error handling. The Run() method follows a clear sequencing: validate -> execute work -> persist state -> run between-iteration hooks -> continue loop. Log warnings on timeout errors rather than returning early.

### 2026-02-11 | Router Conversion Pattern | patterns
*Related to: gromit-juyb, gromit-2zju, gromit-557p, gromit-3gdz, gromit-gibz*

Router-based calls use phase + tier parameters: phase identifies the operation (build/validate/review/refactor), tier selects the complexity level (low/medium/high). Follow the executeClaudeInvocation pattern: extract tier selection, use router.StreamRun() for the initial call, handle UsageLimitError by escalating to the next tier. Distinguish between tier selection and model selection — SetEscalatedTo gets the final model name from TierToModel, not the tier string. Standard tiers: validation uses phase="validate" tier="low"; reviews use selectTier(bead) or "high" for opus builds/thorough reviews. When converting call sites, check all related functions in the same flow (e.g., runRefactorPhase + handleRefactorValidationFailure + runPostSuccessReview). Router conversions in tests require mockProviderWithRouterTracking and real git repo fixtures for acceptance tests. Use selectReviewTier() helper which delegates to selectTier() for non-opus and returns "high" for opus builds.

### 2026-02-11 | Acceptance Test Line Budget | conventions
*Related to: gromit-lxlp, gromit-nqf1, gromit-2edx, gromit-557p, gromit-3gdz*

Acceptance tests (//go:build acceptance) are subject to line count audits — total across all files must not exceed the budget ceiling (currently 6,000 lines, rebased after cleanup achieved 38.5% reduction from original 8,370). New test code should prefer unit tests for API verification, not acceptance tests. Task specs that add test coverage should account for the total line budget. Test metrics are enforced via final_verification_test.go.

### 2026-02-11 | Prompt Template Structure | conventions
*Related to: gromit-rpne*

Prompt templates in .gromit/templates/ use explicit section headers (##) and preserve exact whitespace/structure when updating. Template files follow a consistent structure: context section at top, then Guidelines, then preserved sections like 'Avoiding Sibling Overlap' and ATDD blocks. When modifying sections, maintain blank lines between sections and ensure downstream blocks remain unchanged. Acceptance tests for template changes must match the exact content being added, including specific phrases and subsection structure.

---

## Provisional

*Seen once - may be specific to one task.*

### 2026-02-11 | gromit-03lk | conventions
When expanding a struct with new fields in the pipeline package, you must: (1) update ReadStatus() to populate the new count fields by iterating through beads and counting by status, (2) update the formatting/display logic (likely in runner/format_bead_breakdown.go) to include these new counts in status output, and (3) ensure any helper methods for counting beads are called correctly. The count fields won't be used automatically—they require explicit population and display logic.

### 2026-02-11 | gromit-mz3m | conventions
When migrating from one pattern to another (e.g., ClaudeClient → Provider), ensure structural changes are completed across all types (not just the adapter/analyzer). Search for all embedded fields and struct initializations across the codebase—not just method calls. Fields like Runner.claude need to be identified and removed after all call sites are migrated to the new interface. Acceptance test files must include the `//go:build acceptance` build tag to be properly categorized.

---

## Archived

*No longer relevant or superseded.*

### 2026-02-12 | router conversion originals | patterns
Archived: gromit-juyb (x2), gromit-2zju (x2), gromit-557p, gromit-3gdz, gromit-gibz (x3) consolidated into Confirmed "Router Conversion Pattern" entry.

### 2026-02-12 | acceptance test budget originals | conventions
Archived: gromit-lxlp, gromit-nqf1, gromit-2edx, gromit-557p, gromit-3gdz consolidated into Confirmed "Acceptance Test Line Budget" entry.

### 2026-02-12 | prompt template originals | conventions
Archived: gromit-rpne (x3) consolidated into Confirmed "Prompt Template Structure" entry.

### 2026-02-12 | provider migration originals | conventions
Archived: gromit-mz3m (x2) consolidated into Provisional "Provider Migration" entry.

### 2026-02-12 | gromit-c2ax | conventions
Archived: SetDefaults() + NormalizeNilFields() requirement promoted to RULES.md Process section. Rule is the source of truth.

*Archived from provisional: promoted to rules*

### 2026-02-12 | gromit-juyb tier-vs-model | patterns
Archived: tier-vs-model distinction promoted to RULES.md Code Style section. Rule is the source of truth.

*Archived from provisional: promoted to rules*

### 2026-02-12 | gromit-o4ow | patterns
Archived: standard Go testing patterns (subtests with t.Run, table-driven tests). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-12 | gromit-j2p9 | patterns
Archived: standard testing practice (synthetic test data with varying completeness for backward compatibility). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-12 | gromit-oqtr | patterns
Archived: standard data flow advice (wire through capture/store/propagate layers). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-12 | gromit-2xrq | conventions
Archived: generic documentation practice (inline comments explaining rationale). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-12 | gromit-eo57 | patterns
Archived: gromit-eo57 (x2) aggregation function patterns — standard Go file processing (glob, filter, iterate, accumulate). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-12 | gromit-jsta | patterns
Archived: atomic file updates (read-modify-write with temp-file-then-rename). Standard Go pattern, not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-12 | gromit-zyc8 | patterns
Archived: generic process advice (spike tasks document findings in markdown). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-12 | gromit-7oob | patterns
Archived: standard Go pattern (map lookups with strings.ToLower, pass-through for unknown values). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-12 | gromit-2zju | conventions
Archived: acceptance tests validating codebase-wide metrics — subsumed by Confirmed "Acceptance Test Line Budget" entry.

*Archived from provisional: consolidated*

### 2026-02-11 | ValidateSpec Before ResolveSpec Convention | conventions
*Related to: gromit-4yrb, gromit-wwhq*

Archived: promoted to RULES.md Code Style section. Rule is the source of truth.

*Archived from confirmed: promoted to rules*

### 2026-02-11 | ValidateSpec ordering originals | conventions
Archived: gromit-4yrb, gromit-wwhq (x2) consolidated into Confirmed "ValidateSpec Before ResolveSpec Convention" entry.

### 2026-02-11 | gromit-1wen | conventions
Bead sizing rules are enforced across multiple documentation files (.gromit/RULES.md, SKILL.md, PROMPT_decompose.md, and CLAUDE.md). Changes to sizing rules must be propagated to all four files consistently, not just CLAUDE.md.

*Archived from provisional: promoted to rules*

### 2026-02-11 | gromit-1wen | conventions
Archived: generic engineering advice (keep architecture docs directory-based). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-10 | Refactoring Bead Cascade Risks | conventions
*Related to: gromit-d6sl, gromit-gdzl, gromit-uwuh, gromit-w0lo, gromit-66dz*

Archived: key insights incorporated into RULES.md Process section (bead splitting rule — cascade verification for shared packages, diff-vs-intent review). Rule is the source of truth.

*Archived from provisional: promoted to rules*

### 2026-02-10 | Test-Only Bead Detection Pattern | patterns
*Related to: gromit-755e*

Archived: promoted to RULES.md Process section. Rule is the source of truth.

*Archived from provisional: promoted to rules*

### 2026-02-09 | gromit-pame | conventions
Archived: Describes standard cobra CLI pattern (package-level flag variables for direct access). This is basic Go CLI convention, not project-specific — any cobra project uses this pattern.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-06 | gromit-ehn | gotchas
Archived: duplicate of consolidated validation commands learning. Subsumed by promoted rule.

### 2026-02-06 | gromit-0o2 | gotchas
Archived: duplicate of consolidated validation commands learning. Subsumed by promoted rule.

### 2026-02-06 | gromit-rz1 | gotchas
Archived: duplicate of consolidated validation commands learning. Subsumed by promoted rule.

### 2026-02-07 | ralph-runner-0354 | conventions
Archived: restates a fundamental property of statically typed languages (changing signatures requires updating callers). Existing bead-sizing rules already cover this implicitly.

### 2026-02-07 | ralph-runner-4ahm | gotchas
Archived: restates system-level scratchpad directory guidance already provided in the Claude session environment. Environment-specific operational advice, not a project constraint.

### 2026-02-06 | validation command originals | gotchas
Archived: already promoted to rule in RULES.md Process section. Rule is the source of truth.

### 2026-02-07 | ralph-runner-utv8 | conventions
Archived: consolidated into golden file maintenance rule. When adding new prompt templates to the init command, update both the template implementation AND the corresponding CLI contract test golden file to match the new help text output.

### 2026-02-07 | ralph-runner-utv8 | conventions
Archived: consolidated into golden file maintenance rule. When modifying CLI commands that generate files or output, check and update any golden file tests (contract tests) that capture the expected output.

### 2026-02-07 | ralph-runner-utv8 | conventions
Archived: consolidated into golden file maintenance rule. When adding new templates to the gromit init process, the golden file test that validates CLI help text output must be updated in the same commit.

### 2026-02-07 | ralph-runner-utv8 | patterns
Archived: consolidated into Template and Renderer Architecture confirmed learning.

### 2026-02-07 | ralph-runner-uu07 | patterns
Archived: consolidated into Template and Renderer Architecture confirmed learning.

### 2026-02-07 | ralph-runner-j3ln | patterns
Archived: consolidated into Dependency Injection for Testability confirmed learning.

### 2026-02-07 | ralph-runner-ge6j | patterns
Archived: consolidated into Mock Implementation Patterns confirmed learning.

### 2026-02-07 | ralph-runner-rax7 | conventions
Archived: consolidated into Test Helper Delegation confirmed learning.

### 2026-02-07 | shell safety originals | gotchas
Archived: m7td and 6rao consolidated into Confirmed "Shell Safety" entry.

### 2026-02-07 | test helper originals | conventions
Archived: rkub, ewqu, uucn, y0v3 consolidated into Confirmed "Test Helper Delegation" entry.

### 2026-02-07 | documentary test originals | patterns
Archived: dpk5, prk6, q2id, l0ka consolidated into Confirmed "Documentary Test Replacement" entry.

### 2026-02-07 | mock pattern originals | patterns
Archived: ge6j, atmb consolidated into Confirmed "Mock Implementation Patterns" entry.

### 2026-02-07 | status file originals | patterns
Archived: nalr, k8c2, kydj, ead1, xpfn, lm34, 2y2d, yj2h, vpyl, kim2 consolidated into Confirmed "Status File Management" entry.

### 2026-02-07 | formatting originals | patterns
Archived: 5xbi, 88fk, 9ntm consolidated into Confirmed "Output Formatting" entry.

### 2026-02-07 | ralph-runner-8i7k | gotchas
Archived: promoted to RULES.md Code Style section. bead.Client Ready()/CountReady() vs List() semantic distinction.

### 2026-02-07 | ralph-runner-ouh4 | gotchas
Archived: one-off fallback syntax issue (jq vs python3). Not a recurring project pattern.

### 2026-02-07 | LEARNINGS.md format originals | conventions
Archived: gromit-8a52, gromit-i0wm, gromit-wgtu, gromit-2v5n, gromit-49dg, gromit-8et, gromit-3gb, gromit-kim, gromit-due, gromit-1u7, gromit-evq, gromit-w3x, gromit-ltg, gromit-knc, gromit-3fqu, gromit-3ibd, gromit-hvbu, gromit-fntb, gromit-7e5o, gromit-2qf1, gromit-yew, gromit-ie2 consolidated into Confirmed "LEARNINGS.md Format Validation" entry.

### 2026-02-07 | template originals | conventions
Archived: ralph-runner-utv8, ralph-runner-yx7b, ralph-runner-5lk0, ralph-runner-kjix consolidated into Confirmed "Template and Renderer Architecture" entry.

### 2026-02-07 | methodology originals | patterns
Archived: ralph-runner-4a3f, ralph-runner-nzue consolidated into Confirmed "Methodology Label Activation" entry.

### 2026-02-07 | renderer originals | conventions
Archived: ralph-runner-628c, ralph-runner-nxdm consolidated into Confirmed "Renderer Method Pattern" entry.

### 2026-02-07 | testing originals | patterns
Archived: ralph-runner-yysu, ralph-runner-2dsc consolidated into Confirmed "Table-Driven and Concurrent Testing" entry.

### 2026-02-07 | DI originals | patterns
Archived: ralph-runner-tizz, ralph-runner-tsf4 consolidated into Confirmed "Dependency Injection for Testability" entry.

### 2026-02-07 | test failure originals | conventions
Archived: gromit-jva, gromit-knc consolidated into Confirmed "Test Failure Root Cause Analysis" entry.

### 2026-02-07 | helper originals | conventions
Archived: ralph-runner-uq8m, ralph-runner-2m53 consolidated into Confirmed "Helper Function Extraction" entry.

### 2026-02-07 | syncWriter originals | patterns
Archived: ralph-runner-qcoc, ralph-runner-t13e, ralph-runner-z6li consolidated into Confirmed "syncWriter Thread Safety" entry.

### 2026-02-07 | ralph-runner-d9xd | gotchas
Archived: already promoted to RULES.md Safety section (temp files for large strings to avoid ARG_MAX).

### 2026-02-07 | ralph-runner-en3f | gotchas
Archived: standard Unix/Go programming knowledge (git %at timestamps need int64 parsing). Not project-specific.

### 2026-02-07 | gromit-f4t | patterns
Archived: trivial test maintenance (updating fixture IDs). Standard development practice.

### 2026-02-07 | ralph-runner-9u1c | conventions
Archived: too specific to streaming output formatting. Describes one function's behavior rather than a reusable pattern.

### 2026-02-07 | gromit-2na | gotchas
Archived: standard programming advice about comparison operators in retry logic. Not project-specific.

### 2026-02-07 | gromit-due | gotchas
Archived: standard Go programming knowledge (cmd.Stdin vs StdinPipe). Not project-specific.

### 2026-02-07 | gromit-3ibd | patterns
Archived: generic software engineering advice about returning measurements from functions. Not project-specific.

### 2026-02-08 | gromit-gh9 | conventions
Archived: normalizeNilFields() already captured in RULES.md Code Style section. Redundant with promoted rule.

### 2026-02-08 | ralph-runner-nekg json omitempty | conventions
Archived: json omitempty backward compatibility already covered by RULES.md Process rule about config fields with omitempty.

### 2026-02-08 | gromit-md3w | conventions
Archived: standard Go naming convention (PascalCase for exported fields). Not project-specific knowledge.

### 2026-02-08 | gromit-m0k | conventions
Archived: general documentation practice (superseded notices). Not specific to this codebase's architecture.

### 2026-02-08 | JSON tag originals | conventions
Archived: gromit-s73k, gromit-0xbs, gromit-goaz, gromit-zxxs consolidated into Confirmed "JSON Serialization Conventions" entry.

### 2026-02-08 | config defaults originals | conventions
Archived: gromit-9ckj, gromit-w971 consolidated into Confirmed "Config Defaults Pattern" entry.

### 2026-02-08 | template infrastructure originals | patterns
Archived: gromit-s7tm, gromit-avbc consolidated into Confirmed "Template Infrastructure" entry.

### 2026-02-08 | runner method originals | patterns
Archived: gromit-5pvp, gromit-82qx, gromit-vabo consolidated into Confirmed "Runner Method Pattern" entry.

### 2026-02-08 | gromit-zt4n | gotchas
Archived: generic engineering advice (test helpers should call setDefaults or document expectations). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-bq2g | patterns
Archived: standard Go testing patterns (t.Run, t.TempDir). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-2cly | patterns
Archived: generic engineering advice (temp files for large args, callback injection for testability). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-hzf6 | conventions
Archived: generic engineering advice (skill file structure). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-8r7z | patterns
Archived: generic engineering advice (reverse-order mutation to avoid index shifting). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-p44z | conventions
Archived: generic engineering advice (JSON omitempty for optional fields). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-8tnl | conventions
Archived: generic engineering advice (minimal interfaces). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-8nfq | patterns
Archived: generic engineering advice (functional options pattern). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-s2va | gotchas
Archived: generic engineering advice (bufio.Scanner vs fmt.Scanln). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-q4l8 | patterns
Archived: generic engineering advice (integration tests check side effects). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-vlk9 | conventions
Archived: generic engineering advice (strings.Builder for conditional sections). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-6x11 | patterns
Archived: generic engineering advice (intermediate accumulator types). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-a9yc | patterns
Archived: generic engineering advice (extract values via dedicated methods). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-plww | patterns
Archived: generic engineering advice (thread return values with blank identifier). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-9at0 | patterns
Archived: generic engineering advice (thread-safe accessors with mutex). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-q224 | conventions
Archived: generic engineering advice (YAML config documentation style). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-c4jr | patterns
Archived: generic engineering advice (guardian flag for crash detection). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | gromit-flt8 | patterns
Archived: generic engineering advice (nil-safety test cases). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | gromit-w9rs | gotchas
Archived: generic engineering advice (optional *bool nil-pointer testing). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | gromit-0azh | patterns
Archived: generic engineering advice (*bool pointer pattern). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | gromit-w3x | conventions
Archived: generic engineering advice (validate downstream consumers can parse). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | ralph-runner-543u | gotchas
Archived: generic engineering advice (test fixtures enforce env vars). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | ralph-runner-nekg | conventions
Archived: generic engineering advice (verify tests pass after struct changes). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | ralph-runner-8ayf | conventions
Archived: generic engineering advice (verify refactored code preserves behavior). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | ralph-runner-zgri | conventions
Archived: generic engineering advice (test new fields in existing tests first). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | Documentary Test Replacement | patterns
Archived: generic TDD advice (replace documentary tests with DI, integration tests make narrow tests redundant). Not project-specific.

*Archived from confirmed: filtered: generic engineering advice*

### 2026-02-08 | Output Formatting | patterns
Archived: generic Go pattern (build via []string joined with newlines, strings.Contains for tests). Not project-specific.

*Archived from confirmed: filtered: generic engineering advice*

### 2026-02-08 | Table-Driven and Concurrent Testing | patterns
Archived: standard Go testing patterns (t.Run, t.Parallel, sync.WaitGroup, defer/recover). Language-level conventions.

*Archived from confirmed: filtered: generic engineering advice*

### 2026-02-08 | Test Failure Root Cause Analysis | conventions
Archived: generic debugging advice (check if test failure is related to current task or pre-existing). Universal practice.

*Archived from confirmed: filtered: generic engineering advice*

### 2026-02-08 | Helper Function Extraction | conventions
Archived: restates basic SRP/DRY (extract small focused helpers, inject rather than reimplement). Not project-specific.

*Archived from confirmed: filtered: generic engineering advice*

### 2026-02-08 | renderer and template consolidation | conventions
Archived: ralph-runner-628c, ralph-runner-nxdm "Renderer Method Pattern" consolidated into Confirmed "Prompt Rendering Architecture" entry alongside Template and Renderer Architecture.

### 2026-02-08 | gromit-4zg | gotchas
Archived: one-off migration issue with bd rename-prefix. Not a recurring pattern.

*Archived from provisional: filtered: one-off issue*

### 2026-02-08 | gromit-yw0x | patterns
Archived: too thin to be actionable (templates use Guidelines sections). Already covered by consolidated Prompt Rendering Architecture learning.

*Archived from provisional: filtered: covered by consolidation*

### 2026-02-08 | gromit-o93z | patterns
Archived: standard Go test patterns (mock command, test success/failure, validate JSON, check nil). Thin project-specific veneer.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | Test Data File Validation | conventions
Archived: gromit-ie2 consolidated into Confirmed "LEARNINGS.md Format Validation" entry. Test data sync requirement now covered by rule (7).

### 2026-02-08 | Template Infrastructure originals | patterns
Archived: gromit-s7tm, gromit-avbc consolidated into Confirmed "Prompt and Template Infrastructure" entry.

### 2026-02-08 | ralph-runner-3kow | conventions
Archived: meta-advice about what makes a good learning entry. Process guidance, not a codebase-specific pattern.

*Archived from provisional: filtered: meta-process advice*

### 2026-02-08 | gromit-9hau | patterns
Archived: already promoted to RULES.md Process section (skippedBeads map pattern). Redundant with rule.

### 2026-02-08 | gromit-elej | conventions
Archived: generic engineering advice (errgroup concurrent tasks, error handling). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-x9bq | conventions
Archived: generic engineering advice (update all interface implementations). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-leha | conventions
Archived: generic engineering advice (propagate config changes across packages). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-j5x0 | patterns
Archived: standard API usage ordering. Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-t5x0 | patterns
Archived: standard go:embed usage. Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | Parallel Post-Success Execution Fragility | conventions
Archived: parallel post-success execution removed entirely. Sequential execution is simpler and eliminates race condition risks.

*Archived from confirmed: feature removed*

### 2026-02-08 | syncWriter Thread Safety | patterns
Archived: parallel post-success execution removed. syncWriter remains but parallel-writes context no longer applies.

*Archived from confirmed: feature removed*

### 2026-02-08 | parallel post-success originals | conventions
Archived: gromit-x9bq, gromit-o2vl, gromit-sliw consolidated into Confirmed "Parallel Post-Success Execution Fragility" entry.

### 2026-02-08 | gromit-uwyu | conventions
Archived: temporal observation about parallel tests appearing flaky. Root causes fixed. Captured in consolidated learning.

*Archived from provisional: filtered: stale observation*

### 2026-02-08 | gromit-kcdt | gotchas
Archived: generic engineering advice (module context for temp directory compilation). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | Shell Safety | gotchas
Archived: already promoted to RULES.md Safety section. Rule is the source of truth.

*Archived from confirmed: promoted to rules*

### 2026-02-08 | LEARNINGS.md Format Validation | conventions
Archived: already promoted to RULES.md Process section. Rule is the source of truth.

*Archived from confirmed: promoted to rules*

### 2026-02-08 | Test Helper Delegation | conventions
Archived: generic DRY advice (delegate to shared testutil packages). Standard software engineering practice.

*Archived from confirmed: filtered: generic engineering advice*

### 2026-02-08 | Dependency Injection for Testability | patterns
Archived: already promoted to RULES.md Code Style section. Rule is the source of truth.

*Archived from confirmed: promoted to rules*

### 2026-02-08 | JSON Serialization Conventions | conventions
Archived: already promoted to RULES.md Code Style section. Rule is the source of truth.

*Archived from confirmed: promoted to rules*

### 2026-02-08 | Config Defaults Pattern | conventions
Archived: already covered by RULES.md Process section. Redundant with rule.

*Archived from confirmed: promoted to rules*

### 2026-02-08 | gromit-qnlq | conventions
Archived: generic engineering advice (errgroup concurrency testing). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-qnlq | gotchas
Archived: generic engineering advice (timing-based concurrency assertions are flaky). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-k9mi | patterns
Archived: generic engineering advice (custom definitions over built-in presets). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-k9mi | patterns
Archived: generic engineering advice (nil checks before config access). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-tjjs | patterns
Archived: generic engineering advice (table-driven tests, verify observable behavior). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-703n | patterns
Archived: generic engineering advice (table-driven tests for config precedence). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-703n | patterns
Archived: generic engineering advice (test by scenario not condition). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-4y8h | patterns
Archived: generic engineering advice (flag validation extraction). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-4y8h | conventions
Archived: generic engineering advice (variadic arguments for backward compat). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-71od | patterns
Archived: one-off timeout observation. Not actionable.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-7guf | patterns
Archived: generic engineering advice (extract common helpers). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-5rpz | patterns
Archived: generic engineering advice (consolidate tests with table-driven patterns). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-9945 | conventions
Archived: generic engineering advice (section divider comments in test files). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-9945 | patterns
Archived: generic engineering advice (extract test helpers, document with comments). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-75qw | patterns
Archived: generic engineering advice (delete acceptance tests that mirror unit tests). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-75qw | conventions
Archived: generic engineering advice (consolidate into table-driven unit tests). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-9qtb | conventions
Archived: generic engineering advice (extract setup helpers in acceptance tests). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-g8j7 | patterns
Archived: one-off timeout observation. Not actionable.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-7esx | conventions
Archived: generic engineering advice (run full test suite after interface changes). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-9idu | conventions
Archived: generic engineering advice (acceptance tests belong with unit tests). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-10 | gromit-9idu | conventions
Archived: generic engineering advice (reclassify test files by purpose). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-10 | gromit-u3z9 | conventions
Archived: generic engineering advice (extract skipped test behaviors to backlog). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-10 | gromit-qnlq | gotchas
Archived: barrier pattern already implemented. Not project-specific.

*Archived from provisional: filtered: stale/served its purpose*

### 2026-02-10 | Agent Selection and Execution | patterns
Archived: already captured as a rule in RULES.md Code Style section. Redundant.

*Archived from provisional: promoted to rules*

### 2026-02-10 | acceptance test convention originals | conventions
Archived: gromit-nf7p, gromit-xeub (x2) consolidated into Confirmed "Acceptance Test File Conventions" entry.

### 2026-02-08 | Compile-time Interface Verification | conventions
*Related to: gromit-l7v4*

Archived: generic Go engineering advice. Already codified as a rule.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-10 | Acceptance Test File Conventions | conventions
*Related to: gromit-nf7p, gromit-xeub*

Archived: restates rules already codified in RULES.md Test Quality section. Redundant.

*Archived from confirmed: redundant with rules*

### 2026-02-10 | gromit-d6sl consolidation originals | conventions
Archived: consolidated into "Refactoring Bead Cascade Risks" entry.

### 2026-02-10 | gromit-gdzl | conventions
Archived: generic engineering advice (use flexible matching in integration tests). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-7guf | conventions
Archived: generic defensive programming. Covered by rules.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-10 | gromit-d6sl, gromit-gdzl consolidation originals | conventions
Archived: consolidated into "Refactoring and Consolidation Bead Hygiene" entry.

### 2026-02-10 | learnings refactoring cascade originals | conventions
Archived: consolidated into "Learnings Package Refactoring Cascades" entry.

### 2026-02-10 | refactoring cascade consolidation | conventions
Archived: consolidated into "Refactoring Bead Cascade Risks" entry.

### 2026-02-10 | Prompt and Template Infrastructure | conventions
*Related to: ralph-runner-utv8, ralph-runner-yx7b, ralph-runner-5lk0, ralph-runner-kjix, ralph-runner-628c, ralph-runner-nxdm, gromit-s7tm, gromit-avbc*

Archived: promoted to RULES.md Code Style section. Rule is the source of truth.

*Archived from confirmed: promoted to rules*

### 2026-02-10 | gromit-66dz | gotchas
Archived: generic SRP advice. Covered by Runner Method Pattern.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-66dz conventions | conventions
Archived: consolidated into "Refactoring Bead Cascade Risks" entry.

*Archived from provisional: consolidated*

### 2026-02-10 | gromit-66dz | gotchas
Archived: generic SRP advice. Redundant with Runner Method Pattern.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-10 | gromit-vbdo | patterns
Archived: covered by RULES.md (normalizeNilFields, zero sentinel). Redundant.

*Archived from provisional: redundant with rules*

### 2026-02-10 | gromit-vbdo | gotchas
Archived: promoted to RULES.md Process section. Rule is the source of truth.

*Archived from provisional: promoted to rules*

### 2026-02-10 | gromit-755e consolidation originals | conventions
Archived: consolidated into "Test-Only Bead Detection Pattern" entry.

*Archived from provisional: consolidated*

### 2026-02-11 | gromit-r3mq | gotchas
Archived: generic engineering advice (.gitignore for build artifacts). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-r3mq | gotchas
Archived: generic engineering advice (gitignore test binaries). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-0ypa | conventions
Archived: generic engineering advice (delete t.Skip test files). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-s7hk | patterns
Archived: generic engineering advice (os.Stat fast path). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-4yrb | gotchas
Archived: generic engineering advice (update all call sites). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-1wen | conventions
Archived: generic engineering advice (keep docs directory-focused). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-lxlp | patterns
Archived: generic engineering advice (extract common logic into helpers). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-d46r | patterns
Archived: generic engineering advice (concise summary with detail below). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-j2p9 | conventions
Archived: generic engineering advice (individual test functions, t.TempDir, t.Fatalf). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-oqtr | conventions
Archived: generic engineering advice (check test reduction targets). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-qpjd | conventions
Archived: generic engineering advice (descriptive test names). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-qpjd | patterns
Archived: generic engineering advice (dedicated test files for features). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-2xrq | conventions
Archived: generic engineering advice (config comments explain rationale). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-nrzg | conventions
Archived: generic engineering advice (cobra command patterns). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-nrzg | patterns
Archived: generic engineering advice (extract formatting helpers). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-w0en | patterns
Archived: generic engineering advice (value types for read-only data). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-lubw | conventions
Archived: generic engineering advice (resolve paths at call sites). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-zyc8 | patterns
Archived: generic engineering advice (acceptance test reduction as hard requirement). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-zyc8 | gotchas
Archived: generic engineering advice (validate CLI mechanics empirically). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-vujh | patterns
Archived: generic engineering advice (provider interface for DI). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-vujh | patterns
Archived: generic engineering advice (self-contained packages). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-cz7a | gotchas
Archived: generic engineering advice (verify actual completion). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-z56a | gotchas
Archived: generic engineering advice (verify metrics thresholds). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-z56a | patterns
Archived: generic engineering advice (thin adapter/delegation pattern). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-z56a | patterns
Archived: generic engineering advice (tier→model mapping in provider layer). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-nqf1 | patterns
Archived: generic engineering advice (provider delegation). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-nqf1 | patterns
Archived: generic engineering advice (converter helpers for domain type reuse). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-2edx | patterns
Archived: generic engineering advice (dual-layer state pattern). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-bu86 | patterns
Archived: generic engineering advice (nil checks, early returns). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-6tyj | conventions
Archived: generic engineering advice (update test mocks for new dependencies). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-mz3m | gotchas
Archived: generic engineering advice (search all usages for interface migration). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-12 | gromit-mz3m | conventions
Archived: generic engineering advice (check acceptance metrics before declaring complete). Not project-specific.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-u1gm | patterns
Archived: generic engineering advice (check both instantiation paths for new fields). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-11 | gromit-qz0m | gotchas
Archived: generic engineering advice (don't call methods for side effects). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-11 | gromit-qz0m | patterns
Archived: generic engineering advice (use constructors for proper initialization). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-11 | gromit-6pvd | gotchas
Archived: generic engineering advice (initialize maps in NormalizeNilFields). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-11 | gromit-6pvd | conventions
Archived: generic engineering advice (JSON tags + NormalizeNilFields for state). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-11 | gromit-c2ax | conventions
Archived: generic engineering advice (update SetDefaults and NormalizeNilFields). Promoted to rule.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-11 | gromit-cz7a | patterns
Archived: generic engineering advice (dedicated mapping functions for backward compat). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-11 | gromit-w0en | patterns
Archived: generic engineering advice (value-to-pointer conversion at call site). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-11 | gromit-6w39 | patterns
Archived: generic engineering advice (format functions return strings with embedded newlines). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-11 | gromit-vvea | gotchas
Archived: generic engineering advice (nil checks when calling functions earlier in lifecycle). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-11 | gromit-d46r | patterns
Archived: generic engineering advice (only include non-zero counts in breakdowns). Not project-specific.

*Archived from provisional: filtered: generic engineering advice*
