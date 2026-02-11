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
	for model, stats := range projectStats {
		successRate := stats.SuccessRate() * 100
		avgCost := 0.0
		if stats.Iterations > 0 {
			avgCost = stats.TotalCostUSD / float64(stats.Iterations)
		}

		fmt.Printf("  %s  %.0f%% success  (%d/%d)  avg $%.2f/iter",
			model,
			successRate,
			stats.Successes,
			stats.Iterations,
			avgCost)

		// Show escalation info
		if stats.EscalationsFrom > 0 || stats.EscalationsTo > 0 {
			fmt.Printf("  escalation:")
			if stats.EscalationsFrom > 0 {
				fmt.Printf(" from=%d", stats.EscalationsFrom)
			}
			if stats.EscalationsTo > 0 {
				fmt.Printf(" to=%d", stats.EscalationsTo)
			}
		}

		fmt.Println()
	}

	// Display bead costs
	if len(beadCosts) > 0 {
		fmt.Println()
		fmt.Println("cost per completed bead (including retries):")
		for beadID, cost := range beadCosts {
			fmt.Printf("  %s: $%.2f\n", beadID, cost)
		}
	}

	// Display global stats
	if globalStats != nil && len(globalStats.Models) > 0 {
		fmt.Println()
		fmt.Println("Global Model Performance (all projects):")
		fmt.Println()

		for model, stats := range globalStats.Models {
			successRate := 0.0
			if stats.Iterations > 0 {
				successRate = float64(stats.Successes) / float64(stats.Iterations) * 100
			}
			avgCost := 0.0
			if stats.Iterations > 0 {
				avgCost = stats.TotalCostUSD / float64(stats.Iterations)
			}

			fmt.Printf("  %s  %.0f%% success  (%d/%d)  avg $%.2f/iter",
				model,
				successRate,
				stats.Successes,
				stats.Iterations,
				avgCost)

			// Show Escalation info
			if stats.EscalationsFrom > 0 {
				fmt.Printf("  Escalation from: %d", stats.EscalationsFrom)
			}
			if stats.EscalationsTo > 0 {
				fmt.Printf("  Escalation to: %d", stats.EscalationsTo)
			}

			fmt.Println()
		}
	}

	return nil
}
