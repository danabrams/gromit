package prompt

import (
	"fmt"
	"strings"
)

// RenderBuild renders the build prompt for a bead.
func (r *Renderer) RenderBuild(ctx *Context) (string, error) {
	return r.renderBuildPrompt("build", "PROMPT_build.md", ctx)
}

// RenderAnalyze renders the failure analysis prompt.
func (r *Renderer) RenderAnalyze(ctx *AnalyzeContext) (string, error) {
	if r != nil {
		taskIdentity := ""
		failureContext := ""
		if ctx != nil {
			taskIdentity = formatBeadIdentity(ctx.BeadID, ctx.BeadTitle, "", "", 0)
			failureContext = ctx.FailureOutput
		}
		r.lastDiagnostics = r.computeDiagnostics("analyze", map[string]string{
			SectionTaskIdentity:   taskIdentity,
			SectionFailureContext: failureContext,
			SectionTemplateStatic: "PROMPT_analyze.md",
		})
	}
	return r.render("PROMPT_analyze.md", ctx)
}

// RenderLearn renders the success learning extraction prompt.
func (r *Renderer) RenderLearn(ctx *LearnContext) (string, error) {
	if r != nil {
		taskIdentity := ""
		planBody := ""
		if ctx != nil {
			taskIdentity = formatBeadIdentity(ctx.BeadID, ctx.BeadTitle, "", "", 0)
			planBody = ctx.Summary
		}
		r.lastDiagnostics = r.computeDiagnostics("learn", map[string]string{
			SectionTaskIdentity:   taskIdentity,
			SectionPlanBody:       planBody,
			SectionTemplateStatic: "PROMPT_learn.md",
		})
	}
	return r.render("PROMPT_learn.md", ctx)
}

// RenderValidate renders the validation prompt.
func (r *Renderer) RenderValidate(ctx *Context, commands []string) (string, error) {
	// Add commands to context for validation template.
	type ValidateContext struct {
		*Context
		Commands []string
	}
	vctx := &ValidateContext{Context: ctx, Commands: commands}
	if r != nil {
		r.lastDiagnostics = r.computeBuildDiagnostics("validate", ctx)
	}
	return r.render("PROMPT_validate.md", vctx)
}

// RenderDecompose renders the task decomposition prompt.
func (r *Renderer) RenderDecompose(ctx *DecomposeContext) (string, error) {
	if r != nil {
		taskIdentity := ""
		if ctx != nil {
			taskIdentity = formatTaskIdentity(ctx.Bead, ctx.ParentBead, 0, "")
		}
		r.lastDiagnostics = r.computeDiagnostics("decompose", map[string]string{
			SectionTaskIdentity:   taskIdentity,
			SectionTemplateStatic: "PROMPT_decompose.md",
		})
	}
	return r.render("PROMPT_decompose.md", ctx)
}

// RenderScope renders the scope estimation prompt.
func (r *Renderer) RenderScope(ctx *ScopeContext) (string, error) {
	if r != nil {
		taskIdentity := ""
		if ctx != nil {
			taskIdentity = formatTaskIdentity(ctx.Bead, ctx.ParentBead, 0, "")
		}
		r.lastDiagnostics = r.computeDiagnostics("scope", map[string]string{
			SectionTaskIdentity:   taskIdentity,
			SectionTemplateStatic: "PROMPT_scope.md",
		})
	}
	return r.render("PROMPT_scope.md", ctx)
}

// RenderPrecheck renders the precheck prompt.
func (r *Renderer) RenderPrecheck(ctx *PrecheckContext) (string, error) {
	if r != nil {
		taskIdentity := ""
		if ctx != nil {
			taskIdentity = formatTaskIdentity(ctx.Bead, ctx.ParentBead, 0, "")
		}
		r.lastDiagnostics = r.computeDiagnostics("precheck", map[string]string{
			SectionTaskIdentity:   taskIdentity,
			SectionTemplateStatic: "PROMPT_precheck.md",
		})
	}
	return r.render("PROMPT_precheck.md", ctx)
}

