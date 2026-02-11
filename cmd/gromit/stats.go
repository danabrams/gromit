package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

func init() {
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "Output stats as JSON")
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

	gromitDir := resolveGromitDir(cfg)
	logsDir := filepath.Join(gromitDir, "logs")

	// Read project stats
	projectStats, err := logger.ReadModelStats(logsDir)
	if err != nil {
		return fmt.Errorf("reading project stats: %w", err)
	}

	// Read cost per completed bead
	beadCosts, err := logger.CostPerCompletedBead(logsDir)
	if err != nil {
		return fmt.Errorf("computing cost per bead: %w", err)
	}

	// Read global stats
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}
	globalStatsPath := filepath.Join(homeDir, ".gromit", "stats.json")
	globalStats, err := logger.ReadGlobalStats(globalStatsPath)
	if err != nil {
		return fmt.Errorf("reading global stats: %w", err)
	}

	if statsJSON {
		return outputJSON(projectStats, globalStats, beadCosts)
	}

	return outputText(projectStats, globalStats, beadCosts)
}

func outputJSON(projectStats map[string]logger.ModelStats, globalStats *logger.GlobalStats, beadCosts map[string]float64) error {
	output := map[string]interface{}{
		"project_stats": projectStats,
		"global_stats":  globalStats,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func outputText(projectStats map[string]logger.ModelStats, globalStats *logger.GlobalStats, beadCosts map[string]float64) error {
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

	if globalStats != nil && len(globalStats.Models) > 0 {
		fmt.Println()
		fmt.Println("Global Model Performance (all projects, global aggregate):")
		fmt.Println()
		printGlobalModelStats(globalStats.Models)
	}

	return nil
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
