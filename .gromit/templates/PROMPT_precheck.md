# Pre-Check: Acceptance Criteria Already Met?

You are performing a lightweight pre-check to determine whether the acceptance criteria for this task are already satisfied by the current codebase.

## Task Details

**ID:** {{.Bead.ID}}
**Title:** {{.Bead.Title}}
**Priority:** P{{.Bead.Priority}}
{{if .Bead.Labels}}**Labels:** {{join .Bead.Labels ", "}}{{end}}

{{if .Bead.Description}}
### Description
{{.Bead.Description}}
{{end}}

{{if .ParentBead}}
## Parent Context

This task is part of: **{{.ParentBead.Title}}**
{{if .ParentBead.Description}}
{{.ParentBead.Description}}
{{end}}
{{end}}

## Your Job

**Read the codebase** and determine whether ALL acceptance criteria for this task are already met.

### Instructions

1. **Examine relevant files** in the codebase to check each acceptance criterion
2. **Do NOT make any changes** — this is a read-only inspection
3. **Be conservative** — when uncertain, err on the side of NOT_MET
4. **Output your verdict clearly:**
   - If ALL criteria are already satisfied: output exactly `PRECHECK_PASSED`
   - If ANY criterion is not yet satisfied: output exactly `PRECHECK_NOT_MET`

### Why Be Conservative?

- A false negative (saying NOT_MET when criteria are met) just means we run the normal iteration — no harm done
- A false positive (saying PASSED when criteria aren't met) would skip needed work — this is bad
- When in doubt, choose `PRECHECK_NOT_MET`

## Important

- This is a quick check, not a full code review
- Focus on whether the acceptance criteria are met, not code quality
- Do NOT write or modify any code
- Do NOT run any commands or tests
- Just read files and report your verdict

## Output Format

After your analysis, output ONE of these exact strings on a line by itself:
- `PRECHECK_PASSED` (if all acceptance criteria are satisfied)
- `PRECHECK_NOT_MET` (if any criterion is not satisfied or you're uncertain)
