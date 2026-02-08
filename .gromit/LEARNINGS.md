# Learnings

Accumulated operational knowledge from Gromit iterations.
This file is automatically updated. Review periodically with `gromit retro`.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

### 2026-02-06 | Shell Safety | gotchas
*Related to: m7td, 6rao*

Shell scripts handling user content must use quoted <<'EOF' heredocs to prevent variable/command expansion, and pass dynamic values via arguments rather than string interpolation. Both heredoc quoting and argument-passing are required for injection safety.
**Promoted to RULES.md Safety section.**

### 2026-02-06 | Documentary Test Replacement | patterns
*Related to: dpk5, prk6, q2id, l0ka*

Replace documentary tests that manually simulate behavior with real tests using dependency injection of callbacks. Include structural property tests (e.g., unmarshal round-trips) to verify extraction fidelity. Integration tests that cover end-to-end flows make narrow documentary tests redundant.

### 2026-02-06 | Mock Implementation Patterns | patterns
*Related to: ge6j, atmb*

Mock implementations use optional function pointer fields (FnField pattern) with nil-safe defaults. Tests set up only the behavior needed via tracking flags or injected callbacks, enabling focused verification of specific code paths without requiring full mock setup.

### 2026-02-06 | Status File Management | patterns
*Related to: nalr, k8c2, kydj, ead1, xpfn, lm34, 2y2d, yj2h, vpyl, kim2*

Status struct fields require backward-compatible changes (omitempty for new optional fields). Use ReadStatus()/IsProcessAlive() for state + liveness checks. Return nil,nil for missing optional files (not an error). StatusWriter handles both lifecycle states and preserves completed iteration count on shutdown. Round-trip tests verify serialization fidelity. Stale resource cleanup integrates into status reporting via process liveness checks. Process utilities (IsProcessAlive) are co-located with Status in status.go. Test file I/O uses t.TempDir() for isolation.

### 2026-02-06 | Output Formatting | patterns
*Related to: 5xbi, 88fk, 9ntm*

Format functions build output via []string slice joined with newlines. Duration formatting composes by delegating to formatDuration() then appending context like " ago". Use strings.Contains() for multi-line output tests; exact equality for single-value formatters.

### 2026-02-06 | LEARNINGS.md Format Validation | conventions
*Related to: gromit-i0wm, gromit-wgtu, gromit-2v5n*

Consolidated learning about strict LEARNINGS.md format validation requirements and consolidation processes. Details covered in gromit-2v5n entry below.

### 2026-02-06 | Test Helper Delegation | conventions
*Related to: rkub, ewqu, uucn, y0v3*

Test helpers delegate to shared testutil packages rather than duplicating logic. Use t.Fatalf for error handling when adapting function signatures with different return counts. Handle optional parameters using zero-value checks (empty string for dir, nil for environ) rather than pointers. E2E test files delegate to testutil package helpers (e.g., testutil.PickerStdin) rather than hardcoding raw strings.

### 2026-02-07 | gromit-ie2 | conventions
*Related to: gromit-ie2*

When tests validate real data files (like LEARNINGS.md), verify that the file structure matches test expectations before implementing code changes. Test fixtures and actual data files must stay in sync - if a test reads real files, those files must conform to the validated schema. Data consolidation processes must update both the data AND any dependent tests that validate that data's structure.

---

## Provisional

*Seen once - may be specific to one task.*

### 2026-02-07 | ralph-runner-4a3f | patterns
Label injection for sub-beads should check parent labels first before adding globally-active methodology labels, to avoid duplicates and respect parent specifications

### 2026-02-07 | ralph-runner-utv8 | conventions
Prompt templates follow a consistent architecture: renderer methods call r.render("PROMPT_<name>.md", ctx) with a naming convention matching the method suffix. Template variants reuse common context sections (Rules, Learnings, Task, Spec, Parent) and only customize Instructions and Completion sections for methodology-specific workflows.

### 2026-02-07 | ralph-runner-nzue | patterns
Methodologies in this codebase use label-based activation ("methodology:true"/"false") with global config fallback via bead.IsMethodologyActive(). When active, replace the build prompt with a specialized renderer method (RenderXXXBuild) rather than generic RenderBuild. Order methodology checks carefully to handle precedence when multiple methodologies are active.

### 2026-02-07 | ralph-runner-tsf4 | patterns
CLI functions with user prompts or subprocess calls should accept these as injected function parameters, not call them directly—this makes the core logic testable with simple mocks rather than requiring stdin/stdout management or actual subprocess execution

### 2026-02-07 | ralph-runner-uq8m | patterns
Extract small, focused helper functions from larger functions to improve testability and reusability - this codebase follows single-responsibility principle even for utility functions like argument building

