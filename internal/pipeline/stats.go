package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/backlog"
)

// StatsSummary describes aggregated data about backlog ideas.
type StatsSummary struct {
	Total  int            `json:"total"`
	ByType map[string]int `json:"by_type"`
}

// Stats returns aggregated backlog statistics, including counts by idea type.
func (p *Pipeline) Stats(ctx context.Context) (*StatsSummary, error) {
	if p == nil || p.deps == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}
	if err := requireNonNilDep("BacklogClient", p.deps.BacklogClient); err != nil {
		return nil, err
	}

	ideas, err := p.deps.BacklogClient.List()
	if err != nil {
		return nil, fmt.Errorf("listing backlog ideas: %w", err)
	}

	summary := &StatsSummary{ByType: map[string]int{}}
	summary.Total = len(ideas)

	for _, idea := range ideas {
		ideaType := classifyIdeaType(idea)
		summary.ByType[ideaType]++
	}

	return summary, nil
}

func classifyIdeaType(idea *Idea) string {
	if idea == nil {
		return "unknown"
	}

	normalized := normalizeIdeaType(idea.Type)
	if normalized == "" || normalized == "unknown" {
		return categorizeByText(idea.Text)
	}

	return normalized
}

func categorizeByText(text string) string {
	if strings.TrimSpace(text) == "" {
		return "unknown"
	}
	return backlog.CategorizeIdea(text)
}

func normalizeIdeaType(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
