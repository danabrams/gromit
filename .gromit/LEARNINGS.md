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

---

## Provisional

*Seen once - may be specific to one task.*

### 2026-02-11 | gromit-rpne | conventions
Acceptance tests for prompt template changes must be kept in sync with the actual template updates. When updating templates like PROMPT_decompose.md, ensure test expectations match the exact content being added, including specific phrases and subsection structure. Also remember to add `//go:build acceptance` tag to acceptance test files in this codebase.

### 2026-02-11 | gromit-rpne | conventions
Prompt templates in .gromit/templates/ use explicit section headers (##) and preserve exact whitespace/structure when updating; when modifying sections, maintain blank lines between sections and ensure ATDD blocks remain unchanged

### 2026-02-11 | gromit-rpne | conventions
Template files in .gromit/templates/ follow a consistent structure: context section at top, then Guidelines, then preserved sections like 'Avoiding Sibling Overlap' and ATDD blocks—preserve exact line ranges when updating guidelines to avoid disrupting downstream sections

### 2026-02-11 | gromit-o4ow | patterns
Test cases in learnings_test.go use subtests with t.Run and table-driven patterns, with mock implementations via testutils package providing controlled state for archived duplicate detection

### 2026-02-11 | gromit-lxlp | conventions
In this codebase, acceptance tests (build tag: //go:build acceptance) are subject to strict line count audits - total across all files must not exceed a fixed threshold. New test code should use unit tests for API verification, not acceptance tests. When adding new methods, place comprehensive tests in regular *_test.go files unless testing actual bd CLI integration/behavior.

### 2026-02-11 | gromit-03lk | conventions
When expanding a struct with new fields in the pipeline package, you must: (1) update ReadStatus() to populate the new count fields by iterating through beads and counting by status, (2) update the formatting/display logic (likely in runner/format_bead_breakdown.go) to include these new counts in status output, and (3) ensure any helper methods for counting beads are called correctly. The count fields won't be used automatically—they require explicit population and display logic.

### 2026-02-11 | gromit-d46r | patterns
When building comma-separated status breakdowns, only include non-zero counts and provide a fallback value ('none') when all counts are zero; use conditional logical operators (if > 0) to filter parts before joining to avoid empty entries

### 2026-02-11 | gromit-j2p9 | patterns
Test files for log parsing use synthetic JSONL with varying field completeness to document backward compatibility—test both full-format and minimal-format entries to prevent regressions when log schema evolves

### 2026-02-11 | gromit-oqtr | patterns
When adding a field to a logging structure (IterationLog), the field must be wired through three layers: capture it in the execution layer (executeClaudeInvocation extracts from DiagnosticSnapshot), store it in the intermediate result struct (IterationResult), and propagate it when writing the final log entry (writeIterationLog). Missing any layer breaks the data flow to JSONL output.

### 2026-02-11 | gromit-vvea | gotchas
When moving a function call earlier in a lifecycle (e.g., escalateModel called preemptively in setupBeadContext before promptCtx exists), defensive nil checks are required for fields initialized later. escalateModel must check `if bc.promptCtx != nil` before updating it.

### 2026-02-11 | gromit-2xrq | conventions
Configuration files in gromit.yaml use inline comments (after values) to explain the rationale and trade-offs for each setting, not just what the setting does—comments include why a value differs from defaults and what it optimizes for (e.g., 'longer invocation timeout — sonnet consistently needs >900s')

### 2026-02-11 | gromit-eo57 | gotchas
Aggregation functions in logger package often duplicate logic rather than compose (e.g., ReadModelStats and ReadRunModelStats have nearly identical implementations). When adding similar per-model aggregation functions, consider whether refactoring to a filtered helper would reduce duplication, but follow existing patterns if choosing direct implementation.

### 2026-02-11 | gromit-eo57 | patterns
Aggregation functions follow a consistent pattern: glob run-*.jsonl files, optionally filter by extracted run ID, read with readLogFile(), then iterate entries building maps/accumulators. Reuse helper functions (extractRunID, readLogFile) across multiple aggregation modules to avoid duplication.

### 2026-02-11 | gromit-jsta | patterns
Atomic file updates in logger package follow read-modify-write with temp-file-then-rename: ReadGlobalStats handles missing files gracefully (returns initialized empty state, not error), UpdateGlobalStats merges data then writes atomically via CreateTemp+Rename, with defer cleanup. Apply this pattern to other file-based aggregations.

### 2026-02-11 | gromit-6w39 | patterns
All format functions return single strings with embedded newlines (built by appending to []string and joining). Handle nil/empty inputs with early returns showing '(no data)'. Use 2-space indentation for all sub-items and consistent section headers like 'Section Name:' followed by indented details.

### 2026-02-11 | gromit-w0en | patterns
When passing read-only data structures between functions, convert value types to pointer types at the call site (not in the return type) to allow formatters to handle both nil and zero-initialized cases uniformly

### 2026-02-11 | gromit-zyc8 | patterns
Spike tasks document CLI mechanics and behavior in markdown files (FINDINGS.md) that serve as reference for future implementation tasks; acceptance tests verify the documented behavior matches reality

### 2026-02-11 | gromit-7oob | patterns
Mapping functions in provider package use local map lookups with case-insensitive matching (strings.ToLower) and pass-through returns for unrecognized values to maintain forward compatibility with new model names

---

## Archived

*No longer relevant or superseded.*

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
CLAUDE.md is the authoritative source for project conventions and should be updated when patterns change; keep the architecture section directory-based rather than file-specific to reduce maintenance burden

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
Test helpers like NewRunnerWithDeps that create partial configs should either call setDefaults() or document that callers must explicitly set all needed config fields to avoid accidental defaults

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-bq2g | patterns
Use table-driven tests with t.Run() and temp directories created via t.TempDir() for file-based unit tests - this pattern is established in the codebase for testing functions that interact with the filesystem

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-2cly | patterns
When launching interactive subprocesses with large context (prompts, system messages), write to a temp file and pass the file path as an argument to avoid ARG_MAX errors. Use callback injection (confirmPrompt, execGromit functions) for testability of interactive chaining flows that invoke subprocesses and parse side effects.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-hzf6 | conventions
Skill files use SKILL.md naming convention with structured sections: Purpose, When to Use, How It Works, Investigation Report Format, and Example Output to match the pattern seen in other skills like superpowers:systematic-debugging

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-8r7z | patterns
When filtering/modifying slice items conditionally, collect target indices first, then apply mutations in reverse order to avoid index shifting bugs

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-p44z | conventions
State struct fields use JSON tags with 'omitempty' for optional fields to handle serialization of empty slices/maps gracefully

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-8tnl | conventions
Interface definitions in this codebase are minimal and focused—ClaudeRunner only defines the single method needed (Run), avoiding over-specification. Define interfaces with the smallest necessary surface area.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-8nfq | patterns
Functional options pattern with SetXxx() methods is used for configuring struct fields (e.g., SetFilter()) rather than passing parameters to constructors or modification functions

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-s2va | gotchas
fmt.Scanln() stops at whitespace and doesn't read full lines with spaces; use bufio.Scanner with Scan() + Text() for line-based input that preserves spaces

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-q4l8 | patterns
Integration-style tests verify complete scenarios by checking side effects (mock state changes, logs, output) alongside primary assertions, ensuring behavior is traceable through the entire system

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-vlk9 | conventions
For conditional multi-section prompt/output building, use strings.Builder with conditional WriteString calls instead of fmt.Sprintf. This makes section logic clearer and allows each section to be conditionally included based on whether supporting data exists.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-6x11 | patterns
Use intermediate accumulator helper types for multi-level aggregations—collect raw totals during iteration, then convert via dedicated methods. This separates accumulation from final calculations and makes weighted averages (like cost/duration per bead across models) testable and less error-prone.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-a9yc | patterns
Extract values from intermediate results using dedicated methods (like stats.CostData()) and assign them directly to destination struct fields, rather than passing the intermediate object through the chain. This simplifies dependency flow and makes data transformation points explicit.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-plww | patterns
Thread new return values through function signatures first using blank identifier (_), then add consumers in follow-up tasks. This separates infrastructure changes from behavioral changes.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-9at0 | patterns
Thread-safe accessors for mutable shared state use lock/unlock around field reads/writes and nil-check the receiver first, following the pattern: if s == nil { return } followed by s.mu.Lock/defer s.mu.Unlock

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-q224 | conventions
YAML config sections consistently document defaults with 'Default: X' format, explain when/why users would customize fields, and cross-reference related implementation details in comments to aid troubleshooting

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-c4jr | patterns
Use a guardian flag (set false at operation start, true at clean exit) as the primary crash detector, with timestamp age as a secondary signal. Auto-heal should reset unreliable state while preserving git anchors (commits, historical timestamps).

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | gromit-flt8 | patterns
Table-driven tests for runner methods include explicit nil-safety cases with dedicated flags (nilRunner, nilConfig) to verify graceful nil-handling; use description field for readability alongside test name

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | gromit-w9rs | gotchas
Optional boolean fields (*bool) in config require separate unit tests for nil-pointer safety alongside table-driven YAML tests; nil defaults to true is a gotcha worth isolating in dedicated tests

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | gromit-0azh | patterns
Helper methods like IsAutoPushEnabled() follow the *bool pointer pattern with explicit enabled check: return b != nil && *b

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | gromit-w3x | conventions
When validating skill extraction, verify that downstream consumers (like learnings loader) can properly parse the injected content. Test failures in dependent systems indicate the validation point may be in the wrong layer or the format contract needs to be checked.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | ralph-runner-543u | gotchas
Test fixtures in fake scripts should enforce required environment variables (e.g., TEST_DIR) rather than falling back to /tmp, and cleanup code should be idempotent and run even if tests fail (consider using trap handlers or defer statements)

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | ralph-runner-nekg | conventions
When adding fields to structs that are serialized/deserialized (especially JSON), verify that existing tests that depend on that struct's behavior still pass—including contract tests that exercise external tool integration.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | ralph-runner-8ayf | conventions
When helpers are consolidated into a shared package, verify that the refactored code doesn't change behavior of callers—the tests should catch regressions, but pre-existing test failures in a codebase can hide whether consolidation was successful.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-07 | ralph-runner-zgri | conventions
Test new fields by adding assertions to the existing main test first, then create a dedicated test if the field requires special or isolated validation

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
When using errgroup for concurrent task execution in Go, ensure the tasks themselves are not blocking. The extractSuccessLearning() function may have context-based timeouts or synchronous I/O that prevents true concurrency. Additionally, error handling in concurrent code requires careful management - using errgroup.Wait() with ignored error returns and deferred error handling can cause errors to be silently dropped. Test concurrent execution with actual timing measurements and verify error propagation through the goroutines.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-x9bq | conventions
When adding new methods to interfaces (BeadClient in this case), ensure that: (1) all implementations of that interface are updated, not just the interface definition; (2) run the full test suite before considering a task complete, as interface changes can have cascading effects on mock implementations and tests; (3) parallel execution and timing-sensitive tests are brittle and failures may indicate the new code wasn't properly integrated into the concurrent execution paths.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-leha | conventions
When adding new configuration structs (like AgentsConfig), ensure that all code that depends on that configuration is updated to read and use it. Configuration changes often require coordinated updates across multiple packages. Test failures in different areas (runner tests + contract tests) suggest incomplete propagation of the new config fields.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-j5x0 | patterns
Archived: describes standard API usage ordering (call SetFilter before Load/Add). Generic 'call init before use' advice — obvious from function signatures and not a surprising gotcha.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-08 | gromit-t5x0 | patterns
Archived: describes standard go:embed usage with filesystem patterns. One-time implementation detail obvious from reading the code.

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
Archived: temporal observation about parallel tests appearing flaky. Root causes fixed in commits d11a43b (barrier patterns) and cc053dd (syncWriter races). Substantive content captured in consolidated parallel post-success learning.

*Archived from provisional: filtered: stale observation*

### 2026-02-08 | gromit-kcdt | gotchas
When generating and compiling Go code in temporary/subdirectories during tests, either: (1) copy go.mod/go.sum to the temp directory, (2) use GOPATH mode, or (3) compile from the parent module root with proper -C or working directory flags. bd contract tests need module context for generated programs to resolve internal imports.

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
Archived: already covered by RULES.md Process section (config defaults with setDefaults() and zero-value sentinels). Redundant with rule.

*Archived from confirmed: promoted to rules*

### 2026-02-08 | gromit-qnlq | conventions
When testing concurrent execution with errgroup in Go, ensure that goroutines are actually spawned and executed in parallel. Sequential execution can occur if goroutines block on synchronization primitives, if stages are not properly spawned in separate goroutines, or if the errgroup is not correctly configured to run tasks concurrently. Tests should use barriers or sync points rather than timing-based assertions to verify concurrency.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-qnlq | gotchas
Timing-based concurrency assertions in tests are inherently flaky. Use synchronization primitives (barriers, channels, mutexes, or atomic operations) to verify concurrent execution rather than measuring elapsed time. The concurrent execution may be correct but still fail due to system variance.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-k9mi | patterns
Config merging pattern: check custom user definitions first, then fall back to built-in presets. This ensures user customizations override framework defaults without duplicating logic.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-k9mi | patterns
Custom definitions take precedence over built-in presets by checking them first in resolution order; defensive nil checks precede all config field access to prevent panics

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-tjjs | patterns
Use table-driven tests with t.Run() for complex scenarios and verify observable behavior (output written, error states) not just return values

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-703n | patterns
Test table-driven tests with t.Run for each case, using subtests to organize related test scenarios. When testing functions that handle configuration precedence (flag > phase config > defaults), structure tests to verify each priority level separately with clear assertions on which value wins.

*Archived from new: filtered: generic engineering advice*

### 2026-02-08 | gromit-703n | patterns
Use table-driven tests with t.Run() for testing priority resolution logic; organize tests by testing scenario (priority levels, edge cases, full chain) rather than individual conditions to catch interaction bugs between priority levels

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-4y8h | patterns
Flag validation logic should be extracted to dedicated functions in domain packages with comprehensive parametrized tests covering edge cases like whitespace handling and all pairwise flag combinations

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-4y8h | conventions
Flag validation functions should use variadic arguments (e.g., param ...string) for optional parameters to allow backward-compatible function calls with different argument counts without breaking existing callers

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-71od | patterns
Bead 'Add epic and spec flags to retro command' timed out on sonnet — may need simpler scope or higher model tier

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-7guf | patterns
Extract common helper functions (like listMarkdownFiles) to eliminate duplication between similar operations across different commands - both getEpicFiles and getSpecFiles now delegate to the same implementation, ensuring consistent behavior and reducing maintenance burden

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-5rpz | patterns
Consolidate related acceptance tests using: (1) extract common setup into helper functions, (2) convert multiple test functions testing similar behavior into a single table-driven test with t.Run() subtests

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-9945 | conventions
When consolidating related acceptance test files, use section divider comments (// --- Category (from source_file.go) ---) to organize tests by purpose and origin, improving navigation and maintainability of the consolidated file

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-9945 | patterns
When consolidating multiple test files, extract common setup helpers (writeSpecFiles, writeIterationLogs, assertLabelSet) and document with section comments ("--- Test helpers ---") to reduce duplication and make test organization clear

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-75qw | patterns
Acceptance tests that mirror unit tests should be deleted—unit tests (especially table-driven) are sufficient. Extract common test setup into helper functions (setupLabelFilterTest) to enable table-driven testing and reduce duplication.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-75qw | conventions
Acceptance tests that mirror unit test scenarios should be consolidated into table-driven unit tests with shared setup helpers (setupLabelFilterTest) and the redundant acceptance test files deleted entirely—unit tests in focused format are more maintainable and provide equivalent coverage.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-9qtb | conventions
Acceptance test files with //go:build acceptance tag should extract common setup into helper structs and functions (like hashEvictionEnv with setupHashEviction) to reduce duplication across multiple test cases

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-g8j7 | patterns
Bead 'Handle edge cases in epic status command' timed out on sonnet — may need simpler scope or higher model tier

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-7esx | conventions
When updating interfaces or mock implementations, run the full test suite first to establish baseline failures, then verify that interface changes don't cause cascading test failures in other packages that depend on the mocks. Mock-related changes have broad impact across the codebase.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-9idu | conventions
Acceptance tests that verify contracts against external tools (like bd CLI) belong in main test files with unit tests, not in separate *_acceptance_test.go files. Use build tags (e.g., // +build BD_AVAILABLE) to gate integration tests within the same file.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-10 | gromit-9idu | conventions
Acceptance tests should be reclassified by purpose and merged into appropriately named test files (*_integration_test.go for bd contract/environment-gated tests, *_test.go for functional tests) rather than maintaining separate *_acceptance_test.go files to reduce file proliferation and organize tests by their actual testing strategy

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-10 | gromit-u3z9 | conventions
Skipped acceptance tests document future features; when a test file is entirely skipped, extract the described behaviors to backlog items via gromit add before deleting the dead code

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-10 | gromit-qnlq | gotchas
Archived: barrier pattern for concurrency tests already implemented. Core advice (use sync primitives not timing) is standard testing practice. Learning served its purpose.

*Archived from provisional: filtered: stale/served its purpose*

### 2026-02-10 | Agent Selection and Execution | patterns
Archived: already captured verbatim as a rule in RULES.md Code Style section (agent.Resolve() + agent.Launch() pattern). Redundant with rule.

*Archived from provisional: promoted to rules*

### 2026-02-10 | acceptance test convention originals | conventions
Archived: gromit-nf7p, gromit-xeub (x2) consolidated into Confirmed "Acceptance Test File Conventions" entry.

### 2026-02-08 | Compile-time Interface Verification | conventions
*Related to: gromit-l7v4*

Use compile-time interface verification with `var _ InterfaceName = (*impl)(nil)` at the top of interface files to catch implementation drift early instead of runtime tests — see internal/runner/interfaces.go for the pattern.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-10 | Acceptance Test File Conventions | conventions
*Related to: gromit-nf7p, gromit-xeub*

Archived: restates rules already codified in RULES.md Test Quality section (//go:build acceptance tags, file naming). No project-specific insight beyond what the rules capture.

*Archived from confirmed: redundant with rules*

### 2026-02-10 | gromit-d6sl consolidation originals | conventions
Archived: two gromit-d6sl provisional learnings consolidated into single "Consolidation beads require strict scope verification" entry.

### 2026-02-10 | gromit-gdzl | conventions
When consolidating learnings and archiving entries, existing integration tests that hardcode expected learning positions break immediately. Always verify that learnings integration tests use flexible matching (check for minimum count and validate presence of expected items in any order) rather than positional assertions.

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-7guf | conventions
Archived: generic defensive programming advice (return empty lists for missing optional files/directories). Already covered by Status File Management confirmed learning and normalizeNilFields() rule.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-10 | gromit-d6sl, gromit-gdzl consolidation originals | conventions
Archived: gromit-d6sl, gromit-gdzl consolidated into Provisional "Refactoring and Consolidation Bead Hygiene" entry.

### 2026-02-10 | learnings refactoring cascade originals | conventions
Archived: gromit-uwuh, gromit-w0lo consolidated into Provisional "Learnings Package Refactoring Cascades" entry.

### 2026-02-10 | refactoring cascade consolidation | conventions
Archived: "Refactoring and Consolidation Bead Hygiene" (gromit-d6sl, gromit-gdzl) and "Learnings Package Refactoring Cascades" (gromit-uwuh, gromit-w0lo) consolidated into Provisional "Refactoring Bead Cascade Risks" entry.

### 2026-02-10 | Prompt and Template Infrastructure | conventions
*Related to: ralph-runner-utv8, ralph-runner-yx7b, ralph-runner-5lk0, ralph-runner-kjix, ralph-runner-628c, ralph-runner-nxdm, gromit-s7tm, gromit-avbc*

Archived: promoted to RULES.md Code Style section. Rule is the source of truth.

*Archived from confirmed: promoted to rules*

### 2026-02-10 | gromit-66dz | gotchas
When a function like escalateModel() is designed to handle state changes with side effects (logging, flag setting), all code paths that perform that operation should route through it rather than duplicating the logic—direct assignments bypass the intended behavior

*Archived from new: filtered: generic engineering advice*

### 2026-02-10 | gromit-66dz conventions | conventions
Archived: consolidated into Provisional "Refactoring Bead Cascade Risks" entry point (5).

*Archived from provisional: consolidated*

### 2026-02-10 | gromit-66dz | gotchas
Archived: generic SRP advice (route state changes through single function). Escalation routing already captured in Runner Method Pattern confirmed learning.

*Archived from provisional: filtered: generic engineering advice*

### 2026-02-10 | gromit-vbdo | patterns
Archived: non-zero value precedence and NormalizeNilFields() already codified in RULES.md (Code Style: normalizeNilFields after unmarshaling; Process: zero is sentinel for 'not configured'). Redundant with rules.

*Archived from provisional: redundant with rules*

### 2026-02-10 | gromit-vbdo | gotchas
Archived: promoted to RULES.md Process section (config file compliance testing). Rule is the source of truth.

*Archived from provisional: promoted to rules*

### 2026-02-10 | gromit-755e consolidation originals | conventions
Archived: gromit-755e patterns and gromit-755e conventions consolidated into Provisional "Test-Only Bead Detection Pattern" entry.

*Archived from provisional: consolidated*

### 2026-02-11 | gromit-r3mq | gotchas
Use .gitignore to prevent accidentally committing build artifacts (*.test binaries, *.o files, etc.) - Go test binaries are auto-generated and should never be in version control

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-r3mq | gotchas
Go test binaries (*.test files) should be added to .gitignore to prevent accidental commits of large binary artifacts

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-0ypa | conventions
Test files with t.Skip() on all tests are violations of RULES.md and should be deleted entirely rather than kept as placeholders — the codebase prefers removing untestable code over maintaining skipped test suites

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-s7hk | patterns
When validating specific file existence, check with os.Stat() first for fast path; only call os.ReadDir() when file is missing to list alternatives for user-friendly error messages, avoiding unnecessary directory scans on success

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-4yrb | gotchas
Function signature changes that add parameters should be updated at all call sites in the same file; use grep to verify all callers are updated before testing.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-1wen | conventions
CLAUDE.md is the source of truth for project conventions and should be updated when patterns change, not when individual files are added. Keep the architecture section directory-focused rather than file-focused to avoid constant updates.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-lxlp | patterns
Extract common logic into helper functions when adding similar methods to Client. Multiple Count* methods should delegate to a shared countBeads() helper that handles bd invocation, JSON parsing, and empty-result handling.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-d46r | patterns
The formatPipeline function uses a pattern where multiple conditional fields are combined into a single display line, with detailed lists preserved on separate lines below—this keeps the summary concise while retaining full information.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-j2p9 | conventions
Tests use individual test functions with simple direct assertions rather than table-driven tests; use t.TempDir() for temporary files and t.Fatalf() for setup errors

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-oqtr | conventions
When a task requires adding new functionality with acceptance tests, check if there's an explicit test reduction target or acceptance test line limit in the validation criteria. Large test suites may require consolidating, removing redundant, or refactoring existing tests to stay within budgets. The gromit codebase appears to track acceptance test line counts as a quality metric to prevent test bloat.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-qpjd | conventions
Test functions use descriptive underscore-separated names indicating the specific scenario being tested (e.g., TestSetupBeadContext_PreemptiveEscalationP1HighComplexity, TestSetupBeadContext_NoEscalationWhenScopeCheckDisabled) rather than generic names, making each test's purpose immediately clear without reading the test body

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-qpjd | patterns
Consolidate related test scenarios for a feature into a dedicated, focused test file (e.g., preemptive_escalation_test.go) rather than scattering them across general test files like process_test.go—this improves maintainability and reduces duplicate test code

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-2xrq | conventions
Configuration inline comments should explain the rationale (why) using keywords like 'needs', 'consistently', 'longer', 'complex', 'prevent' — not just the value itself (what). Tests can verify this by searching for rationale keywords in comments.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-nrzg | conventions
Cobra commands follow a consistent pattern: define flags at package level with var declarations, initialize the command struct in init(), and use PersistentPreRunE for validation/setup that applies across command variations. Always check existing similar commands (like debug.go) for the exact registration and error-handling patterns.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-nrzg | patterns
When formatting output with multiple conditional sections, extract each section (calculations, visibility checks, formatting) into dedicated helper functions—this reduces duplication when the same components appear across different output modes or stat types (e.g., printModelLine handles all model stats formatting, printEscalations handles conditional escalation display)

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-w0en | patterns
When wiring new data into Status() output sections, accept value types (not pointers) in formatting functions to avoid allocation overhead on every call; Reader functions should log warnings but continue on error since display is informational

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-lubw | conventions
Logger functions are called with resolved filesystem paths (os.UserHomeDir()) rather than relative paths; always resolve home directory paths at call sites, not inside logger functions

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-zyc8 | patterns
Acceptance test line count reduction is a hard requirement for task completion. The spike task was meant to reduce acceptance test bloat by documenting findings and shifting validation responsibility. Ensure acceptance tests are consolidated or removed when shifting to provider-based validation, rather than just adding new provider code alongside existing tests.

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-zyc8 | gotchas
Spike tasks should validate CLI mechanics empirically by executing actual commands and examining output/exit codes rather than reading docs—discovered that `codex` requires explicit model selection via flags (not env vars), and accepts stdin for prompts

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-vujh | patterns
Define interfaces for external service integrations in a separate provider package to enable clean dependency injection and testing with mock implementations

*Archived from new: filtered: generic engineering advice*

### 2026-02-11 | gromit-vujh | patterns
Internal packages define their own interfaces and types rather than importing from external packages - each package is self-contained with explicit handler type definitions that match external signatures rather than reusing them

*Archived from new: filtered: generic engineering advice*