### 2026-02-07 | ralph-runner-zgri | conventions
Test new fields by adding assertions to the existing main test first, then create a dedicated test if the field requires special or isolated validation

### 2026-02-07 | ralph-runner-en3f | gotchas
Unix timestamps from git format %at are strings that need int64 parsing before numeric comparison; string comparison fails for timestamps with different digit lengths

### 2026-02-07 | ralph-runner-d9xd | gotchas
When passing large strings to Claude CLI, write to temp files instead of using CLI arguments to avoid exceeding OS ARG_MAX limits. The review.go file demonstrates this pattern correctly.

### 2026-02-07 | ralph-runner-8ayf | conventions
When helpers are consolidated into a shared package, verify that the refactored code doesn't change behavior of callers—the tests should catch regressions, but pre-existing test failures in a codebase can hide whether consolidation was successful.

### 2026-02-07 | ralph-runner-nekg | conventions
When adding fields to structs that are serialized/deserialized (especially JSON), verify that existing tests that depend on that struct's behavior still pass—including contract tests that exercise external tool integration. Use json:"field,omitempty" tags to maintain backward compatibility when adding optional fields.

### 2026-02-07 | ralph-runner-628c | patterns
Context structs in the prompt package (ScopeContext, DecomposeContext, PrecheckContext, etc.) follow a minimal pattern with Bead and ParentBead fields. Each has a corresponding RenderXxx() method on the Renderer. New context types should follow this same structure.

### 2026-02-07 | ralph-runner-nxdm | patterns
Renderer methods that render templates follow a consistent pattern: accept a context struct as parameter, call r.tmpl.ExecuteTemplate with the context, and return (string, error). Use this pattern for all new template rendering methods.

### 2026-02-07 | ralph-runner-5lk0 | conventions
Template constants in cmd/gromit/init.go follow naming convention defaultXxxTemplate and contain complete prompt content with clear instructions for Claude

### 2026-02-07 | ralph-runner-vabo | conventions
Runner methods that call claude.Run with timeout should use context.WithTimeout(ctx, duration) to enforce the timeout, and log warnings on error rather than returning early

### 2026-02-07 | ralph-runner-2m53 | conventions
When a reusable utility function exists for a common pattern (like confirmPrompt for yes/no prompts), other functions should inject and call it rather than reimplementing similar logic, to ensure consistency across the codebase

### 2026-02-07 | ralph-runner-543u | gotchas
Test fixtures in fake scripts should enforce required environment variables (e.g., TEST_DIR) rather than falling back to /tmp, and cleanup code should be idempotent and run even if tests fail (consider using trap handlers or defer statements)

### 2026-02-07 | ralph-runner-qcoc | patterns
Wrapper types around io.Writer should use sync.Mutex for thread safety and track state (like lastWasOverwrite) to manage transitions between different write modes. Test files use table-driven tests with t.Run for each case. For concurrent safety, use t.Parallel() with goroutines and sync.WaitGroup for coordination.

### 2026-02-07 | ralph-runner-t13e | patterns
When wrapping interfaces with new types, store the concrete wrapper type as a struct field (not just the interface) if you need to access wrapper-specific methods like WriteOverwrite

### 2026-02-07 | ralph-runner-z6li | gotchas
syncWriter.WriteOverwrite() handles newline transitions automatically when called - don't manually write newlines before or after, as this can create duplicate blank lines. The writer manages the state machine for overwriting vs. appending output.

### 2026-02-07 | ralph-runner-9u1c | conventions
Phase boundaries in output streams require explicit blank line calls using r.log("") to separate logical sections - especially important before result summaries that conclude streaming output

### 2026-02-07 | ralph-runner-3kow | conventions
PROMPT_*.md files should use pragmatic criteria for learning extraction — task-specific patterns (package conventions, test setup requirements, common gotchas in a subsystem) are valuable as provisional learnings even if not universally applicable; only set learning to null for truly one-off issues (typos, accidental mistakes)

### 2026-02-07 | gromit-yew | conventions
LEARNINGS.md tracks bead IDs in both entry headers and consolidated-from references; when ID schemes change, update both locations consistently to maintain referential integrity across the learning history

### 2026-02-07 | gromit-4zg | gotchas
bd rename-prefix creates ID mappings that must be manually reflected in runtime state files like status.json to keep bead tracking consistent across the system

### 2026-02-07 | gromit-f4t | patterns
Test fixtures use inline struct instantiation with expected output verified via string matching; updating fixture IDs requires changes in both the struct field and corresponding expected output strings

### 2026-02-07 | gromit-m0k | conventions
Superseded docs should add a clear notice at the top referencing the replacement spec/plan, then annotate specific decisions or sections that are superseded inline with reasoning for why they're no longer valid

