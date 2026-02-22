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
