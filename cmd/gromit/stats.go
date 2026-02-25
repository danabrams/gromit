package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Display model performance statistics",
	Long: `Display per-model statistics including success rates, iteration counts,
cost per completed bead, and escalation frequency for both project and global scopes.

Examples:
  gromit stats              # Show project and global stats
  gromit stats --json       # Output stats as JSON`,
	RunE: runStats,
}

var statsJSON bool
var statsTDD bool

func init() {
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "Output stats as JSON")
	statsCmd.Flags().BoolVar(&statsTDD, "tdd", false, "Include TDD phase/cycle metrics")
	rootCmd.AddCommand(statsCmd)
}

func runStats(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := loadConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("loading config: %w", err)
		}
		cfg = nil
	}

	statsData, err := loadStatsData(cfg)
	if err != nil {
		return err
	}

	if statsJSON {
		return outputJSON(statsData.projectStats, statsData.globalStats, statsData.costPerSpec, statsData.tddMetrics)
	}

	return outputText(statsData.projectStats, statsData.globalStats, statsData.beadCosts, statsData.costPerSpec, statsData.tddMetrics)
}

type statsData struct {
	projectStats map[string]logger.ModelStats
	globalStats  *logger.GlobalStats
	beadCosts    map[string]float64
	costPerSpec  map[string]logger.SpecCost
	tddMetrics   *logger.TDDStats
}

func loadStatsData(cfg *config.Config) (*statsData, error) {
	gromitDir := resolveGromitDir(cfg)
	logsDir := filepath.Join(gromitDir, "logs")

	// Read project stats
	projectStats, err := logger.ReadModelStats(logsDir)
	if err != nil {
		return nil, fmt.Errorf("reading project stats: %w", err)
	}

	// Read cost per completed bead
	beadCosts, err := logger.CostPerCompletedBead(logsDir)
	if err != nil {
		return nil, fmt.Errorf("computing cost per bead: %w", err)
	}

	// Read cost per spec
	costPerSpec, err := logger.CostPerSpec(logsDir)
	if err != nil {
		return nil, fmt.Errorf("computing cost per spec: %w", err)
	}

	// Read global stats
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}
	globalStatsPath := filepath.Join(homeDir, ".gromit", "stats.json")
	globalStats, err := logger.ReadGlobalStats(globalStatsPath)
	if err != nil {
		return nil, fmt.Errorf("reading global stats: %w", err)
	}

	var tddMetrics *logger.TDDStats
	if statsTDD {
		metrics, err := loadTDDMetrics(logsDir)
		if err != nil {
			return nil, err
		}
		tddMetrics = metrics
	}

	return &statsData{
		projectStats: projectStats,
		globalStats:  globalStats,
		beadCosts:    beadCosts,
		costPerSpec:  costPerSpec,
		tddMetrics:   tddMetrics,
	}, nil
}

func loadTDDMetrics(logsDir string) (*logger.TDDStats, error) {
	if _, err := logger.ReadTDDPhaseRecords(logsDir); err != nil {
		return nil, fmt.Errorf("reading tdd phase records: %w", err)
	}
	if _, err := logger.ReadTDDSummaries(logsDir); err != nil {
		return nil, fmt.Errorf("reading tdd summaries: %w", err)
	}
	metrics, err := logger.AggregateTDDStats(logsDir)
	if err != nil {
		return nil, fmt.Errorf("aggregating tdd stats: %w", err)
	}
	return &metrics, nil
}

type statsJSONOutput struct {
	ProjectStats map[string]logger.ModelStats `json:"project_stats"`
	GlobalStats  *logger.GlobalStats          `json:"global_stats"`
	CostPerSpec  map[string]logger.SpecCost   `json:"cost_per_spec"`
	TDDMetrics   *logger.TDDStats             `json:"tdd_metrics"`
}

