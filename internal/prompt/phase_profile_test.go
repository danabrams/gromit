package prompt

import (
	"reflect"
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

func TestApplyReviewPhaseProfile_ThoroughReviewPreservesDeepReviewContext(t *testing.T) {
	ctx := &ThoroughReviewContext{
		Diff:           "diff",
		CompletedBeads: []CompletedBeadSummary{{ID: "b1", Title: "title"}},
		ClaudeMD:       "claude",
		Rules:          "rules",
		Model:          "opus",
	}

	ApplyReviewPhaseProfile(ctx, "thorough_review")

	if ctx.Diff == "" {
		t.Fatalf("Diff should be preserved for thorough_review")
	}
	if len(ctx.CompletedBeads) == 0 {
		t.Fatalf("CompletedBeads should be preserved for thorough_review")
	}
	if ctx.ClaudeMD == "" {
		t.Fatalf("ClaudeMD should be preserved for thorough_review")
	}
	if ctx.Rules == "" {
		t.Fatalf("Rules should be preserved for thorough_review")
	}
	if ctx.Model == "" {
		t.Fatalf("Model should be preserved for thorough_review")
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

func TestApplyPhaseProfile_GreenMatchesBuildImplementationContext(t *testing.T) {
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

	ApplyPhaseProfile(ctx, "green")

	if ctx.Spec == "" {
		t.Fatalf("Spec should be preserved for green")
	}
	if ctx.ClaudeMD == "" {
		t.Fatalf("ClaudeMD should be preserved for green")
	}
	if ctx.Rules == "" {
		t.Fatalf("Rules should be preserved for green")
	}
	if len(ctx.ConfirmedLearnings) == 0 {
		t.Fatalf("ConfirmedLearnings should be preserved for green")
	}
	if len(ctx.RecentLearnings) != 0 {
		t.Fatalf("RecentLearnings should be pruned for green")
	}
	if len(ctx.RecentValidationFailures) == 0 {
		t.Fatalf("RecentValidationFailures should be preserved for green")
	}
	if ctx.CoverageState == "" {
		t.Fatalf("CoverageState should be preserved for green")
	}
	if ctx.TargetCriterion == "" {
		t.Fatalf("TargetCriterion should be preserved for green")
	}
	if ctx.PrevFailure == "" {
		t.Fatalf("PrevFailure should be preserved for green")
	}
	if len(ctx.SiblingTouchedPackages) == 0 {
		t.Fatalf("SiblingTouchedPackages should be preserved for green")
	}
}

func TestApplyPhaseProfile_RefactorKeepsRulesAndValidationFailures(t *testing.T) {
	ctx := &Context{
		Spec:                     "spec",
		ClaudeMD:                 "claude",
		Rules:                    "rules",
		ConfirmedLearnings:       []learnings.Learning{{Category: "c", Content: "confirmed"}},
		RecentValidationFailures: []string{"failure"},
		CoverageState:            "coverage",
		TargetCriterion:          "criterion",
		PrevFailure:              "prev",
	}

	ApplyPhaseProfile(ctx, "refactor")

	if ctx.Rules == "" {
		t.Fatalf("Rules should be preserved for refactor")
	}
	if len(ctx.RecentValidationFailures) == 0 {
		t.Fatalf("RecentValidationFailures should be preserved for refactor")
	}
	if ctx.Spec != "" {
		t.Fatalf("Spec should be pruned for refactor")
	}
	if ctx.ClaudeMD != "" {
		t.Fatalf("ClaudeMD should be pruned for refactor")
	}
	if len(ctx.ConfirmedLearnings) != 0 {
		t.Fatalf("ConfirmedLearnings should be pruned for refactor")
	}
	if ctx.CoverageState != "" {
		t.Fatalf("CoverageState should be pruned for refactor")
	}
	if ctx.TargetCriterion != "" {
		t.Fatalf("TargetCriterion should be pruned for refactor")
	}
}

func TestApplyPhaseProfile_UnknownPhaseNoOp(t *testing.T) {
	baseCtx := &Context{
		Bead: &bead.Bead{
			ID:              "b1",
			ExpectedOutputs: []string{"criterion"},
		},
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
	originalCtx := *baseCtx
	originalCtx.Bead = &bead.Bead{
		ID:              baseCtx.Bead.ID,
		ExpectedOutputs: append([]string{}, baseCtx.Bead.ExpectedOutputs...),
	}
	originalCtx.ConfirmedLearnings = append([]learnings.Learning{}, baseCtx.ConfirmedLearnings...)
	originalCtx.RecentLearnings = append([]learnings.Learning{}, baseCtx.RecentLearnings...)
	originalCtx.RecentValidationFailures = append([]string{}, baseCtx.RecentValidationFailures...)
	originalCtx.SiblingTouchedPackages = append([]string{}, baseCtx.SiblingTouchedPackages...)

	ApplyPhaseProfile(baseCtx, "unknown_phase")

	if !reflect.DeepEqual(*baseCtx, originalCtx) {
		t.Fatalf("ApplyPhaseProfile should leave unknown phases unchanged")
	}

	reviewCtx := &ReviewContext{
		Spec:               "spec",
		Diff:               "diff",
		ClaudeMD:           "claude",
		Rules:              "rules",
		ValidationCommands: []string{"go test ./..."},
	}
	originalReview := *reviewCtx
	originalReview.ValidationCommands = append([]string{}, reviewCtx.ValidationCommands...)

	ApplyReviewPhaseProfile(reviewCtx, "unknown_phase")

	if !reflect.DeepEqual(*reviewCtx, originalReview) {
		t.Fatalf("ApplyReviewPhaseProfile should leave unknown phases unchanged")
	}
}
