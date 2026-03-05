package pipeline

import (
	"context"
	"fmt"
	"github.com/danabrams/gromit/internal/analytics"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/config"
)

const defaultRoutingStrategy = "priority_based"

// StatsOptions configures how pipeline statistics are aggregated.
type StatsOptions struct {
	IncludeTDD bool
}

// StatsSummary holds all data needed to render the stats command output.
type StatsSummary struct {
	ProjectStats    map[string]analytics.ModelStats `json:"project_stats"`
	GlobalStats     *analytics.GlobalStats          `json:"global_stats"`
	BeadCosts       map[string]float64              `json:"bead_costs"`
	CostPerSpec     map[string]analytics.SpecCost   `json:"cost_per_spec"`
	ProviderMetrics []analytics.ProviderMetrics     `json:"provider_metrics"`
	RoutingStrategy string                          `json:"routing_strategy"`
	TDDMetrics      *analytics.TDDStats             `json:"tdd_metrics"`
	Backlog         *BacklogSummary                 `json:"backlog,omitempty"`
}

// BacklogSummary reports counts of backlog ideas by type.
type BacklogSummary struct {
	Total  int            `json:"total"`
	ByType map[string]int `json:"by_type"`
}

// Stats aggregates all of the data required by the CLI stats command.
func (p *Pipeline) Stats(ctx context.Context, cfg *config.Config, opts StatsOptions) (*StatsSummary, error) {
	if p == nil {
		return nil, fmt.Errorf("pipeline: nil pipeline")
	}
	if p.paths == nil {
		return nil, fmt.Errorf("pipeline: missing paths")
	}

	gromitDir := p.paths.GromitDir
	if gromitDir == "" {
		gromitDir = ".gromit"
	}

	summary, err := loadPipelineStats(gromitDir, cfg, opts.IncludeTDD, p.deps)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

func loadPipelineStats(gromitDir string, cfg *config.Config, includeTDD bool, deps *Deps) (*StatsSummary, error) {
	logsDir := filepath.Join(gromitDir, "logs")
	metricsDir := filepath.Join(gromitDir, "metrics")

	projectStats, err := analytics.ReadModelStats(logsDir)
	if err != nil {
		return nil, fmt.Errorf("reading project stats: %w", err)
	}

	beadCosts, err := analytics.CostPerCompletedBead(logsDir)
	if err != nil {
		return nil, fmt.Errorf("computing cost per bead: %w", err)
	}

	costPerSpec, err := analytics.CostPerSpec(logsDir)
	if err != nil {
		return nil, fmt.Errorf("computing cost per spec: %w", err)
	}
	costPerSpec = filterSpecCosts(costPerSpec)

	var providerMetrics []analytics.ProviderMetrics
	processTrendPath := filepath.Join(metricsDir, "process_trend.json")
	processTrend, err := analytics.ReadProcessTrend(processTrendPath)
	if err != nil {
		return nil, fmt.Errorf("reading process trend: %w", err)
	}
	if processTrend != nil {
		providerMetrics = processTrend.ProviderMetrics
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}
	globalStatsPath := filepath.Join(homeDir, ".gromit", "stats.json")
	globalStats, err := analytics.ReadGlobalStats(globalStatsPath)
	if err != nil {
		return nil, fmt.Errorf("reading global stats: %w", err)
	}

	var tddMetrics *analytics.TDDStats
	if includeTDD {
		metrics, err := loadTDDMetrics(logsDir)
		if err != nil {
			return nil, err
		}
		tddMetrics = metrics
	}

	backlogSummary, err := collectBacklogSummary(deps)
	if err != nil {
		return nil, err
	}

	routingStrategy := defaultRoutingStrategy
	if cfg != nil && cfg.Routing.Strategy != "" {
		routingStrategy = cfg.Routing.Strategy
	}

	return &StatsSummary{
		ProjectStats:    projectStats,
		GlobalStats:     globalStats,
		BeadCosts:       beadCosts,
		CostPerSpec:     costPerSpec,
		ProviderMetrics: providerMetrics,
		RoutingStrategy: routingStrategy,
		TDDMetrics:      tddMetrics,
		Backlog:         backlogSummary,
	}, nil
}

func loadTDDMetrics(logsDir string) (*analytics.TDDStats, error) {
	if _, err := analytics.ReadTDDPhaseRecords(logsDir); err != nil {
		return nil, fmt.Errorf("reading tdd phase records: %w", err)
	}
	if _, err := analytics.ReadTDDSummaries(logsDir); err != nil {
		return nil, fmt.Errorf("reading tdd summaries: %w", err)
	}
	metrics, err := analytics.AggregateTDDStats(logsDir)
	if err != nil {
		return nil, fmt.Errorf("aggregating tdd stats: %w", err)
	}
	return &metrics, nil
}

func collectBacklogSummary(deps *Deps) (*BacklogSummary, error) {
	if deps == nil || deps.BacklogClient == nil {
		return nil, nil
	}

	ideas, err := deps.BacklogClient.List()
	if err != nil {
		return nil, fmt.Errorf("listing backlog ideas: %w", err)
	}

	summary := &BacklogSummary{
		Total:  len(ideas),
		ByType: map[string]int{},
	}

	for _, idea := range ideas {
		summary.ByType[classifyIdeaType(idea)]++
	}

	return summary, nil
}

func filterSpecCosts(costs map[string]analytics.SpecCost) map[string]analytics.SpecCost {
	filtered := make(map[string]analytics.SpecCost, len(costs))
	for specID, cost := range costs {
		if specID == analytics.UnassignedSpecID {
			continue
		}
		filtered[specID] = cost
	}
	return filtered
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
