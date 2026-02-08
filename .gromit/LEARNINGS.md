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

### 2026-02-07 | Test Helper Delegation | conventions
*Related to: rkub, ewqu, uucn, y0v3*

Test helpers delegate to shared testutil packages rather than duplicating logic. Use t.Fatalf for error handling when adapting function signatures with different return counts. Handle optional parameters using zero-value checks (empty string for dir, nil for environ) rather than pointers. E2E test files delegate to testutil package helpers (e.g., testutil.PickerStdin) rather than hardcoding raw strings.

### 2026-02-07 | Documentary Test Replacement | patterns
*Related to: dpk5, prk6, q2id, l0ka*

Replace documentary tests that manually simulate behavior with real tests using dependency injection of callbacks. Include structural property tests (e.g., unmarshal round-trips) to verify extraction fidelity. Integration tests that cover end-to-end flows make narrow documentary tests redundant.

### 2026-02-07 | Mock Implementation Patterns | patterns
*Related to: ge6j, atmb*

Mock implementations use optional function pointer fields (FnField pattern) with nil-safe defaults. Tests set up only the behavior needed via tracking flags or injected callbacks, enabling focused verification of specific code paths without requiring full mock setup.

### 2026-02-07 | Status File Management | patterns
*Related to: nalr, k8c2, kydj, ead1, xpfn, lm34, 2y2d, yj2h, vpyl, kim2*

Status struct fields require backward-compatible changes (omitempty for new optional fields). Use ReadStatus()/IsProcessAlive() for state + liveness checks. Return nil,nil for missing optional files (not an error). StatusWriter handles both lifecycle states and preserves completed iteration count on shutdown. Round-trip tests verify serialization fidelity. Stale resource cleanup integrates into status reporting via process liveness checks. Process utilities (IsProcessAlive) are co-located with Status in status.go. Test file I/O uses t.TempDir() for isolation.

### 2026-02-07 | Output Formatting | patterns
*Related to: 5xbi, 88fk, 9ntm*

Format functions build output via []string slice joined with newlines. Duration formatting composes by delegating to formatDuration() then appending context like " ago". Use strings.Contains() for multi-line output tests; exact equality for single-value formatters.

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

