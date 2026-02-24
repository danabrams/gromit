package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func resolveBuildStrategy(cfg *config.Config, b *bead.Bead) string {
	strategy := "single_pass"
	if cfg != nil && cfg.Methodology.BuildStrategy != "" {
		strategy = cfg.Methodology.BuildStrategy
	}
	if b == nil {
		return strategy
	}
	for _, label := range b.Labels {
		switch label {
		case "build_strategy:tdd":
			strategy = "tdd"
		case "build_strategy:single_pass":
			strategy = "single_pass"
		}
	}
	return strategy
}

func extractRequirementsViaLLM(ctx context.Context, cfg *config.Config, title, description string, invoke func(ctx context.Context, prompt, tier string) (*provider.Result, error)) []string {
	const maxDescLen = 2000
	desc := description
	if len(desc) > maxDescLen {
		desc = desc[:maxDescLen]
	}
	promptText := fmt.Sprintf(`Extract the individual deliverables from the following task.
Title: %s
Description: %s

List each function, component, or independently testable item as a separate line.
Do not summarize. Do not group items. Return each individual requirement on its own line.`, title, desc)

	invokeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tier := resolveUtilityTaskTier(cfg, "summarization", provider.TierLow)
	result, err := invoke(invokeCtx, promptText, tier)
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

// isRequirementHeader returns true if line is a single word ending with ":"
// (case-insensitive), e.g. "Requirements:", "Functions:", "delivers:".
func isRequirementHeader(line string) bool {
	if len(line) < 2 || line[len(line)-1] != ':' {
		return false
	}
	word := line[:len(line)-1]
	for _, c := range word {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return true
}

// extractInlineHeader checks if line matches "Header: rest" where Header is a
// single word. Returns (rest, true) if matched or ("", false) otherwise.
func extractInlineHeader(line string) (string, bool) {
	idx := strings.Index(line, ":")
	if idx < 1 || idx >= len(line)-1 {
		return "", false
	}
	word := line[:idx]
	for _, c := range word {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return "", false
		}
	}
	rest := strings.TrimSpace(line[idx+1:])
	if rest == "" {
		return "", false
	}
	return rest, true
}

// splitCommaList splits text on commas, trims whitespace, and strips a leading
// "and" from the last item (Oxford comma). Returns nil if fewer than 2 items.
func splitCommaList(text string) []string {
	if !strings.Contains(text, ",") {
		return nil
	}
	parts := strings.Split(text, ",")
	var items []string
	for _, p := range parts {
		item := strings.TrimSpace(p)
		if item == "" || item == "and" {
			continue
		}
		item = strings.TrimPrefix(item, "and ")
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	if len(items) < 2 {
		return nil
	}
	return items
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
		// Standalone header line (e.g. "Functions:")
		if isRequirementHeader(line) {
			inHeaderSection = true
			continue
		}
		// Inline header with items (e.g. "Functions: X, Y, Z")
		if rest, ok := extractInlineHeader(line); ok {
			if commaItems := splitCommaList(rest); commaItems != nil {
				results = append(results, commaItems...)
			} else {
				results = append(results, rest)
			}
			inHeaderSection = false
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
		// Comma-separated list (e.g. "X, Y, Z")
		if commaItems := splitCommaList(line); commaItems != nil {
			results = append(results, commaItems...)
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
