package prompt

import "github.com/danabrams/gromit/internal/learnings"

var phaseProfiles = map[string]func(*Context){
	"decompose": applyDecomposeProfile,
	"red":       applyRedProfile,
	"build":     applyImplementationProfile,
	"green":     applyImplementationProfile,
	"refactor":  applyRefactorProfile,
}

// ApplyPhaseProfile prunes context fields that are not needed for the given phase.
func ApplyPhaseProfile(ctx *Context, phase string) {
	if ctx == nil {
		return
	}

	if apply, ok := phaseProfiles[phase]; ok && apply != nil {
		apply(ctx)
	}
}

func applyDecomposeProfile(ctx *Context) {
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

func applyRedProfile(ctx *Context) {
	ctx.ClaudeMD = ""
	ctx.ConfirmedLearnings = []learnings.Learning{}
	ctx.RecentLearnings = []learnings.Learning{}
	ctx.RecentValidationFailures = []string{}
	ctx.PrevFailure = ""
	ctx.SiblingTouchedPackages = []string{}
}

func applyImplementationProfile(ctx *Context) {
	ctx.RecentLearnings = []learnings.Learning{}
}

func applyRefactorProfile(ctx *Context) {
	ctx.Spec = ""
	ctx.ClaudeMD = ""
	ctx.ConfirmedLearnings = []learnings.Learning{}
	ctx.RecentLearnings = []learnings.Learning{}
	ctx.CoverageState = ""
	ctx.TargetCriterion = ""
	ctx.PrevFailure = ""
	ctx.SiblingTouchedPackages = []string{}
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
	}
}
