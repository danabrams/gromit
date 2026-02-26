---
id: normalize-nil-fields-convention
source_ideas: []
created: 2026-02-26
epic: codebase-health
---

# Normalize Nil Fields Naming Convention Policy

## Problem

`GateResult` uses unexported `normalizeNilFields()` while `SubTask` uses exported `NormalizeNilFields()`. The inconsistency makes it unclear which visibility to use when adding the method to new types, leading to ad-hoc decisions that accumulate over time.

## Approach

- Document the convention as policy in `CLAUDE.md` under a "Conventions" or "Code Patterns" section: use **unexported** `normalizeNilFields()` when the method is only called within the same package; use **exported** `NormalizeNilFields()` when called from other packages
- Add a brief inline comment on one example of each visibility in the codebase pointing to the policy (e.g., a comment on `GateResult.normalizeNilFields()` and `SubTask.NormalizeNilFields()`)
- No code changes needed — the current split is already correct and follows the principle of minimal exported surface
- Verify the existing usages match the documented policy before writing it down; if any mismatch is found, fix it as part of this item

## Files to Change

- `CLAUDE.md` — add convention note under a "Code Patterns" section
- `internal/` (whichever file defines `GateResult.normalizeNilFields`) — add brief comment referencing the policy
- `internal/` (whichever file defines `SubTask.NormalizeNilFields`) — add brief comment referencing the policy

## Acceptance Criteria

- `CLAUDE.md` documents the exported vs. unexported `normalizeNilFields` convention
- All existing `normalizeNilFields`/`NormalizeNilFields` usages are consistent with the documented policy
- Any mismatches found during audit are corrected
- No new exported methods are introduced where unexported suffices
