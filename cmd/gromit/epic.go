package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/frontmatter"
	"github.com/spf13/cobra"
)

var epicCmd = &cobra.Command{
	Use:   "epic",
	Short: "Manage epics",
	Long:  `Commands for managing epics (problem spaces and explorations).`,
}

var epicStatusCmd = &cobra.Command{
	Use:   "status <epic-id>",
	Short: "Show epic status and linked specs",
	Long:  `Display epic title, linked specs, their pipeline stages, and bead progress.`,
	Args:  cobra.ExactArgs(1),
	RunE:  epicStatus,
}

func init() {
	rootCmd.AddCommand(epicCmd)
	epicCmd.AddCommand(epicStatusCmd)
}

func epicStatus(cmd *cobra.Command, args []string) error {
	epicID := args[0]

	// Load config
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Determine epic and spec directories
	epicsDir := filepath.Join(cfg.Paths.GromitDir, "epics")
	specsDir := cfg.Paths.Specs
	if specsDir == "" {
		specsDir = filepath.Join(cfg.Paths.GromitDir, "specs")
	}

	// Find and read epic document
	epicPath, epicTitle, err := findEpicByID(epicID, epicsDir)
	if err != nil {
		return err
	}

	// Find specs linked to this epic
	linkedSpecs, err := findLinkedSpecs(epicID, specsDir, cfg)
	if err != nil {
		return fmt.Errorf("finding linked specs: %w", err)
	}

	// Display status
	fmt.Println("=" + strings.Repeat("=", 78))
	fmt.Printf("EPIC: %s\n", epicTitle)
	fmt.Printf("ID: %s\n", epicID)
	fmt.Println("=" + strings.Repeat("=", 78))

	if len(linkedSpecs) == 0 {
		fmt.Println("No linked specs.")
	} else {
		fmt.Println("\nLinked Specs:")
		fmt.Println()

		// Create bead client for querying beads
		beadClient, err := bead.NewClient()
		if err == nil {
			// Only show bead counts if bd is available
			for _, spec := range linkedSpecs {
				stage := determinePipelineStage(spec.id, cfg)

				// Get bead counts for this spec
				openCount, closedCount, beadErr := getBeadCounts(beadClient, spec.id)

				fmt.Printf("  %s\n", spec.id)
				fmt.Printf("    Title: %s\n", spec.title)
				fmt.Printf("    Stage: %s\n", stage)

				// Show bead progress if we can query beads
				if beadErr == nil {
					totalCount := openCount + closedCount
					if totalCount > 0 {
						fmt.Printf("    Beads: %d open, %d closed (%d total)\n", openCount, closedCount, totalCount)
					} else {
						fmt.Printf("    Beads: none\n")
					}
				}

				fmt.Println()
			}
		} else {
			// If bd is not available, show specs without bead counts
			for _, spec := range linkedSpecs {
				stage := determinePipelineStage(spec.id, cfg)
				fmt.Printf("  %s\n", spec.id)
				fmt.Printf("    Title: %s\n", spec.title)
				fmt.Printf("    Stage: %s\n", stage)
				fmt.Println()
			}
		}
	}

	// Perform gap analysis
	fmt.Println("\nGap Analysis:")
	fmt.Println()

	// Read epic content
	epicContent, err := os.ReadFile(epicPath)
	if err != nil {
		return fmt.Errorf("reading epic file: %w", err)
	}

	// Create Claude client
	claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
	if err != nil {
		return fmt.Errorf("creating claude client: %w", err)
	}

	// Build spec summaries
	specSummaries := buildSpecSummaries(linkedSpecs)

	// Determine model to use (haiku for cost efficiency)
	model := cfg.Models.P2 // P2 is haiku by default
	if model == "" {
		model = "claude-haiku-4.5-20251001"
	}

	// Run gap analysis
	analysis, err := performGapAnalysis(claudeClient, model, string(epicContent), specSummaries)
	if err != nil {
		return fmt.Errorf("performing gap analysis: %w", err)
	}

	// Print analysis
	fmt.Println(analysis)

	return nil
}

