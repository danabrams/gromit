# Learnings

Accumulated operational knowledge from Gromit iterations.
This file is automatically updated. Review periodically with `gromit retro`.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

### Shell Safety | gotchas | consolidated from m7td, 6rao
Shell scripts handling user content must use quoted <<'EOF' heredocs to prevent variable/command expansion, and pass dynamic values via arguments rather than string interpolation. Both heredoc quoting and argument-passing are required for injection safety.
**Promoted to RULES.md Safety section.**

### Test Helper Delegation | conventions | consolidated from rkub, ewqu, uucn, y0v3
Test helpers delegate to shared testutil packages rather than duplicating logic. Use t.Fatalf for error handling when adapting function signatures with different return counts. Handle optional parameters using zero-value checks (empty string for dir, nil for environ) rather than pointers. E2E test files delegate to testutil package helpers (e.g., testutil.PickerStdin) rather than hardcoding raw strings.

### Documentary Test Replacement | patterns | consolidated from dpk5, prk6, q2id, l0ka
Replace documentary tests that manually simulate behavior with real tests using dependency injection of callbacks. Include structural property tests (e.g., unmarshal round-trips) to verify extraction fidelity. Integration tests that cover end-to-end flows make narrow documentary tests redundant.

### Mock Implementation Patterns | patterns | consolidated from ge6j, atmb
Mock implementations use optional function pointer fields (FnField pattern) with nil-safe defaults. Tests set up only the behavior needed via tracking flags or injected callbacks, enabling focused verification of specific code paths without requiring full mock setup.

### Status File Management | patterns | consolidated from nalr, k8c2, kydj, ead1, xpfn, lm34, 2y2d, yj2h, vpyl, kim2
Status struct fields require backward-compatible changes (omitempty for new optional fields). Use ReadStatus()/IsProcessAlive() for state + liveness checks. Return nil,nil for missing optional files (not an error). StatusWriter handles both lifecycle states and preserves completed iteration count on shutdown. Round-trip tests verify serialization fidelity. Stale resource cleanup integrates into status reporting via process liveness checks. Process utilities (IsProcessAlive) are co-located with Status in status.go. Test file I/O uses t.TempDir() for isolation.

### Output Formatting | patterns | consolidated from 5xbi, 88fk, 9ntm
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
