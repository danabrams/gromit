# Learnings

Accumulated operational knowledge from Ralph iterations.
This file is automatically updated. Review periodically with `ralph retro`.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

*No confirmed learnings yet.*

---

## Provisional

*Seen once - may be specific to one task.*

### 2026-02-05 | ralph-runner-btj.1 | conventions
Before implementing a new command in a modular system, explore existing similar commands and their structure to understand the established patterns and conventions. Understanding how the system's configuration, templates, and file structures work is critical context.

### 2026-02-05 | ralph-runner-btj.1 | conventions
When a task is reported as failed with no error output, verify that the implementation actually failed before attempting recovery - successful implementations may be incorrectly flagged as failures due to missing execution context or reporting issues.

### 2026-02-05 | ralph-runner-btj.1 | patterns
When implementing command handlers, ensure all referenced functions are fully implemented before testing. Breaking up command logic across multiple files requires explicit implementation in each file, not just in main.go.