func outputJSON(projectStats map[string]logger.ModelStats, globalStats *logger.GlobalStats, costPerSpec map[string]logger.SpecCost, tddMetrics *logger.TDDStats) error {
	output := statsJSONOutput{
		ProjectStats: projectStats,
		GlobalStats:  globalStats,
		CostPerSpec:  costPerSpec,
		TDDMetrics:   tddMetrics,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func outputText(projectStats map[string]logger.ModelStats, globalStats *logger.GlobalStats, beadCosts map[string]float64, costPerSpec map[string]logger.SpecCost, tddMetrics *logger.TDDStats) error {
	fmt.Println("Project Model Performance (Escalation rates shown):")
	fmt.Println()
	printProjectModelStats(projectStats)

	if len(beadCosts) > 0 {
		fmt.Println()
		fmt.Println("cost per completed bead (including retries):")
		for beadID, cost := range beadCosts {
			fmt.Printf("  %s: $%.2f\n", beadID, cost)
		}
	}

	if len(costPerSpec) > 0 {
		fmt.Println()
		fmt.Println("Cost per spec:")
		for _, entry := range sortedSpecCosts(costPerSpec) {
			fmt.Printf("  %s: $%.2f  %d iter  %d beads  [%s]\n",
				entry.specID, entry.cost.TotalCostUSD,
				entry.cost.Iterations, entry.cost.Beads,
				formatModelMix(entry.cost.ModelMix))
		}
	}

	if globalStats != nil && len(globalStats.Models) > 0 {
		fmt.Println()
		fmt.Println("Global Model Performance (all projects, global aggregate):")
		fmt.Println()
		printGlobalModelStats(globalStats.Models)
	}

	if tddMetrics != nil {
		fmt.Println()
		printTDDMetrics(*tddMetrics)
	}

	return nil
}

type specCostEntry struct {
	specID string
	cost   logger.SpecCost
}

func sortedSpecCosts(costs map[string]logger.SpecCost) []specCostEntry {
	entries := make([]specCostEntry, 0, len(costs))
	for specID, cost := range costs {
		entries = append(entries, specCostEntry{specID: specID, cost: cost})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].cost.TotalCostUSD == entries[j].cost.TotalCostUSD {
			return entries[i].specID < entries[j].specID
		}
		return entries[i].cost.TotalCostUSD > entries[j].cost.TotalCostUSD
	})
	return entries
}

func printProjectModelStats(stats map[string]logger.ModelStats) {
	for model, s := range stats {
		printModelLine(model, s.SuccessRate()*100, s.Successes, s.Iterations, s.TotalCostUSD)
		printEscalations(s.EscalationsFrom, s.EscalationsTo)
		fmt.Println()
	}
}

func printGlobalModelStats(stats map[string]*logger.GlobalModelStats) {
	for model, s := range stats {
		successRate := calculateSuccessRate(s.Successes, s.Iterations)
		printModelLine(model, successRate, s.Successes, s.Iterations, s.TotalCostUSD)
		printEscalations(s.EscalationsFrom, s.EscalationsTo)
		fmt.Println()
	}
}

func printModelLine(model string, successRate float64, successes, iterations int, totalCost float64) {
	avgCost := calculateAvgCost(totalCost, iterations)
	fmt.Printf("  %s  %.0f%% success  (%d/%d)  avg $%.2f/iter",
		model, successRate, successes, iterations, avgCost)
}

func printEscalations(escalationsFrom, escalationsTo int) {
	if escalationsFrom > 0 || escalationsTo > 0 {
		fmt.Printf("  escalation:")
		if escalationsFrom > 0 {
			fmt.Printf(" from=%d", escalationsFrom)
		}
		if escalationsTo > 0 {
			fmt.Printf(" to=%d", escalationsTo)
		}
	}
}

func calculateSuccessRate(successes, iterations int) float64 {
	if iterations == 0 {
		return 0.0
	}
	return float64(successes) / float64(iterations) * 100
}

func calculateAvgCost(totalCost float64, iterations int) float64 {
	if iterations == 0 {
		return 0.0
	}
	return totalCost / float64(iterations)
}

// formatModelMix formats the model mix map as a sorted "model:count" string
func formatModelMix(mix map[string]int) string {
	models := make([]string, 0, len(mix))
	for m := range mix {
		models = append(models, m)
	}
	sort.Strings(models)
	parts := make([]string, 0, len(models))
	for _, m := range models {
		parts = append(parts, fmt.Sprintf("%s:%d", m, mix[m]))
	}
	return strings.Join(parts, ", ")
}

func printTDDMetrics(stats logger.TDDStats) {
	fmt.Println("TDD Metrics:")
	fmt.Printf("  avg cycles/bead: %.2f\n", stats.AvgCyclesPerBead)
	fmt.Printf("  avg cost/cycle: $%.2f\n", stats.AvgCostUSDPerCycle)
	fmt.Printf("  avg tokens/cycle: in=%.0f out=%.0f\n", stats.AvgInputTokensCycle, stats.AvgOutputTokensCycle)

	if len(stats.PhaseSuccessRates) > 0 {
		fmt.Println("  per-phase success rates:")
		for _, phase := range sortedStringFloatKeys(stats.PhaseSuccessRates) {
			fmt.Printf("    %s: %.0f%%\n", phase, stats.PhaseSuccessRates[phase]*100)
		}
	}

	if len(stats.EscalationPatterns) > 0 {
		fmt.Println("  escalation counts:")
		for _, pattern := range sortedStringIntKeys(stats.EscalationPatterns) {
			fmt.Printf("    %s: %d\n", pattern, stats.EscalationPatterns[pattern])
		}
	}
}

func sortedStringFloatKeys(values map[string]float64) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedStringIntKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
