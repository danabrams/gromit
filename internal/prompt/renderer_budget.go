package prompt

import (
	"fmt"
	"os"
	"strings"

	"github.com/danabrams/gromit/internal/learnings"
)

// shapeBuildContext applies budget shaping to a Context before rendering.
// Returns the original context if budget is zero or context is under budget.
func (r *Renderer) shapeBuildContext(ctx *Context, phase string) (*Context, *ShapeReport) {
	if r == nil || r.budgetMaxChars <= 0 || ctx == nil {
		return ctx, nil
	}
	shaped, report := ShapeContextForBudget(ctx, r.budgetMaxChars, r.budgetLearningCapChars, phase)
	logBudgetTrim(report)
	return shaped, report
}

// shapeATDDBuildContext applies ATDD-specific trimming to a Context before rendering.
func (r *Renderer) shapeATDDBuildContext(ctx *Context) (*Context, *ShapeReport) {
	if r == nil || ctx == nil || r.atddPromptConfig == nil {
		return ctx, nil
	}
	return ShapeATDDContextForBudget(ctx, *r.atddPromptConfig)
}

// shapeReviewContext applies budget shaping to a ReviewContext before rendering.
func (r *Renderer) shapeReviewContext(ctx *ReviewContext) (*ReviewContext, *ShapeReport) {
	if r == nil || r.budgetMaxChars <= 0 || ctx == nil {
		return ctx, nil
	}
	shaped, report := ShapeReviewContextForBudget(ctx, r.budgetMaxChars, promptPhaseReview)
	logBudgetTrim(report)
	return shaped, report
}

// shapeThoroughReviewContext applies budget shaping to a ThoroughReviewContext before rendering.
func (r *Renderer) shapeThoroughReviewContext(ctx *ThoroughReviewContext) (*ThoroughReviewContext, *ShapeReport) {
	if r == nil || r.budgetMaxChars <= 0 || ctx == nil {
		return ctx, nil
	}
	shaped, report := ShapeThoroughReviewContextForBudget(ctx, r.budgetMaxChars, promptPhaseReview)
	logBudgetTrim(report)
	return shaped, report
}

func logBudgetTrim(report *ShapeReport) {
	if report == nil || len(report.TrimActions) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "Prompt budget: %d -> %d chars (trimmed: %s)\n",
		report.BeforeChars, report.AfterChars, strings.Join(report.TrimActions, ", "))
}

// ShapeATDDContextForBudget trims ATDD context to fit within cfg.MaxChars.
// Trim order:
// 1) Apply ATDD toggles and confirmed-learning cap
// 2) drop RecentLearnings
// 3) reduce/drop ConfirmedLearnings by remaining budget
// 4) replace Rules with build-relevant ATDD subset
// 5) truncate Spec with marker
// 6) drop Rules entirely
func ShapeATDDContextForBudget(ctx *Context, cfg ATDDPromptConfig) (*Context, *ShapeReport) {
	shaped := cloneMethodologyContext(ctx)
	if shaped == nil {
		return nil, newShapeReport(0, 0, map[string]int{})
	}

	beforeChars := measureContext(shaped)
	preShapeTokens := measureContextTokens(shaped)

	report := newShapeReport(beforeChars, preShapeTokens, sectionSizes(shaped))
	withinBudget := func() bool {
		return cfg.MaxChars > 0 && measureContext(shaped) <= cfg.MaxChars
	}

	if !cfg.IncludeRules {
		shaped.Rules = ""
		report.TrimActions = append(report.TrimActions, trimDropATDDRules)
	}

	if !cfg.IncludeSpec {
		shaped.Spec = ""
		report.TrimActions = append(report.TrimActions, trimDropATDDSpec)
	}

	if !cfg.IncludeClaudeMD {
		shaped.ClaudeMD = ""
		report.TrimActions = append(report.TrimActions, trimDropClaudeMD)
	}

	if cfg.MaxConfirmedLearningChars >= 0 {
		capped := capLearnings(shaped.ConfirmedLearnings, cfg.MaxConfirmedLearningChars)
		if len(capped) != len(shaped.ConfirmedLearnings) {
			shaped.ConfirmedLearnings = capped
			report.TrimActions = append(report.TrimActions, trimCapATDDLearnings)
			if withinBudget() {
				return finishReport(shaped, report)
			}
		}
	}

	if cfg.MaxChars <= 0 {
		return finishReport(shaped, report)
	}

	if withinBudget() {
		return finishReport(shaped, report)
	}

	if len(shaped.RecentLearnings) > 0 {
		shaped.RecentLearnings = []learnings.Learning{}
		report.TrimActions = append(report.TrimActions, trimDropRecentLearnings)
		if withinBudget() {
			return finishReport(shaped, report)
		}
	}

	if len(shaped.ConfirmedLearnings) > 0 {
		budgeted := cfg.MaxChars - measureContextWithoutConfirmedLearnings(shaped)
		capped := capLearnings(shaped.ConfirmedLearnings, budgeted)
		if len(capped) == 0 {
			shaped.ConfirmedLearnings = []learnings.Learning{}
			report.TrimActions = append(report.TrimActions, trimDropConfirmedLearnings)
			if withinBudget() {
				return finishReport(shaped, report)
			}
		} else if len(capped) < len(shaped.ConfirmedLearnings) {
			shaped.ConfirmedLearnings = capped
			report.TrimActions = append(report.TrimActions, trimCapConfirmedLearnings)
			if withinBudget() {
				return finishReport(shaped, report)
			}
		}
	}

	if cfg.IncludeRules {
		if filtered, ok := maybeATDDRuleSubset(shaped.Rules); ok {
			shaped.Rules = filtered
			report.TrimActions = append(report.TrimActions, trimReplaceATDDRules)
			if withinBudget() {
				return finishReport(shaped, report)
			}
		}
	}

	if shaped.Spec != "" {
		if truncated, ok := truncateFieldForBudget(shaped.Spec, measureContext(shaped), cfg.MaxChars); ok {
			shaped.Spec = truncated
			report.TrimActions = append(report.TrimActions, trimTruncateSpec)
			if withinBudget() {
				return finishReport(shaped, report)
			}
		}
	}

	if shaped.Rules != "" {
		shaped.Rules = ""
		report.TrimActions = append(report.TrimActions, trimDropATDDRules)
	}

	return finishReport(shaped, report)
}

func measureContextWithoutConfirmedLearnings(ctx *Context) int {
	if ctx == nil {
		return 0
	}
	measure := measureContext(ctx)
	for _, l := range ctx.ConfirmedLearnings {
		measure -= len(l.Content)
	}
	return measure
}

func maybeATDDRuleSubset(rules string) (string, bool) {
	if rules == "" {
		return rules, false
	}
	filtered := filterRulesByPhase(rules, promptPhaseBuild)
	if filtered == rules {
		return rules, false
	}
	return filtered, true
}
