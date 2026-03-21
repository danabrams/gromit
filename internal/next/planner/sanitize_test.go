package planner

import "testing"

func TestSanitizeWorktreePaths_StripsWorktreePrefix(t *testing.T) {
	plan := Plan{
		Tasks: []TaskDef{
			{
				ProofChecks: []string{
					"go build ./...",
					"grep -q 'func Foo' .gromit-next/worktrees/wt-2474025559/internal/next/reviewdistiller/types.go",
					"! grep -q 'import.*\"os\"' .gromit-next/worktrees/wt-2474025559/internal/next/reviewdistiller/validate.go || echo 'FAIL' && exit 1",
				},
				ExpectedTouchedArea: []string{
					".gromit-next/worktrees/wt-2474025559/internal/next/reviewdistiller/types.go",
					"cmd/gromit-next/review_distill.go",
				},
			},
		},
	}

	SanitizeWorktreePaths(&plan)

	wantChecks := []string{
		"go build ./...",
		"grep -q 'func Foo' internal/next/reviewdistiller/types.go",
		"! grep -q 'import.*\"os\"' internal/next/reviewdistiller/validate.go || echo 'FAIL' && exit 1",
	}
	for i, got := range plan.Tasks[0].ProofChecks {
		if got != wantChecks[i] {
			t.Errorf("ProofChecks[%d] = %q, want %q", i, got, wantChecks[i])
		}
	}

	wantArea := []string{
		"internal/next/reviewdistiller/types.go",
		"cmd/gromit-next/review_distill.go",
	}
	for i, got := range plan.Tasks[0].ExpectedTouchedArea {
		if got != wantArea[i] {
			t.Errorf("ExpectedTouchedArea[%d] = %q, want %q", i, got, wantArea[i])
		}
	}
}

func TestSanitizeWorktreePaths_NoOpWithoutPrefix(t *testing.T) {
	plan := Plan{
		Tasks: []TaskDef{
			{
				ProofChecks:         []string{"go build ./...", "grep -q 'func Foo' internal/pkg/foo.go"},
				ExpectedTouchedArea: []string{"internal/pkg/foo.go"},
			},
		},
	}

	SanitizeWorktreePaths(&plan)

	if plan.Tasks[0].ProofChecks[1] != "grep -q 'func Foo' internal/pkg/foo.go" {
		t.Errorf("unexpected modification: %q", plan.Tasks[0].ProofChecks[1])
	}
}

func TestSanitizeWorktreePaths_MultipleWorktrees(t *testing.T) {
	plan := Plan{
		Tasks: []TaskDef{
			{
				ProofChecks: []string{
					"grep -q 'X' .gromit-next/worktrees/wt-1111111111/a.go",
					"grep -q 'Y' .gromit-next/worktrees/wt-9999999999/b.go",
				},
			},
		},
	}

	SanitizeWorktreePaths(&plan)

	if plan.Tasks[0].ProofChecks[0] != "grep -q 'X' a.go" {
		t.Errorf("check[0] = %q", plan.Tasks[0].ProofChecks[0])
	}
	if plan.Tasks[0].ProofChecks[1] != "grep -q 'Y' b.go" {
		t.Errorf("check[1] = %q", plan.Tasks[0].ProofChecks[1])
	}
}
