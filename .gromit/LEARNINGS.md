# Learnings

Accumulated operational knowledge from Gromit iterations.
This file is automatically updated. Review periodically with `gromit retro`.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

### 2026-02-07 | Shell Safety | gotchas
Shell scripts handling user content must use quoted <<'EOF' heredocs to prevent variable/command expansion, and pass dynamic values via arguments rather than string interpolation. Both heredoc quoting and argument-passing are required for injection safety.
**Promoted to RULES.md Safety section.**

*Related to: m7td, 6rao*

### 2026-02-07 | Documentary Test Replacement | conventions
Replace documentary tests that manually simulate behavior with real tests using dependency injection of callbacks. Include structural property tests (e.g., unmarshal round-trips) to verify extraction fidelity. Integration tests that cover end-to-end flows make narrow documentary tests redundant. Test helpers delegate to shared testutil packages rather than duplicating logic. Use t.Fatalf for error handling when adapting function signatures with different return counts. Handle optional parameters using zero-value checks (empty string for dir, nil for environ) rather than pointers. E2E test files delegate to testutil package helpers (e.g., testutil.PickerStdin) rather than hardcoding raw strings.

*Related to: dpk5, prk6, q2id, l0ka, rkub, ewqu, uucn, y0v3*

### 2026-02-07 | Mock Implementation Patterns | patterns
Mock implementations use optional function pointer fields (FnField pattern) with nil-safe defaults. Tests set up only the behavior needed via tracking flags or injected callbacks, enabling focused verification of specific code paths without requiring full mock setup.

*Related to: ge6j, atmb*

### 2026-02-07 | Status File Management | patterns
Status struct fields require backward-compatible changes (omitempty for new optional fields). Use ReadStatus()/IsProcessAlive() for state + liveness checks. Return nil,nil for missing optional files (not an error). StatusWriter handles both lifecycle states and preserves completed iteration count on shutdown. Round-trip tests verify serialization fidelity. Stale resource cleanup integrates into status reporting via process liveness checks. Process utilities (IsProcessAlive) are co-located with Status in status.go. Test file I/O uses t.TempDir() for isolation.

*Related to: nalr, k8c2, kydj, ead1, xpfn, lm34, 2y2d, yj2h, vpyl, kim2*

### 2026-02-07 | Output Formatting | patterns
Format functions build output via []string slice joined with newlines. Duration formatting composes by delegating to formatDuration() then appending context like " ago". Use strings.Contains() for multi-line output tests; exact equality for single-value formatters.

*Related to: 5xbi, 88fk, 9ntm*

### 2026-02-07 | LEARNINGS.md Format Validation | conventions
When working with LEARNINGS.md, understand its structure and format requirements. Each confirmed learning entry must have: (1) valid date timestamps (not zero values), (2) a BeadID field with a descriptive learning title (not category names), (3) a Category field, and (4) a RelatedTo field listing consolidated source bead IDs in a separate '*Related to: id1, id2*' line. The learnings file is validated by integration tests that strictly enforce this format, so malformed entries will cause test failures. Always verify that file format modifications maintain expected field positions and required metadata before committing.

*Related to: gromit-ie2*

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

### 2026-02-07 | ralph-runner-yx7b | conventions
Template registration in gromit init follows a consistent pattern: define a constant with the template content, then call writeFileIfNotExists in runInit with the .gromit/templates/ path, and add the filename to the command's Long help text

### 2026-02-07 | ralph-runner-yysu | patterns
Table-driven tests in this codebase use t.Run with subtests for each case, and panic recovery should use defer/recover() pattern with checking for nil in defer block

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
When adding fields to structs that are serialized/deserialized (especially JSON), verify that existing tests that depend on that struct's behavior still pass—including contract tests that exercise external tool integration.

### 2026-02-07 | ralph-runner-nekg | conventions
Use json:"field,omitempty" tags on struct fields to maintain backward compatibility when adding optional fields to types that are serialized to JSON

