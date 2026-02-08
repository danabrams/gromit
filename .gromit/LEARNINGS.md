# Learnings

Accumulated operational knowledge from Gromit iterations.
This file is automatically updated. Review periodically with `gromit retro`.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

### 2026-02-07 | Shell Safety | gotchas
*Related to: m7td, 6rao*

Shell scripts handling user content must use quoted <<'EOF' heredocs to prevent variable/command expansion, and pass dynamic values via arguments rather than string interpolation. Both heredoc quoting and argument-passing are required for injection safety.
**Promoted to RULES.md Safety section.**

### 2026-02-07 | Mock Implementation Patterns | patterns
*Related to: ge6j, atmb*

Mock implementations use optional function pointer fields (FnField pattern) with nil-safe defaults. Tests set up only the behavior needed via tracking flags or injected callbacks, enabling focused verification of specific code paths without requiring full mock setup.

### 2026-02-07 | Status File Management | patterns
*Related to: nalr, k8c2, kydj, ead1, xpfn, lm34, 2y2d, yj2h, vpyl, kim2*

Status struct fields require backward-compatible changes (omitempty for new optional fields). Use ReadStatus()/IsProcessAlive() for state + liveness checks. Return nil,nil for missing optional files (not an error). StatusWriter handles both lifecycle states and preserves completed iteration count on shutdown. Round-trip tests verify serialization fidelity. Stale resource cleanup integrates into status reporting via process liveness checks. Process utilities (IsProcessAlive) are co-located with Status in status.go. Test file I/O uses t.TempDir() for isolation.

### 2026-02-07 | LEARNINGS.md Format Validation | conventions
*Related to: gromit-8a52, gromit-i0wm, gromit-wgtu, gromit-2v5n, gromit-49dg, gromit-8et, gromit-3gb, gromit-kim, gromit-due, gromit-1u7, gromit-evq, gromit-w3x, gromit-ltg, gromit-knc, gromit-3fqu, gromit-3ibd, gromit-hvbu, gromit-fntb, gromit-7e5o, gromit-2qf1, gromit-yew, gromit-ie2*

LEARNINGS.md requires strict pipe-delimited header format: `### YYYY-MM-DD | DESCRIPTIVE_TITLE | CATEGORY_NAME`. Rules: (1) Date must be a valid current/past date, never 0001-01-01. (2) BeadID field must contain the full descriptive learning title, not category names. (3) Category must be one of: gotchas, conventions, patterns. (4) Consolidated entries must include source bead IDs in a separate `*Related to: id1, id2*` line in the content body, not in the header. (5) Integration test TestLoadActualLearningsFile validates this format strictly — malformed headers silently produce zero-value dates that are hard to debug. (6) When consolidating learnings, validate output format before persisting. (7) Data files tested by integration tests must conform to the validated schema — test fixtures and actual data files must stay in sync. When prefix renames or ID migrations occur, update both code references and documentation/metadata files.
**Promoted to RULES.md Process section.**

### 2026-02-07 | Test Helper Delegation | conventions
*Related to: rkub, ewqu, uucn, y0v3*

Test helpers delegate to shared testutil packages rather than duplicating logic. Use t.Fatalf for error handling when adapting function signatures with different return counts. Handle optional parameters using zero-value checks (empty string for dir, nil for environ) rather than pointers. E2E test files delegate to testutil package helpers (e.g., testutil.PickerStdin) rather than hardcoding raw strings.

### 2026-02-08 | Prompt and Template Infrastructure | conventions
*Related to: ralph-runner-utv8, ralph-runner-yx7b, ralph-runner-5lk0, ralph-runner-kjix, ralph-runner-628c, ralph-runner-nxdm, gromit-s7tm, gromit-avbc*

