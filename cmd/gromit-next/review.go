package main

import (
	"fmt"
	"sort"

	"github.com/danabrams/gromit/internal/next/enrich"
	"github.com/spf13/cobra"
)

var reviewInferredCmd = &cobra.Command{
	Use:   "review-inferred [project-name]",
	Short: "List inferred facts with their statuses",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := resolveProjectStore()
		if err != nil {
			return err
		}
		cell, err := store.Get(args[0])
		if err != nil {
			return err
		}

		cfg, err := enrich.LoadConfig(cell.CellPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		factStore := enrich.NewFactStore()
		facts, err := factStore.LoadFacts(cell.CellPath)
		if err != nil {
			return fmt.Errorf("load facts: %w", err)
		}

		if len(facts) == 0 {
			fmt.Println("No inferred facts found. Run 'gromit-next project enrich' first.")
			return nil
		}

		// Filter expired
		fresh := enrich.FilterExpired(facts, cfg.StalenessExpiryDays)
		if len(fresh) < len(facts) {
			fmt.Printf("Warning: %d expired facts excluded (older than %d days)\n\n", len(facts)-len(fresh), cfg.StalenessExpiryDays)
		}

		// Group by category
		grouped := make(map[string][]enrich.InferredFact)
		for _, f := range fresh {
			cat := string(f.Category)
			grouped[cat] = append(grouped[cat], f)
		}

		var cats []string
		for cat := range grouped {
			cats = append(cats, cat)
		}
		sort.Strings(cats)

		for _, cat := range cats {
			catFacts := grouped[cat]
			fmt.Printf("## %s\n", cat)
			for _, f := range catFacts {
				displayID := f.FactID
				if len(displayID) > 8 {
					displayID = displayID[:8]
				}
				fmt.Printf("  [%s] %s (confidence: %s, status: %s)\n", displayID, f.Statement, f.Confidence, f.Status)
			}
			fmt.Println()
		}

		return nil
	},
}

var acceptInferredCmd = &cobra.Command{
	Use:   "accept-inferred [project-name]",
	Short: "Accept an inferred fact",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		factID, _ := cmd.Flags().GetString("fact")
		if factID == "" {
			return fmt.Errorf("--fact flag is required")
		}
		return updateInferredStatus(args[0], factID, enrich.StatusAccepted)
	},
}

var rejectInferredCmd = &cobra.Command{
	Use:   "reject-inferred [project-name]",
	Short: "Reject an inferred fact",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		factID, _ := cmd.Flags().GetString("fact")
		if factID == "" {
			return fmt.Errorf("--fact flag is required")
		}
		return updateInferredStatus(args[0], factID, enrich.StatusRejected)
	},
}

func updateInferredStatus(projectName, factID string, status enrich.Status) error {
	store, err := resolveProjectStore()
	if err != nil {
		return err
	}
	cell, err := store.Get(projectName)
	if err != nil {
		return err
	}

	factStore := enrich.NewFactStore()
	if err := factStore.UpdateStatus(cell.CellPath, factID, status); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	fmt.Printf("Fact %s marked as %s\n", factID, status)
	return nil
}

func init() {
	acceptInferredCmd.Flags().String("fact", "", "fact ID to accept")
	rejectInferredCmd.Flags().String("fact", "", "fact ID to reject")
	projectCmd.AddCommand(reviewInferredCmd)
	projectCmd.AddCommand(acceptInferredCmd)
	projectCmd.AddCommand(rejectInferredCmd)
}
