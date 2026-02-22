package prompt

import "github.com/danabrams/gromit/internal/learnings"

// ApplyPhaseProfile prunes context fields that are not needed for the given phase.
func ApplyPhaseProfile(ctx *Context, phase string) {
	if ctx == nil {
		return
	}

	if phase != "decompose" {
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