// RenderSpecAcceptance renders the spec acceptance prompt.
func (r *Renderer) RenderSpecAcceptance(ctx *SpecAcceptanceContext) (string, error) {
	if r != nil {
		rules := ""
		spec := ""
		if ctx != nil {
			rules = ctx.Rules
			spec = ctx.Spec
		}
		r.lastDiagnostics = r.computeDiagnostics("spec_acceptance", map[string]string{
			SectionRules:          rules,
			SectionSpec:           spec,
			SectionTemplateStatic: "PROMPT_spec_acceptance.md",
		})
	}
	return r.render("PROMPT_spec_acceptance.md", ctx)
}

// RenderSpecGate renders the spec gate prompt.
func (r *Renderer) RenderSpecGate(ctx *SpecGateContext) (string, error) {
	const specGateTemplate = "PROMPT_spec_gate.md"
	if r != nil {
		specCriteriaAndAcceptance := ""
		failureAndTestOutput := ""
		cumulativeDiff := ""
		if ctx != nil {
			specCriteriaAndAcceptance = ctx.SpecCriteria + "\n" + ctx.AcceptanceCriteria
			failureAndTestOutput = ctx.FailureOutput + "\n" + ctx.TestOutput
			cumulativeDiff = ctx.CumulativeDiff
		}
		r.lastDiagnostics = r.computeDiagnostics("spec_gate", map[string]string{
			SectionSpec:           specCriteriaAndAcceptance,
			SectionFailureContext: failureAndTestOutput,
			SectionDiff:           cumulativeDiff,
			SectionTemplateStatic: specGateTemplate,
		})
	}
	return r.render(specGateTemplate, ctx)
}

// RenderReview renders the light review prompt.
func (r *Renderer) RenderReview(ctx *ReviewContext) (string, error) {
	var shapeReport *ShapeReport
	ctx, shapeReport = r.shapeReviewContext(ctx)
	if r != nil {
		diagnostics := r.computeReviewDiagnostics(ctx)
		applyShapeReportToDiagnostics(diagnostics, shapeReport, r.budgetMaxChars)
		r.lastDiagnostics = diagnostics
	}
	return r.render("PROMPT_review.md", ctx)
}

// RenderThoroughReview renders the thorough review prompt.
func (r *Renderer) RenderThoroughReview(ctx *ThoroughReviewContext) (string, error) {
	var shapeReport *ShapeReport
	ctx, shapeReport = r.shapeThoroughReviewContext(ctx)
	if r != nil {
		diagnostics := r.computeThoroughReviewDiagnostics(ctx)
		applyShapeReportToDiagnostics(diagnostics, shapeReport, r.budgetMaxChars)
		r.lastDiagnostics = diagnostics
	}
	return r.render("PROMPT_thorough_review.md", ctx)
}

// RenderAcceptanceTests renders the acceptance tests prompt for ATDD workflow.
func (r *Renderer) RenderAcceptanceTests(ctx *Context) (string, error) {
	return r.renderBuildPrompt("acceptance_tests", "PROMPT_acceptance_tests.md", ctx)
}

// RenderATDDBuild renders the ATDD-aware build prompt.
func (r *Renderer) RenderATDDBuild(ctx *Context) (string, error) {
	return r.renderBuildPrompt("atdd_build", "PROMPT_atdd_build.md", ctx)
}

// RenderATDDDiagnostic renders the pass-before-build diagnostic prompt.
func (r *Renderer) RenderATDDDiagnostic(ctx *DiagnosticContext) (string, error) {
	const diagnosticTemplate = "PROMPT_atdd_diagnostic.md"
	if r != nil {
		taskIdentity := ""
		failureAndTestOutput := ""
		diffAndCriteria := ""
		if ctx != nil {
			taskIdentity = ctx.BeadTitle + "\n" + ctx.BeadDescription
			failureAndTestOutput = ctx.TestOutput
			diffAndCriteria = ctx.AcceptanceCriteria + "\n" + ctx.TestDiff
		}
		r.lastDiagnostics = r.computeDiagnostics("atdd_diagnostic", map[string]string{
			SectionTaskIdentity:   taskIdentity,
			SectionSpec:           diffAndCriteria,
			SectionFailureContext: failureAndTestOutput,
			SectionTemplateStatic: diagnosticTemplate,
		})
	}
	return r.render(diagnosticTemplate, ctx)
}