func findEpicByID(epicID string, epicsDir string) (string, string, error) {
	// List all files in epics directory
	entries, err := os.ReadDir(epicsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("epic not found: no epic document for %s", epicID)
		}
		return "", "", fmt.Errorf("reading epics directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(epicsDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		fm, body, err := frontmatter.Parse(string(content))
		if err != nil {
			continue
		}

		if fm["epic_id"] == epicID {
			// Extract title from body (first h1)
			title := extractTitle(body)
			return path, title, nil
		}
	}

	return "", "", fmt.Errorf("epic not found: no epic document for %s", epicID)
}

type spec struct {
	id    string
	title string
}

func findLinkedSpecs(epicID string, specsDir string, cfg *config.Config) ([]spec, error) {
	var linked []spec

	// List all files in specs directory
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return linked, nil
		}
		return nil, fmt.Errorf("reading specs directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(specsDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		fm, body, err := frontmatter.Parse(string(content))
		if err != nil {
			continue
		}

		// Check if this spec references our epic
		if fm["epic"] == epicID {
			specID := fm["id"].(string)
			if specID == "" {
				continue
			}
			title := extractTitle(body)
			linked = append(linked, spec{id: specID, title: title})
		}
	}

	return linked, nil
}

func determinePipelineStage(specID string, cfg *config.Config) string {
	plansDir := cfg.Paths.Plans
	if plansDir == "" {
		plansDir = filepath.Join(cfg.Paths.GromitDir, "plans")
	}

	// Look for plan file matching spec ID
	planPath := filepath.Join(plansDir, specID+".md")
	content, err := os.ReadFile(planPath)
	if err != nil {
		return "unplanned"
	}

	fm, _, err := frontmatter.Parse(string(content))
	if err != nil {
		return "unplanned"
	}

	// Check if plan is decomposed
	decomposed, ok := fm["decomposed"].(bool)
	if !ok {
		// Try as string
		if decomposedStr, ok := fm["decomposed"].(string); ok {
			decomposed = decomposedStr == "true"
		}
	}

	if decomposed {
		return "decomposed"
	}
	return "planned"
}

func extractTitle(body string) string {
	// Find first h1 heading in the body
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

// claudeClient defines the interface for Claude CLI operations needed by gap analysis
type claudeClient interface {
	Run(ctx context.Context, prompt string, model string) (*claude.Result, error)
}

// buildSpecSummaries creates formatted spec summaries for gap analysis
func buildSpecSummaries(specs []spec) []string {
	summaries := make([]string, len(specs))
	for i, s := range specs {
		summaries[i] = fmt.Sprintf("%s: %s", s.id, s.title)
	}
	return summaries
}

// getBeadCounts returns the number of open and closed beads for a given spec
func getBeadCounts(client *bead.Client, specID string) (open int, closed int, err error) {
	// Query beads with label spec:<specID>
	label := "spec:" + specID
	beads, err := client.ListWithLabel(label)
	if err != nil {
		return 0, 0, err
	}

	// Count open and closed beads
	for _, b := range beads {
		if b.Status == "closed" {
			closed++
		} else {
			open++
		}
	}

	return open, closed, nil
}

// performGapAnalysis runs LLM gap analysis on epic content and spec summaries
func performGapAnalysis(client claudeClient, model string, epicContent string, specSummaries []string) (string, error) {
	// Build prompt with instructions, epic content, and spec summaries
	var promptBuilder strings.Builder

	promptBuilder.WriteString("Analyze the epic below and identify what areas are not covered by existing specs.\n\n")
	promptBuilder.WriteString("Epic:\n")
	promptBuilder.WriteString(epicContent)
	promptBuilder.WriteString("\n\n")

	if len(specSummaries) > 0 {
		promptBuilder.WriteString("Existing specs:\n")
		for _, summary := range specSummaries {
			promptBuilder.WriteString("- ")
			promptBuilder.WriteString(summary)
			promptBuilder.WriteString("\n")
		}
		promptBuilder.WriteString("\n")
	} else {
		promptBuilder.WriteString("No specs exist yet.\n\n")
	}

	promptBuilder.WriteString("What areas of the epic are not covered by existing specs?")

	// Call Claude with haiku model
	result, err := client.Run(context.Background(), promptBuilder.String(), model)
	if err != nil {
		return "", err
	}
	return result.Output, nil
}
