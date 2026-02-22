package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func extractRequirementsViaLLM(ctx context.Context, title, description string, invoke func(ctx context.Context, prompt, tier string) (*provider.Result, error)) []string {
	const maxDescLen = 2000
	desc := description
	if len(desc) > maxDescLen {
		desc = desc[:maxDescLen]
	}
	promptText := fmt.Sprintf("Extract the requirements from the following task.\nTitle: %s\nDescription: %s\n\nReturn each requirement on its own line.", title, desc)

	invokeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := invoke(invokeCtx, promptText, provider.TierLow)
	if err != nil {
		return nil
	}

	var items []string
	for _, line := range strings.Split(result.Output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			items = append(items, line)
		}
	}
	if len(items) < 2 {
		return nil
	}
	return items
}

func buildCoverageTrackerFromSpec(bc *runtypes.BeadContext) (*coverage.CoverageTracker, []coverage.Criterion, error) {
	if bc == nil || bc.Bead == nil || bc.PromptCtx == nil {
		return nil, nil, nil
	}
	// Spec-level criteria can't be satisfied by individual beads and waste
	// 30-40 min per bead. The spec gate handles them after all beads complete.
	return nil, nil, nil
}

var requirementHeaders = []string{"Requirements:", "Includes:", "Delivers:"}

func isRequirementHeader(line string) bool {
	for _, h := range requirementHeaders {
		if line == h {
			return true
		}
	}
	return false
}

func extractRequirementsFromDescription(description string) []string {
	var results []string
	inHeaderSection := false
	for _, line := range strings.Split(description, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			inHeaderSection = false
			continue
		}
		if isRequirementHeader(line) {
			inHeaderSection = true
			continue
		}
		if len(line) >= 3 {
			i := 0
			for i < len(line) && line[i] >= '0' && line[i] <= '9' {
				i++
			}
			if i > 0 && i < len(line) && line[i] == '.' {
				item := strings.TrimSpace(line[i+1:])
				if item != "" {
					inHeaderSection = false
					results = append(results, item)
					continue
				}
			}
		}
		if len(line) >= 2 && (line[0] == '-' || line[0] == '*' || line[0] == '+') && line[1] == ' ' {
			item := strings.TrimSpace(line[1:])
			if item != "" {
				inHeaderSection = false
				results = append(results, item)
				continue
			}
		}
		if strings.Contains(line, ";") {
			for _, part := range strings.Split(line, ";") {
				item := strings.TrimSpace(part)
				if item != "" {
					results = append(results, item)
				}
			}
			inHeaderSection = false
			continue
		}
		if inHeaderSection {
			results = append(results, line)
		}
	}
	return results
}

func tddExpectedOutputsOrTitle(b *bead.Bead) []string {
	if b == nil {
		return []string{}
	}
	if len(b.ExpectedOutputs) > 0 {
		return append([]string(nil), b.ExpectedOutputs...)
	}
	if parsed := extractRequirementsFromDescription(b.Description); len(parsed) > 0 {
		return parsed
	}
	trimmedTitle := strings.TrimSpace(b.Title)
	if trimmedTitle == "" {
		return []string{}
	}
	return []string{trimmedTitle}
}
