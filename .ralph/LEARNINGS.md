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

*No provisional learnings.*

---

## Archived

*Learnings that were too generic or no longer relevant.*

### 2026-02-05 | ralph-runner-btj.1 | patterns
When implementing command handlers, ensure all referenced functions are fully implemented before testing. Breaking up command logic across multiple files requires explicit implementation in each file, not just in main.go.
*Archived: Generic programming advice, not project-specific.*

### 2026-02-05 | ralph-runner-zib | patterns
When testing CLI wrappers, mock the external commands or use test fixtures rather than relying on real utilities with different argument parsing. Tests that try to use unrelated commands as standin CLIs lead to brittle, environment-dependent failures.

### 2026-02-05 | ralph-runner-btj.1, btj.5 | conventions
Before implementing, diagnosing, or recovering from task issues, always verify actual file and code state. Explore existing commands to understand patterns before building new ones, and check whether work was actually completed before assuming failure — status reports may be inaccurate due to missing execution context.
*Archived: Promoted to Process rule — "Each iteration starts with fresh context..."*

### 2026-02-05 | ralph-runner-btj.1 | gotchas
Validation failures due to missing tools (Go, Node, Python, etc.) are environment issues, not code issues. Check tool availability before assuming code is broken. Ralph's validation step runs in the same environment as the task, so missing tools block validation even if code is correct.
*Archived: Consolidated into Process rule about distinguishing environment failures from code failures.*

