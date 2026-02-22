package prompt

import "github.com/danabrams/gromit/internal/learnings"

// ApplyPhaseProfile prunes context fields that are not needed for the given phase.
func ApplyPhaseProfile(ctx *Context, phase string) {
	if ctx == nil {
		return
	}

	if phase != "decompose" {
		if phase == "red" {
			ctx.ClaudeMD = ""
			ctx.ConfirmedLearnings = []learnings.Learning{}
			ctx.RecentLearnings = []learnings.Learning{}
			ctx.RecentValidationFailures = []string{}
			ctx.PrevFailure = ""
			ctx.SiblingTouchedPackages = []string{}
		}
		return
	}

	ctx.Rules = ""
	ctx.ConfirmedLearnings = []learnings.Learning{}
	ctx.RecentLearnings = []learnings.Learning{}
	ctx.RecentValidationFailures = []string{}
	ctx.CoverageState = ""
	ctx.TargetCriterion = ""
	ctx.PrevFailure = ""
	ctx.SiblingTouchedPackages = []string{}
	if ctx.Bead != nil {
		ctx.Bead.ExpectedOutputs = []string{}
	}
}

// ApplyReviewPhaseProfile prunes review-context fields not needed for the given phase.
func ApplyReviewPhaseProfile(ctx any, phase string) {
	switch v := ctx.(type) {
	case *ReviewContext:
		if v == nil || phase != "review" {
			return
		}
		v.Spec = ""
		v.ClaudeMD = ""
		v.ValidationCommands = []string{}
	case *ThoroughReviewContext:
		if v == nil || phase != "thorough_review" {
			return
		}
		v.ClaudeMD = ""
	}
}