### 2026-02-07 | gromit-gh9 | conventions
Use normalizeNilFields() pattern after unmarshaling JSON structs and after file loading to convert nil slices to empty slices—prevents bugs with downstream nil checks, ranges, and JSON serialization (nil → 'null' vs [] → '[]')

### 2026-02-07 | gromit-8a52 | gotchas
Learnings file parser requires strict pipe-delimited header format (### DATE | BeadID | Category) and relies on integration tests to catch format corruption rather than lenient parsing—malformed headers silently produce zero-value dates that are hard to debug

### 2026-02-07 | gromit-2v5n | conventions
*Related to: gromit-2v5n, gromit-i0wm, gromit-wgtu*

When modifying config structures or core features that interact with the learnings system, verify that all integration tests pass and that LEARNINGS.md entries maintain structural consistency. The learnings package enforces strict format validation: confirmed learnings must have real dates (not 0001-01-01), BeadID fields containing descriptive learning titles (not category names), and RelatedTo metadata formatted as '*Related to: <ids>*' lines. The integration test TestLoadActualLearningsFile validates this format strictly and catches file corruption. LEARNINGS.md entries require strict pipe-delimited format with position-dependent fields (### DATE | BeadID | Category). When consolidating learnings, ensure dates are current, BeadID contains the full learning title (not categories), and RelatedTo is populated in a separate line, not in the header.

### 2026-02-07 | gromit-49dg | gotchas
When migrating naming conventions across a codebase (like prefix changes), check all documentation and metadata files (LEARNINGS.md, comments, etc.) in addition to code—they can lag behind the main code migration and need explicit updates.

### 2026-02-07 | gromit-3gb | patterns
Learnings consolidation requires careful attention to field mapping: Date must be current date, BeadID must contain the learning title (not category), and RelatedTo must list the consolidated source bead IDs. Integration tests validate this format strictly and will catch format errors before they reach downstream code.

### 2026-02-07 | gromit-kim | conventions
When refactoring test fixtures or mocks, verify that dependent data files (like LEARNINGS.md) are updated to match new bead ID naming conventions and required fields. Learnings data format has strict requirements for dates, beadID format, and RelatedTo fields.

### 2026-02-07 | gromit-due | conventions
When working with LEARNINGS.md, understand its structure: each entry needs a title, date, BeadID, and RelatedTo field. The file is validated by tests that check for specific learning entries by title and structure. Always review the actual LEARNINGS.md content before writing tests that validate it, and ensure new learnings follow the complete schema.

### 2026-02-07 | gromit-due | gotchas
In Go, use cmd.Stdin = strings.NewReader(data) with cmd.Run() instead of StdinPipe+Start+Write+Close+Wait for simple stdin passing - it's simpler, less error-prone, and achieves identical behavior

### 2026-02-07 | gromit-evq | conventions
Data files that are tested by integration tests must conform to the validated schema - format corruption in LEARNINGS.md is caught by TestLoadActualLearningsFile but silently produces zero-value dates and incorrect field mappings that break downstream parsing.

### 2026-02-07 | gromit-w3x | conventions
When consolidating learnings into LEARNINGS.md, verify field positions and formats match the schema (### YYYY-MM-DD | DESCRIPTIVE_TITLE | CATEGORY_NAME with RelatedTo as a separate line). Integration tests catch format corruption but produce confusing zero-value dates, so add explicit validation to the consolidation process to reject invalid dates. When validating skill extraction, verify that downstream consumers (like learnings loader) can properly parse the injected content - test failures indicate the validation point may be in the wrong layer.

### 2026-02-07 | gromit-2na | gotchas
When comparing counter thresholds in retry logic, use consistent comparison operators (either > after increment or >= before increment) across all code paths to avoid subtle off-by-one differences in retry behavior

### 2026-02-07 | gromit-jva | conventions
When a task fails validation, the error output may point to a different test than the one being fixed. Check if the broken test is related to the current task or is a pre-existing issue in the codebase that needs separate fixing.

### 2026-02-07 | gromit-knc | conventions
When reviewing test failures during validation, check if the broken test is related to the current task or is a pre-existing data file validation issue. Integration tests that validate real data files (like LEARNINGS.md) will catch format corruption independently of the task being worked on. The test failure may not be caused by code changes but by non-conforming data files that were not cleaned up by previous iterations.

### 2026-02-07 | gromit-knc | conventions
LEARNINGS.md entries must have: (1) valid date timestamps (not zero/empty), (2) BeadID matching the full bead identifier (not shortened names like 'gotchas'), and (3) non-empty RelatedTo fields. The TestLoadActualLearningsFile test validates this structure and will fail if any learning entry has malformed metadata.

### 2026-02-07 | gromit-3ibd | conventions
When implementing features that write to LEARNINGS.md (or similar structured data files), ensure all required fields are populated with valid values before tests run. Zero dates and empty BeadID fields indicate incomplete writes that bypass normal validation.

### 2026-02-07 | gromit-3ibd | patterns
When functions perform operations with measurable side effects (API calls, I/O), return the measurement from the function itself rather than measuring from the caller. Use multiple return values (value, duration) pattern to pass both the result and its associated metadata.

### 2026-02-07 | gromit-hvbu | conventions
Data consolidation processes must validate output format before persisting - when consolidating LEARNINGS.md entries, verify that dates are current (not zero), BeadID contains the full learning title (not categories), and RelatedTo is populated in a separate line. The integration test is strict about format requirements and will catch corrupted data. When fixes to core features result in test failures in data validation tests, check if the data file format itself is corrupted or if the task introduced new corruption.

### 2026-02-07 | gromit-fntb | conventions
When test failures occur in integration tests that validate real data files (like LEARNINGS.md), distinguish between: (1) code bugs causing the failure, vs (2) pre-existing data file format corruption from previous operations. Data validation tests catch file corruption independent of the current task. Fix the root cause (data format) not just the symptom (test error). LEARNINGS.md requires strict pipe-delimited format with position-specific fields: ### YYYY-MM-DD | DESCRIPTIVE_TITLE | CATEGORY. Consolidated entries must have RelatedTo in a separate '*Related to: id1, id2*' line in the content, not in the header.

### 2026-02-07 | gromit-7e5o | patterns
When modifying code that affects test data or LEARNINGS.md entries, verify that generated/synthetic learnings match the expected format with proper dates (YYYY-MM-DD) and valid beadID/RelatedTo references. Tests validate LEARNINGS.md structure strictly.

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

### 2026-02-06 | gotchas | consolidated from r3h, ehn, 0o2, rz1
Archived: already promoted to rule in RULES.md Process section. Rule is the source of truth.

### 2026-02-07 | ralph-runner-utv8 | conventions
Archived: consolidated into golden file maintenance rule. When adding new prompt templates to the init command, update both the template implementation AND the corresponding CLI contract test golden file to match the new help text output.

### 2026-02-07 | ralph-runner-utv8 | conventions
Archived: consolidated into golden file maintenance rule. When modifying CLI commands that generate files or output, check and update any golden file tests (contract tests) that capture the expected output.

### 2026-02-07 | ralph-runner-utv8 | conventions
Archived: consolidated into golden file maintenance rule. When adding new templates to the gromit init process, the golden file test that validates CLI help text output must be updated in the same commit.

### 2026-02-07 | ralph-runner-utv8 | patterns
Archived: consolidated into template architecture learning. Template variants reuse common context sections and only customize the Instructions and Completion sections.

### 2026-02-07 | ralph-runner-uu07 | patterns
Archived: consolidated into template architecture learning. Renderer methods follow r.render("PROMPT_<name>.md", ctx) naming convention.

### 2026-02-07 | ralph-runner-j3ln | patterns
Archived: consolidated into test mock patterns learning. Mock interfaces use FnField pattern for behavior injection.

### 2026-02-07 | ralph-runner-ge6j | patterns
Archived: consolidated into test mock patterns learning. Use NewRunnerWithDeps with tracking flags to test feature selection.

### 2026-02-07 | ralph-runner-rax7 | conventions
Archived: consolidated into E2E testutil learning. testutil.PickerStdin is the preferred helper for picker selections.

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

### 2026-02-07 | ralph-runner-yx7b | conventions
Archived: template registration approach restated in more recent bead. Consolidated into ralph-runner-kjix approach.

### 2026-02-07 | ralph-runner-kjix | conventions
Archived: consolidated with yx7b into template architecture learning via ralph-runner-utv8.

### 2026-02-07 | ralph-runner-yysu | patterns
Archived: table-driven test pattern consolidated into ralph-runner-qcoc wrapper types learning (concurrent safety section).

### 2026-02-07 | ralph-runner-tizz | conventions
Archived: mock interface pattern consolidated into ge6j mock implementations learning.

### 2026-02-07 | gromit-8et | conventions
Archived: duplicate of gromit-2v5n LEARNINGS.md format validation learning.

### 2026-02-07 | gromit-1u7 | conventions
Archived: duplicate of gromit-w3x consolidation format guidance.

### 2026-02-07 | gromit-3fqu | conventions
Archived: duplicate of gromit-w3x consolidation format guidance.

### 2026-02-07 | gromit-ltg | conventions
Archived: duplicate of gromit-w3x and gromit-3fqu consolidation format guidance.

### 2026-02-07 | gromit-i0wm | conventions
Archived: duplicate of gromit-2v5n LEARNINGS.md format validation learning.

### 2026-02-07 | gromit-wgtu | conventions
Archived: duplicate of gromit-2v5n LEARNINGS.md format validation learning.