Prompt/template infrastructure follows a load-populate-render pattern with consistent conventions: (1) Templates are named PROMPT_<name>.md, registered in runInit()'s templates map, with constants named defaultXxxTemplate in init.go. (2) Renderer methods accept a context struct (with Bead and ParentBead fields), call r.tmpl.ExecuteTemplate, and return (string, error). (3) Template variants reuse common context sections (Rules, Learnings, Task, Spec, Parent) and customize only Instructions and Completion sections. (4) TemplateContext struct fields are populated from data sources via dedicated load/read methods, then passed to template rendering. (5) FuncMap functions (sub, mul, div) enable templates to compute deltas and percentages inline. (6) New context types and render methods should follow existing patterns like ScopeContext, DecomposeContext, PrecheckContext.

### 2026-02-07 | Methodology Label Activation | patterns
*Related to: ralph-runner-4a3f, ralph-runner-nzue*

Methodologies use label-based activation ("methodology:true"/"false") with global config fallback via bead.IsMethodologyActive(). When active, replace the build prompt with a specialized RenderXXXBuild method. Check parent labels before adding globally-active methodology labels to sub-beads to avoid duplicates. Order methodology checks carefully for precedence when multiple methodologies are active.

### 2026-02-07 | Dependency Injection for Testability | patterns
*Related to: ralph-runner-tizz, ralph-runner-tsf4*

Mock implementations in *_test.go require adding Fn fields and calling them in mock methods, mirroring existing Render* patterns. CLI functions with user prompts or subprocess calls should accept these as injected function parameters for testability. When adding new interface methods, add corresponding mock fields and verify against existing mock patterns.
**Promoted to RULES.md Code Style section.**

### 2026-02-07 | syncWriter Thread Safety | patterns
*Related to: ralph-runner-qcoc, ralph-runner-t13e, ralph-runner-z6li*

Wrapper types around io.Writer use sync.Mutex for thread safety and track state (like lastWasOverwrite) for mode transitions. Store concrete wrapper types as struct fields (not just interfaces) when wrapper-specific methods are needed. syncWriter.WriteOverwrite() handles newline transitions automatically — don't manually add newlines around calls.

### 2026-02-08 | JSON Serialization Conventions | conventions
*Related to: gromit-s73k, gromit-0xbs, gromit-goaz, gromit-zxxs*

JSON struct tags in this codebase use snake_case field names (input_tokens, cost_usd). All serialized fields must have explicit JSON tags — omitting tags causes fields to be excluded from output. Use `omitempty` only for optional fields; omit it when the field should always be present in output.
**Promoted to RULES.md Code Style section.**

### 2026-02-08 | Config Defaults Pattern | conventions
*Related to: gromit-9ckj, gromit-w971*

Config defaults use zero-value checking in setDefaults(): `if field == 0` means 'not configured'. Nested structs in YAML configs use `yaml:"field_name"` tags, and defaults for nested fields are set in the parent's setDefaults() after the struct is populated. Zero is the sentinel for 'not configured' for int fields.

### 2026-02-08 | Runner Method Pattern | patterns
*Related to: gromit-5pvp, gromit-82qx, gromit-vabo*

Runner methods follow a consistent pattern: nil-safe receiver/config checks, feature-flag gating (e.g., IsAutoPushEnabled()), context.WithTimeout for subprocess calls, and failure handling via mode field (warn vs stop) or skippedBeads map rather than inline error handling. The Run() method follows a clear sequencing: validate -> execute work -> persist state -> run between-iteration hooks -> continue loop. Log warnings on timeout errors rather than returning early.

---

## Provisional

*Seen once - may be specific to one task.*

### 2026-02-08 | gromit-j5x0 | patterns
Filter must be set on learnings.File via SetFilter() immediately after NewFile() and before Load() or Add(), to ensure filtering applies to all subsequent operations

### 2026-02-08 | gromit-t5x0 | patterns
Skills are embedded as package-level variables using go:embed with filesystem patterns like //go:embed gromit-*/SKILL.md, paired with exported string variables like SkillName to make them accessible to the rest of the codebase

---

## Archived

*No longer relevant or superseded.*

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