### 2026-02-07 | ralph-runner-628c | patterns
Context structs in the prompt package (ScopeContext, DecomposeContext, PrecheckContext, etc.) follow a minimal pattern with Bead and ParentBead fields. Each has a corresponding RenderXxx() method on the Renderer. New context types should follow this same structure.

### 2026-02-07 | ralph-runner-nxdm | patterns
Renderer methods that render templates follow a consistent pattern: accept a context struct as parameter, call r.tmpl.ExecuteTemplate with the context, and return (string, error). Use this pattern for all new template rendering methods.

### 2026-02-07 | ralph-runner-5lk0 | conventions
Template constants in cmd/gromit/init.go follow naming convention defaultXxxTemplate and contain complete prompt content with clear instructions for Claude

### 2026-02-07 | ralph-runner-kjix | conventions
Template registration in gromit init follows a consistent pattern: define a constant with the template content, then add it to the templates map in runInit() with the filename as key. New templates should follow this same approach without needing to update multiple locations.

### 2026-02-07 | ralph-runner-tizz | conventions
Interface mock implementations in *_test.go files require adding corresponding Fn fields and calling them in the mock method, mirroring the pattern of existing Render* methods

### 2026-02-07 | ralph-runner-vabo | conventions
Runner methods that call claude.Run with timeout should use context.WithTimeout(ctx, duration) to enforce the timeout, and log warnings on error rather than returning early

### 2026-02-07 | ralph-runner-2m53 | conventions
When a reusable utility function exists for a common pattern (like confirmPrompt for yes/no prompts), other functions should inject and call it rather than reimplementing similar logic, to ensure consistency across the codebase

### 2026-02-07 | ralph-runner-543u | gotchas
Test fixtures in fake scripts should enforce required environment variables (e.g., TEST_DIR) rather than falling back to /tmp, and cleanup code should be idempotent and run even if tests fail (consider using trap handlers or defer statements)

### 2026-02-07 | ralph-runner-qcoc | patterns
Wrapper types around io.Writer should use sync.Mutex for thread safety and track state (like lastWasOverwrite) to manage transitions between different write modes

### 2026-02-07 | ralph-runner-2dsc | patterns
Test files use table-driven tests with t.Run for each case, and concurrent safety tests should use t.Parallel() with goroutines and sync.WaitGroup for coordination

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

### 2026-02-07 | gromit-i0wm | conventions
When modifying LEARNINGS.md format or structure, verify that all learning entries have valid dates, descriptive BeadID titles (not category names), and non-empty RelatedTo fields. The learnings file has strict format requirements enforced by TestLoadActualLearningsFile that must be satisfied.

### 2026-02-07 | gromit-i0wm | conventions
LEARNINGS.md has a specific structured format with required fields: BeadID should contain full descriptive titles (not category labels), dates should be valid timestamps (not 0001-01-01), and RelatedTo should not be empty. When modifying shared files like LEARNINGS.md, validate the format against test expectations in internal/learnings before committing.

