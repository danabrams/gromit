package prompt

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
)

func TestApplyPhaseProfile_DecomposePrunesExcludedContext(t *testing.T) {
	ctx := &Context{
		Bead: &bead.Bead{
			ExpectedOutputs: []string{"criterion"},
		},
		Spec:                     "full spec",
		ClaudeMD:                 "scoped claude",
		Rules:                    "rules",
		ConfirmedLearnings:       []learnings.Learning{{Category: "c", Content: "v"}},
		RecentLearnings:          []learnings.Learning{{Category: "c", Content: "v"}},
		RecentValidationFailures: []string{"failure"},
		CoverageState:            "coverage",
		TargetCriterion:          "target",
		PrevFailure:              "previous failure",
		SiblingTouchedPackages:   []string{"internal/prompt"},
	}

	ApplyPhaseProfile(ctx, "decompose")

	if ctx.Spec == "" {
		t.Fatalf("Spec should be preserved for decompose")
	}
	if ctx.ClaudeMD == "" {
		t.Fatalf("ClaudeMD should be preserved for decompose")
	}
	if ctx.Rules != "" {
		t.Fatalf("Rules should be pruned for decompose")
	}
	if len(ctx.ConfirmedLearnings) != 0 {
		t.Fatalf("ConfirmedLearnings should be pruned for decompose")
	}
	if len(ctx.RecentLearnings) != 0 {
		t.Fatalf("RecentLearnings should be pruned for decompose")
	}
	if len(ctx.RecentValidationFailures) != 0 {
		t.Fatalf("RecentValidationFailures should be pruned for decompose")
	}
	if ctx.CoverageState != "" {
		t.Fatalf("CoverageState should be pruned for decompose")
	}
	if ctx.TargetCriterion != "" {
		t.Fatalf("TargetCriterion should be pruned for decompose")
	}
	if ctx.PrevFailure != "" {
		t.Fatalf("PrevFailure should be pruned for decompose")
	}
	if len(ctx.SiblingTouchedPackages) != 0 {
		t.Fatalf("SiblingTouchedPackages should be pruned for decompose")
	}
	if len(ctx.Bead.ExpectedOutputs) != 0 {
		t.Fatalf("ExpectedOutputs should be pruned for decompose")
	}
}

func TestApplyReviewPhaseProfile_ReviewPrunesProjectContext(t *testing.T) {
	ctx := &ReviewContext{
		Spec:               "spec",
		Diff:               "diff",
		ClaudeMD:           "claude",
		Rules:              "rules",
		ValidationCommands: []string{"go test ./..."},
	}

	ApplyReviewPhaseProfile(ctx, "review")

	if ctx.Diff == "" {
		t.Fatalf("Diff should be preserved for review")
	}
	if ctx.Rules == "" {
		t.Fatalf("Rules should be preserved for review")
	}
	if ctx.Spec != "" {
		t.Fatalf("Spec should be pruned for review")
	}
	if ctx.ClaudeMD != "" {
		t.Fatalf("ClaudeMD should be pruned for review")
	}
	if len(ctx.ValidationCommands) != 0 {
		t.Fatalf("ValidationCommands should be pruned for review")
	}
}

func TestApplyPhaseProfile_RedPrunesToCriterionFocusedContext(t *testing.T) {
	ctx := &Context{
		Spec:                     "spec",
		Rules:                    "rules",
		TargetCriterion:          "criterion",
		CoverageState:            "coverage",
		ClaudeMD:                 "claude",
		ConfirmedLearnings:       []learnings.Learning{{Category: "c", Content: "confirmed"}},
		RecentLearnings:          []learnings.Learning{{Category: "c", Content: "recent"}},
		RecentValidationFailures: []string{"failure"},
		PrevFailure:              "prev",
		SiblingTouchedPackages:   []string{"internal/prompt"},
	}

	ApplyPhaseProfile(ctx, "red")

	if ctx.Spec == "" {
		t.Fatalf("Spec should be preserved for red")
	}
	if ctx.Rules == "" {
		t.Fatalf("Rules should be preserved for red")
	}
	if ctx.TargetCriterion == "" {
		t.Fatalf("TargetCriterion should be preserved for red")
	}
	if ctx.CoverageState == "" {
		t.Fatalf("CoverageState should be preserved for red")
	}
	if ctx.ClaudeMD != "" {
		t.Fatalf("ClaudeMD should be pruned for red")
	}
	if len(ctx.ConfirmedLearnings) != 0 {
		t.Fatalf("ConfirmedLearnings should be pruned for red")
	}
	if len(ctx.RecentLearnings) != 0 {
		t.Fatalf("RecentLearnings should be pruned for red")
	}
	if len(ctx.RecentValidationFailures) != 0 {
		t.Fatalf("RecentValidationFailures should be pruned for red")
	}
	if ctx.PrevFailure != "" {
		t.Fatalf("PrevFailure should be pruned for red")
	}
	if len(ctx.SiblingTouchedPackages) != 0 {
		t.Fatalf("SiblingTouchedPackages should be pruned for red")
	}
}

func TestApplyPhaseProfile_BuildPreservesImplementationContext(t *testing.T) {
	ctx := &Context{
		Spec:                     "spec",
		ClaudeMD:                 "claude",
		Rules:                    "rules",
		ConfirmedLearnings:       []learnings.Learning{{Category: "c", Content: "confirmed"}},
		RecentLearnings:          []learnings.Learning{{Category: "c", Content: "recent"}},
		RecentValidationFailures: []string{"failure"},
		CoverageState:            "coverage",
		TargetCriterion:          "criterion",
		PrevFailure:              "prev",
		SiblingTouchedPackages:   []string{"internal/prompt"},
	}

	ApplyPhaseProfile(ctx, "build")

	if ctx.Spec == "" {
		t.Fatalf("Spec should be preserved for build")
	}
	if ctx.ClaudeMD == "" {
		t.Fatalf("ClaudeMD should be preserved for build")
	}
	if ctx.Rules == "" {
		t.Fatalf("Rules should be preserved for build")
	}
	if len(ctx.ConfirmedLearnings) == 0 {
		t.Fatalf("ConfirmedLearnings should be preserved for build")
	}
	if len(ctx.RecentLearnings) != 0 {
		t.Fatalf("RecentLearnings should be pruned for build")
	}
	if len(ctx.RecentValidationFailures) == 0 {
		t.Fatalf("RecentValidationFailures should be preserved for build")
	}
	if ctx.CoverageState == "" {
		t.Fatalf("CoverageState should be preserved for build")
	}
	if ctx.TargetCriterion == "" {
		t.Fatalf("TargetCriterion should be preserved for build")
	}
	if ctx.PrevFailure == "" {
		t.Fatalf("PrevFailure should be preserved for build")
	}
	if len(ctx.SiblingTouchedPackages) == 0 {
		t.Fatalf("SiblingTouchedPackages should be preserved for build")
	}
}