// RenderRefactor renders the refactor prompt for code quality improvements.
func (r *Renderer) RenderRefactor(ctx *Context) (string, error) {
	return r.renderBuildPrompt("refactor", "PROMPT_refactor.md", ctx)
}

// RenderTDDBuild renders the TDD-aware build prompt.
func (r *Renderer) RenderTDDBuild(ctx *Context) (string, error) {
	return r.renderBuildPrompt("tdd_build", "PROMPT_tdd_build.md", ctx)
}

// RenderTDDRed renders the TDD red-phase prompt.
func (r *Renderer) RenderTDDRed(ctx *TDDRedContext) (string, error) {
	if r != nil {
		rules := ""
		spec := ""
		taskIdentity := ""
		failure := ""
		if ctx != nil {
			rules = ctx.Rules
			spec = ctx.SpecExcerpt
			taskIdentity = formatBeadIdentity(ctx.BeadID, ctx.BeadTitle, "", "", 0)
			failure = ctx.FailureContext + "\n" + ctx.PrevFailure + "\n" + ctx.CycleSummary
		}
		r.lastDiagnostics = r.computeDiagnostics("tdd_red", map[string]string{
			SectionRules:          rules,
			SectionSpec:           spec,
			SectionTaskIdentity:   taskIdentity,
			SectionFailureContext: failure,
			SectionTemplateStatic: "PROMPT_tdd_red.md",
		})
	}
	return r.render("PROMPT_tdd_red.md", ctx)
}

// RenderTDDGreen renders the TDD green-phase prompt.
func (r *Renderer) RenderTDDGreen(ctx *TDDGreenContext) (string, error) {
	if r != nil {
		rules := ""
		taskIdentity := ""
		failure := ""
		if ctx != nil {
			rules = ctx.Rules
			taskIdentity = formatBeadIdentity(ctx.BeadID, ctx.BeadTitle, "", "", 0)
			failure = ctx.TestFailureOutput + "\n" + ctx.FailureContext + "\n" + ctx.PrevFailure
		}
		r.lastDiagnostics = r.computeDiagnostics("tdd_green", map[string]string{
			SectionRules:          rules,
			SectionTaskIdentity:   taskIdentity,
			SectionFailureContext: failure,
			SectionTemplateStatic: "PROMPT_tdd_green.md",
		})
	}
	return r.render("PROMPT_tdd_green.md", ctx)
}

// RenderTestFix renders the test-fix prompt for fixing implementation without changing tests.
func (r *Renderer) RenderTestFix(ctx *TestFixContext) (string, error) {
	if r != nil {
		claudeMD := ""
		rules := ""
		failure := ""
		testCommand := ""
		if ctx != nil {
			claudeMD = ctx.ClaudeMD
			rules = ctx.Rules
			failure = ctx.TestFailureOutput
			testCommand = ctx.TestCommand
		}
		r.lastDiagnostics = r.computeDiagnostics("test_fix", map[string]string{
			SectionClaudeMD:       claudeMD,
			SectionRules:          rules,
			SectionFailureContext: failure,
			SectionTaskIdentity:   testCommand,
			SectionTemplateStatic: "PROMPT_test_fix.md",
		})
	}
	return r.render("PROMPT_test_fix.md", ctx)
}

// RenderCoverageValidation renders the coverage validation prompt.
func (r *Renderer) RenderCoverageValidation(ctx *CoverageValidationContext) (string, error) {
	const coverageValidationTemplate = "PROMPT_coverage_validation.md"
	if r != nil {
		taskIdentity := "criterion=0"
		criterionAndTestCode := ""
		if ctx != nil {
			taskIdentity = fmt.Sprintf("criterion=%d", ctx.CriterionNumber)
			criterionAndTestCode = ctx.CriterionText + "\n" + ctx.TestCode
		}
		r.lastDiagnostics = r.computeDiagnostics("coverage_validation", map[string]string{
			SectionTaskIdentity:   taskIdentity,
			SectionPlanBody:       criterionAndTestCode,
			SectionTemplateStatic: coverageValidationTemplate,
		})
	}
	return r.render(coverageValidationTemplate, ctx)
}

