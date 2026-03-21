package planner

import "regexp"

// worktreePathRe matches .gromit-next/worktrees/wt-DIGITS/ prefixes that the
// LLM sometimes embeds in proof checks and expected_touched_area when review
// findings reference worktree paths.
var worktreePathRe = regexp.MustCompile(`\.gromit-next/worktrees/wt-\d+/`)

// SanitizeWorktreePaths strips worktree path prefixes from all proof checks
// and expected_touched_area entries in the plan's tasks. Proof checks and
// touched-area paths must be relative to the project root; worktree prefixes
// cause validation failures when tasks execute in a different worktree.
func SanitizeWorktreePaths(plan *Plan) {
	for i := range plan.Tasks {
		sanitizeTask(&plan.Tasks[i])
	}
}

func sanitizeTask(t *TaskDef) {
	for i, check := range t.ProofChecks {
		t.ProofChecks[i] = worktreePathRe.ReplaceAllString(check, "")
	}
	for i, path := range t.ExpectedTouchedArea {
		t.ExpectedTouchedArea[i] = worktreePathRe.ReplaceAllString(path, "")
	}
}
