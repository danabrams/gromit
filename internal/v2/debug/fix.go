package debug

import (
	"context"
	"fmt"
	"strings"
)

// SystemicFixInput captures the fix and diagnostic context for a systemic change.
type SystemicFixInput struct {
	FixCtx        *FixContext
	RootCause     RootCause
	FailureSignal string
}

// SystemicFixResult combines the code fix application outcome with any
// structured recommendation that requires human review.
type SystemicFixResult struct {
	*FixResult
	SystemicRecommendation string
}

// ApplySystemicFix applies the patch described in the fix context and, if the
// diff touches systemic assets, returns a recommendation for human review.
func ApplySystemicFix(ctx context.Context, input *SystemicFixInput) (*SystemicFixResult, error) {
	if input == nil {
		return nil, fmt.Errorf("systemic fix input is nil")
	}
	if input.FixCtx == nil {
		return nil, ErrNilFixContext
	}

	result, err := ApplyFix(ctx, input.FixCtx)
	if err != nil {
		return nil, err
	}

	recommendation := buildSystemicRecommendationForFix(input)
	return &SystemicFixResult{
		FixResult:              result,
		SystemicRecommendation: recommendation,
	}, nil
}

func buildSystemicRecommendationForFix(input *SystemicFixInput) string {
	if input == nil || input.FixCtx == nil {
		return ""
	}
	patch := strings.TrimSpace(input.FixCtx.CodePatch)
	categories := detectSystemicPatchCategories(patch)
	if len(categories) == 0 {
		return ""
	}

	signal := buildFailureSignal(input.FailureSignal, input.FixCtx, categories)
	if rec := BuildSystemicRecommendation(input.RootCause, signal); rec != "" {
		return rec
	}

	return fallbackSystemicRecommendation(signal, categories)
}

func buildFailureSignal(failureSignal string, fixCtx *FixContext, categories []string) string {
	if trimmed := strings.TrimSpace(failureSignal); trimmed != "" {
		return trimmed
	}
	if fixCtx != nil {
		if trimmed := strings.TrimSpace(fixCtx.ErrorMsg); trimmed != "" {
			return trimmed
		}
	}
	return failureSignalFromCategories(categories)
}

var systemicCategorySignals = map[string]string{
	systemicCategoryPromptFragment: "Systemic change touched prompt fragments that need clarity.",
	systemicCategoryGuard:          "Systemic change updated a pipeline guard that should block regressions.",
	systemicCategoryProcessRule:    "Systemic change modified process rules or workflow that require review.",
}

func failureSignalFromCategories(categories []string) string {
	if len(categories) == 0 {
		return ""
	}
	parts := make([]string, 0, len(categories))
	for _, category := range categories {
		if template, ok := systemicCategorySignals[category]; ok {
			parts = append(parts, template)
			continue
		}
		parts = append(parts, category)
	}
	return strings.Join(parts, " ")
}

func fallbackSystemicRecommendation(signal string, categories []string) string {
	categoryDesc := strings.Join(categories, ", ")
	if strings.TrimSpace(signal) == "" {
		signal = fmt.Sprintf("Patch updates %s assets that require human review.", categoryDesc)
	}

	return fmt.Sprintf(
		"Systemic recommendation for human review: patch touches %s. Rationale: %s Awaiting human approval before applying these changes.",
		categoryDesc,
		signal,
	)
}
