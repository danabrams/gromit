# Learnings

Accumulated operational knowledge from Gromit iterations.
This file is automatically updated. Review periodically with `gromit retro`.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

*No confirmed learnings yet.*

---

## Provisional

*Seen once - may be specific to one task.*

### 2026-02-07 | ralph-runner-4a3f | patterns
Label injection for sub-beads should check parent labels first before adding globally-active methodology labels, to avoid duplicates and respect parent specifications

### 2026-02-07 | ralph-runner-utv8 | conventions
When adding new prompt templates to the init command, update both the template implementation AND the corresponding CLI contract test golden file to match the new help text output.

### 2026-02-07 | ralph-runner-utv8 | conventions
When modifying CLI commands that generate files or output, check and update any golden file tests (contract tests) that capture the expected output. CLI contract tests validate exact output format and must be explicitly updated when behavior changes.

### 2026-02-07 | ralph-runner-utv8 | conventions
When adding new templates to the gromit init process, the golden file test that validates CLI help text output must be updated in the same commit. Check for golden/snapshot files in test suites before declaring tasks complete.

### 2026-02-07 | ralph-runner-utv8 | patterns
Template variants reuse common context sections (Rules, Learnings, Task, Spec, Parent) and only customize the Instructions and Completion sections for methodology-specific workflows (TDD, ATDD, standard build)

### 2026-02-07 | ralph-runner-uu07 | patterns
Renderer methods follow a pattern of calling r.render("PROMPT_<name>.md", ctx) with a consistent template naming convention PROMPT_<method_suffix>.md

### 2026-02-07 | ralph-runner-j3ln | patterns
Mock interfaces in tests follow a pattern: define FnField for each method to allow test-specific behavior injection, then call that field in the mock method implementation. This enables flexible test configuration without modifying the mock struct.

### 2026-02-07 | ralph-runner-nzue | patterns
Methodologies in this codebase use label-based activation ("methodology:true"/"false") with global config fallback via bead.IsMethodologyActive(). When active, replace the build prompt with a specialized renderer method (RenderXXXBuild) rather than generic RenderBuild. Order methodology checks carefully to handle precedence when multiple methodologies are active.

### 2026-02-07 | ralph-runner-ge6j | patterns
Use NewRunnerWithDeps with tracking flags in mock renderers (e.g., tddBuildCalled := false) to test feature selection logic that conditionally calls different render methods based on config + labels

### 2026-02-07 | ralph-runner-yx7b | conventions
Template registration in gromit init follows a consistent pattern: define a constant with the template content, then call writeFileIfNotExists in runInit with the .gromit/templates/ path, and add the filename to the command's Long help text

### 2026-02-07 | ralph-runner-yysu | patterns
Table-driven tests in this codebase use t.Run with subtests for each case, and panic recovery should use defer/recover() pattern with checking for nil in defer block

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