func (r *Renderer) renderBuildPrompt(promptType, templateName string, ctx *Context) (string, error) {
	var shapeReport *ShapeReport
	ctx, shapeReport = r.shapeBuildContext(ctx, promptPhaseBuild)
	if r != nil {
		diagnostics := r.computeBuildDiagnostics(promptType, ctx)
		applyShapeReportToDiagnostics(diagnostics, shapeReport, r.budgetMaxChars)
		r.lastDiagnostics = diagnostics
	}
	return r.render(templateName, ctx)
}

func (r *Renderer) computeBuildDiagnostics(promptType string, ctx *Context) *PromptDiagnostics {
	if ctx == nil {
		return r.computeDiagnostics(promptType, map[string]string{
			SectionTemplateStatic: promptType,
		})
	}

	taskIdentity := formatTaskIdentity(ctx.Bead, ctx.ParentBead, ctx.Iteration, ctx.Model)
	confirmedLearnings := formatLearningsForDiagnostics(ctx.ConfirmedLearnings)
	recentLearnings := formatLearningsForDiagnostics(ctx.RecentLearnings)
	failureContext := ctx.FailureContext + "\n" + ctx.PrevFailure

	return r.computeDiagnostics(promptType, map[string]string{
		SectionClaudeMD:           ctx.ClaudeMD,
		SectionRules:              ctx.Rules,
		SectionSpec:               ctx.Spec,
		SectionConfirmedLearnings: confirmedLearnings,
		SectionRecentLearnings:    recentLearnings,
		SectionTaskIdentity:       taskIdentity,
		SectionFailureContext:     failureContext,
		SectionTemplateStatic:     promptType,
	})
}

func (r *Renderer) computeReviewDiagnostics(ctx *ReviewContext) *PromptDiagnostics {
	if ctx == nil {
		return r.computeDiagnostics("review", map[string]string{
			SectionTemplateStatic: "PROMPT_review.md",
		})
	}
	return r.computeDiagnostics("review", map[string]string{
		SectionClaudeMD:       ctx.ClaudeMD,
		SectionRules:          ctx.Rules,
		SectionSpec:           ctx.Spec,
		SectionDiff:           ctx.Diff,
		SectionTaskIdentity:   formatTaskIdentity(ctx.Bead, ctx.ParentBead, 0, ctx.Model),
		SectionTemplateStatic: "PROMPT_review.md",
	})
}

func (r *Renderer) computeThoroughReviewDiagnostics(ctx *ThoroughReviewContext) *PromptDiagnostics {
	if ctx == nil {
		return r.computeDiagnostics("thorough_review", map[string]string{
			SectionTemplateStatic: "PROMPT_thorough_review.md",
		})
	}
	beadSummaries := make([]string, 0, len(ctx.CompletedBeads))
	for _, completed := range ctx.CompletedBeads {
		beadSummaries = append(beadSummaries, formatBeadIdentity(completed.ID, completed.Title, completed.Description, "", 0))
	}

	return r.computeDiagnostics("thorough_review", map[string]string{
		SectionClaudeMD:       ctx.ClaudeMD,
		SectionRules:          ctx.Rules,
		SectionDiff:           ctx.Diff,
		SectionTaskIdentity:   strings.Join(beadSummaries, "\n"),
		SectionTemplateStatic: "PROMPT_thorough_review.md",
	})
}

func (r *Renderer) computeDiagnostics(promptType string, sections map[string]string) *PromptDiagnostics {
	if r == nil {
		return nil
	}
	if sections == nil {
		sections = map[string]string{}
	}
	return NewDiagnostics(promptType, EstimateSectionTokens(sections))
}

func applyShapeReportToDiagnostics(diagnostics *PromptDiagnostics, report *ShapeReport, budgetMaxChars int) {
	if diagnostics == nil || report == nil {
		return
	}

	diagnostics.BudgetMaxChars = budgetMaxChars
	diagnostics.ShapeActions = append([]string{}, report.TrimActions...)
	diagnostics.PreShapeTokens = report.PreShapeTokens
	diagnostics.PostShapeTokens = report.PostShapeTokens
}
