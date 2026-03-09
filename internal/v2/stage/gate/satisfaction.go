package gate

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
)

// satisfactionTier returns the LLM tier for the pre-build satisfaction check
// based on bead generation. Gen 0 returns "" (skip). Gen 1 = low (haiku),
// gen 2 = medium (sonnet), gen 3+ = high (opus).
func satisfactionTier(generation int) string {
	switch {
	case generation <= 0:
		return ""
	case generation == 1:
		return "low"
	case generation == 2:
		return "medium"
	default:
		return "high"
	}
}

var structuralKeywords = []string{
	"refactor",
	"reorganize",
	"extract",
	"move",
	"rename",
	"add test",
	"test coverage",
	"integration test",
}

// isStructuralBead returns true if the bead's title or description indicates
// a refactoring or test-only change that should bypass satisfaction checks.
func isStructuralBead(title, description string) bool {
	combined := strings.ToLower(title + " " + description)
	for _, kw := range structuralKeywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

const satisfactionPromptTemplate = `You are evaluating whether a bead's acceptance criteria are ALREADY satisfied by existing code changes.

## Bead ID: %s

## Criterion to evaluate:
%s

## Cumulative diff (all changes on this branch):
%s

## Instructions
Evaluate whether this criterion is ALREADY satisfied by the code in the diff.
Output ONLY a JSON object: {"pass": true/false, "summary": "brief reason"}
`

// checkSatisfaction evaluates each acceptance criterion against the cumulative
// diff. Returns true only if ALL criteria pass. Returns false with nil error
// when no criteria are provided.
func checkSatisfaction(ctx context.Context, llm llmtypes.LLMProvider, tier, diff, beadID string, criteria []string) (bool, error) {
	if len(criteria) == 0 {
		return false, nil
	}

	for _, criterion := range criteria {
		prompt := fmt.Sprintf(satisfactionPromptTemplate, beadID, criterion, diff)
		resp, err := llm.Invoke(ctx, llmtypes.LLMInvokeRequest{
			Prompt: prompt,
			Model:  tier,
		})
		if err != nil {
			return false, fmt.Errorf("satisfaction check for %s: %w", beadID, err)
		}

		var eval struct {
			Pass    bool   `json:"pass"`
			Summary string `json:"summary"`
		}
		trimmed := strings.TrimSpace(resp.Output)
		if err := jsonutil.ExtractObject(trimmed, &eval); err != nil {
			return false, fmt.Errorf("parse satisfaction response: %w", err)
		}

		if !eval.Pass {
			return false, nil
		}
	}

	return true, nil
}