### 2026-02-07 | gromit-wgtu | conventions
LEARNINGS.md has a strict pipe-delimited header format (### DATE | BeadID | Category). When consolidating learnings, ensure BeadID contains the descriptive title (not the category), use current or past dates (not 0001-01-01), and record consolidated IDs in the *Related to:* metadata line, not in the category field. Integration tests validate this format strictly and will catch malformed entries.

### 2026-02-07 | gromit-2v5n | conventions
When modifying config.go or configuration structures in gromit, verify that test fixtures (especially LEARNINGS.md parsing tests) are compatible with the new schema. The learnings system has strict parsing requirements for dates, BeadID references, and RelatedTo relationships.

### 2026-02-07 | gromit-2v5n | conventions
When modifying core features like precheck that interact with the learnings system, validate that all integration tests pass and that existing learnings maintain structural consistency (proper dates, BeadID mappings, RelatedTo relationships). The LEARNINGS.md file is validated during test runs and changes to how learnings are processed must account for existing entries.

### 2026-02-07 | gromit-2v5n | conventions
When modifying core configuration structs (config.go) and config files (gromit.yaml), verify that integration tests in the learnings package pass—they validate both the structure and content of LEARNINGS.md. These tests are strict about field formats and dates, so any configuration changes that affect how learnings are parsed or generated may require test updates or LEARNINGS.md cleanup.

### 2026-02-07 | gromit-2v5n | conventions
The learnings package has strict format validation (dates must be non-zero, BeadID must be descriptive titles not categories, RelatedTo must be populated for consolidated entries). Integration tests enforce these constraints and catch any malformed LEARNINGS.md entries. When modifying core config structures or running comprehensive tests, verify that LEARNINGS.md entries conform to the expected format before committing.

### 2026-02-07 | gromit-2v5n | conventions
LEARNINGS.md entries require strict pipe-delimited format with position-dependent fields (### DATE | BeadID | Category). Consolidated learnings must include RelatedTo metadata in the content. The integration test TestLoadActualLearningsFile validates this format strictly and catches structural corruption. Always verify that file format modifications maintain expected field positions and required metadata.

### 2026-02-07 | gromit-2v5n | conventions
The learnings package expects confirmed learnings to have real dates, BeadID fields containing learning titles, and RelatedTo fields containing consolidated bead IDs. When consolidating learnings, ensure the LEARNINGS.md markdown format maps correctly to these fields during parsing.

### 2026-02-07 | gromit-2v5n | conventions
The learnings package enforces strict format validation on LEARNINGS.md: confirmed learnings must have real dates (not 0001-01-01), BeadID fields containing descriptive learning titles (not category names), and RelatedTo metadata formatted as '*Related to: <ids>*' lines in the content section. Integration tests validate this format strictly to catch file corruption. When modifying core config structures or running comprehensive test suites, verify that all LEARNINGS.md entries conform to the expected format before committing.

### 2026-02-07 | gromit-49dg | gotchas
When migrating naming conventions across a codebase (like prefix changes), check all documentation and metadata files (LEARNINGS.md, comments, etc.) in addition to code—they can lag behind the main code migration and need explicit updates.

### 2026-02-07 | gromit-8et | conventions
LEARNINGS.md requires strict pipe-delimited header format (### DATE | BeadID | Category). Consolidated learnings must use current dates, have BeadID as a descriptive title (not a category), and include RelatedTo metadata in a separate *Related to: <ids>* line. The integration test TestLoadActualLearningsFile validates this format strictly. When editing LEARNINGS.md manually or via consolidation, verify the header structure matches the parser expectations.

### 2026-02-07 | gromit-8et | conventions
LEARNINGS.md requires strict pipe-delimited header format (### DATE | BeadID | Category). Consolidated learnings must have descriptive BeadID titles (not category names), valid dates (not 0001-01-01), and explicit RelatedTo metadata in a separate '*Related to: <ids>*' line. The integration test TestLoadActualLearningsFile validates this format strictly and will catch malformed entries.

### 2026-02-07 | gromit-3gb | patterns
Learnings consolidation requires careful attention to field mapping: Date must be current date, BeadID must contain the learning title (not category), and RelatedTo must list the consolidated source bead IDs. Integration tests validate this format strictly and will catch format errors before they reach downstream code.

### 2026-02-07 | gromit-kim | conventions
When refactoring test fixtures or mocks, verify that dependent data files (like LEARNINGS.md) are updated to match new bead ID naming conventions and required fields. Learnings data format has strict requirements for dates, beadID format, and RelatedTo fields.

### 2026-02-07 | gromit-due | conventions
When working with LEARNINGS.md, understand its structure: each entry needs a title, date, BeadID, and RelatedTo field. The file is validated by tests that check for specific learning entries by title and structure. Always review the actual LEARNINGS.md content before writing tests that validate it, and ensure new learnings follow the complete schema.

### 2026-02-07 | gromit-due | gotchas
In Go, use cmd.Stdin = strings.NewReader(data) with cmd.Run() instead of StdinPipe+Start+Write+Close+Wait for simple stdin passing - it's simpler, less error-prone, and achieves identical behavior

### 2026-02-07 | gromit-1u7 | conventions
When consolidating learnings into LEARNINGS.md, maintain strict adherence to the pipe-delimited header format: ### CURRENT_DATE | DESCRIPTIVE_LEARNING_TITLE | CATEGORY_NAME. Consolidated source bead IDs belong in a separate line in the content (*Related to: bead1, bead2*), not in header fields. Integration tests validate this format and catch file corruption from consolidation errors.

### 2026-02-07 | gromit-evq | conventions
Data files that are tested by integration tests must conform to the validated schema - format corruption in LEARNINGS.md is caught by TestLoadActualLearningsFile but silently produces zero-value dates and incorrect field mappings that break downstream parsing.

### 2026-02-07 | gromit-w3x | conventions
LEARNINGS.md consolidation must follow strict pipe-delimited format (### YYYY-MM-DD | DESCRIPTIVE_TITLE | CATEGORY_NAME) with RelatedTo as a separate line. When consolidating learnings programmatically or manually, verify field positions and formats match the schema. Integration tests catch format corruption but produce confusing zero-value dates - add explicit validation to the consolidation process to reject invalid dates.

### 2026-02-07 | gromit-w3x | conventions
When validating skill extraction, verify that downstream consumers (like learnings loader) can properly parse the injected content. Test failures in dependent systems indicate the validation point may be in the wrong layer or the format contract needs to be checked.

### 2026-02-07 | gromit-2na | gotchas
When comparing counter thresholds in retry logic, use consistent comparison operators (either > after increment or >= before increment) across all code paths to avoid subtle off-by-one differences in retry behavior

### 2026-02-07 | gromit-ltg | conventions
When consolidating learnings into LEARNINGS.md, maintain strict adherence to the pipe-delimited header format: ### YYYY-MM-DD | DESCRIPTIVE_LEARNING_TITLE | CATEGORY_NAME. The BeadID field must contain the full learning title (not the category), use current or past dates (not 0001-01-01), and record consolidated source IDs in a separate '*Related to: id1, id2*' line in the content section. Integration tests validate this format strictly and catch file corruption during test runs.

### 2026-02-07 | gromit-jva | conventions
When a task fails validation, the error output may point to a different test than the one being fixed. Check if the broken test is related to the current task or is a pre-existing issue in the codebase that needs separate fixing.

### 2026-02-07 | gromit-knc | conventions
When reviewing test failures during validation, check if the broken test is related to the current task or is a pre-existing data file validation issue. Integration tests that validate real data files (like LEARNINGS.md) will catch format corruption independently of the task being worked on. The test failure may not be caused by code changes but by non-conforming data files that were not cleaned up by previous iterations.

### 2026-02-07 | gromit-knc | conventions
LEARNINGS.md entries must have: (1) valid date timestamps (not zero/empty), (2) BeadID matching the full bead identifier (not shortened names like 'gotchas'), and (3) non-empty RelatedTo fields. The TestLoadActualLearningsFile test validates this structure and will fail if any learning entry has malformed metadata.

### 2026-02-07 | gromit-3fqu | conventions
LEARNINGS.md consolidation must follow strict pipe-delimited format (### YYYY-MM-DD | DESCRIPTIVE_TITLE | CATEGORY_NAME) with RelatedTo as a separate line. When consolidated learnings are created, ensure BeadID contains the full learning title, use current dates, and record source IDs in a '*Related to: id1, id2*' line, not in the header. Integration tests validate this format and will catch corruption.

### 2026-02-07 | gromit-3ibd | conventions
When implementing features that write to LEARNINGS.md (or similar structured data files), ensure all required fields are populated with valid values before tests run. Zero dates and empty BeadID fields indicate incomplete writes that bypass normal validation.

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

