package executor

import (
	"context"
	"fmt"
	"strings"
)

// RepairValidator checks whether the current worktree state passes validation.
type RepairValidator interface {
	Validate(ctx context.Context, workDir string) (pass bool, failures []string, err error)
}

// RepairInput holds the parameters for a repair loop.
type RepairInput struct {
	Agent      Agent
	Validator  RepairValidator
	MaxRetries int
	Packet     string
	WorkDir    string
	ModelTier  string
}

// RepairResult holds the outcome of a repair loop.
type RepairResult struct {
	Status      string // "done" or "failed"
	Attempts    int
	TotalTokens int
	TotalCost   float64
	Failures    []string
}

// RepairLoop runs an agent+validate cycle up to MaxRetries additional times
// after the initial attempt. Returns "done" on first passing validation,
// or "failed" if retries are exhausted.
func RepairLoop(ctx context.Context, input RepairInput) (RepairResult, error) {
	var totalTokens int
	var totalCost float64
	packet := input.Packet

	for attempt := 0; attempt <= input.MaxRetries; attempt++ {
		agentResult, err := input.Agent.Invoke(ctx, packet, input.ModelTier)
		if err != nil {
			return RepairResult{}, err
		}
		totalTokens += agentResult.TokensIn + agentResult.TokensOut
		totalCost += agentResult.Cost

		pass, failures, err := input.Validator.Validate(ctx, input.WorkDir)
		if err != nil {
			return RepairResult{}, err
		}

		if pass {
			return RepairResult{
				Status:      "done",
				Attempts:    attempt + 1,
				TotalTokens: totalTokens,
				TotalCost:   totalCost,
			}, nil
		}

		if attempt == input.MaxRetries {
			return RepairResult{
				Status:      "failed",
				Attempts:    attempt + 1,
				TotalTokens: totalTokens,
				TotalCost:   totalCost,
				Failures:    failures,
			}, nil
		}

		packet = buildRepairPrompt(input.Packet, failures)
	}

	// unreachable
	return RepairResult{}, nil
}

// buildRepairPrompt appends failure context to the original packet for the
// next repair attempt.
func buildRepairPrompt(originalPacket string, failures []string) string {
	var b strings.Builder
	b.WriteString(originalPacket)
	b.WriteString("\n\n## Previous Attempt Failed\n\n")
	b.WriteString("The following checks failed. Please fix them:\n\n")
	for _, f := range failures {
		b.WriteString(fmt.Sprintf("- %s\n", f))
	}
	return b.String()
}
