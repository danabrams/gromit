package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/frontmatter"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/skills"
	"github.com/spf13/cobra"
)

var (
	decomposeReview  bool
	decomposeForce   bool
	decomposeNoChain bool
)

var decomposeCmd = &cobra.Command{
	Use:   "decompose <plan-name>",
	Short: "Decompose a plan into bd beads",
	Long: `Decompose an implementation plan into bd beads automatically.

This command reads a plan file, invokes Claude non-interactively to extract
tasks and map them to beads following bead sizing rules, then creates the
beads via bd create.

Usage:
  gromit decompose <plan-name>           # Decompose plan fully automatically
  gromit decompose <plan-name> --review  # Show proposed beads before creating
  gromit decompose <plan-name> --force   # Re-decompose even if already done

The plan must exist in the configured plans directory (.gromit/plans/ by default).
After successful decomposition, the plan frontmatter is updated with:
  - decomposed: true
  - decomposed_at: <timestamp>

Each bead is created with:
  - spec:<plan-name> label for traceability
  - Dependencies mapped from the plan's task dependencies
  - Priority and acceptance criteria from Claude's analysis`,
	Args: cobra.ExactArgs(1),
	RunE: runDecompose,
}

func init() {
	decomposeCmd.Flags().BoolVar(&decomposeReview, "review", false, "Show proposed beads before creating")
	decomposeCmd.Flags().BoolVar(&decomposeForce, "force", false, "Re-decompose even if already done")
	decomposeCmd.Flags().BoolVar(&decomposeNoChain, "no-chain", false, "Skip offering to run next command in pipeline")
	decomposeCmd.Flags().MarkHidden("no-chain")
	rootCmd.AddCommand(decomposeCmd)
}

// beadDef represents a bead definition from Claude's JSON output
type beadDef struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Priority           string   `json:"priority"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	DependsOnIndex     []int    `json:"depends_on_index"`
}

// planInfo holds metadata for a plan file used in the picker
type planInfo struct {
	Name  string
	Title string
	Path  string
}

func runDecompose(cmd *cobra.Command, args []string) error {
	planName := args[0]
	// Remove .md suffix if provided
	planName = strings.TrimSuffix(planName, ".md")

	// Load config
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	plansDir := resolvePlansDir(cfg)

	// Check if plan file exists
	planPath := filepath.Join(plansDir, planName+".md")
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		return fmt.Errorf("plan not found: %s\nLooking for: %s", planName, planPath)
	}

	// Read plan file
	planFrontmatter, planBody, err := frontmatter.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("reading plan file: %w", err)
	}

	// Check if already decomposed
	if decomposed, ok := planFrontmatter["decomposed"].(bool); ok && decomposed && !decomposeForce {
		return fmt.Errorf("plan already decomposed: %s\nUse --force to re-decompose", planName)
	}

	// Build prompt with embedded skill content
	prompt := buildDecomposePrompt(planName, planBody, skills.DecomposeSkill)

	// Create Claude client
	claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
	if err != nil {
		return fmt.Errorf("creating Claude client: %w", err)
	}

	// Run Claude non-interactively with sonnet
	fmt.Printf("Decomposing plan '%s' into beads...\n", planName)
	ctx := context.Background()
	result, err := claudeClient.Run(ctx, prompt, config.ModelSonnet)
	if err != nil {
		return fmt.Errorf("invoking Claude: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("Claude invocation failed (exit code %d)\nOutput:\n%s", result.ExitCode, result.Output)
	}

	// Parse JSON array from result
	var beadDefs []beadDef
	if err := jsonutil.ExtractJSON(result.Output, &beadDefs); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse JSON output from Claude:\n%s\n\n", result.Output)
		return fmt.Errorf("parsing bead definitions: %w", err)
	}

	if len(beadDefs) == 0 {
		return fmt.Errorf("no beads extracted from plan")
	}

	fmt.Printf("\nExtracted %d bead(s) from plan\n\n", len(beadDefs))

	// Review mode: show proposed beads and prompt for confirmation
	if decomposeReview {
		if !promptReviewBeads(beadDefs) {
			fmt.Println("\nDecomposition cancelled.")
			return nil
		}
	}

	// Create beads
	beadClient, err := bead.NewClient()
	if err != nil {
		return fmt.Errorf("creating bead client: %w", err)
	}

	createdBeads := []string{}
	for i, def := range beadDefs {
		// Map priority string to int (P0 -> 0, P1 -> 1, P2 -> 2)
		priority := parsePriority(def.Priority)

		// Build labels: always include spec:<name>
		labels := []string{fmt.Sprintf("spec:%s", planName)}

		// Resolve dependencies from depends_on_index
		var dependencies []string
		for _, depIdx := range def.DependsOnIndex {
			if depIdx < 0 || depIdx >= len(createdBeads) {
				return fmt.Errorf("invalid dependency index %d in bead %d (%s)", depIdx, i, def.Title)
			}
			dependencies = append(dependencies, createdBeads[depIdx])
		}

		// Create bead
		createdBead, err := beadClient.CreateWithDepsAndDescription(
			def.Title,
			priority,
			labels,
			def.AcceptanceCriteria,
			dependencies,
			def.Description,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nFailed to create bead %d: %v\n", i+1, err)
			fmt.Fprintf(os.Stderr, "Successfully created %d bead(s) before failure:\n", len(createdBeads))
			for j, id := range createdBeads {
				fmt.Fprintf(os.Stderr, "  %d. %s\n", j+1, id)
			}
			return fmt.Errorf("bead creation failed")
		}

		createdBeads = append(createdBeads, createdBead.ID)
		fmt.Printf("  ✓ Created: %s (%s)\n", createdBead.ID, def.Title)
	}

	// Update plan frontmatter
	updates := map[string]interface{}{
		"decomposed":    true,
		"decomposed_at": time.Now().Format(time.RFC3339),
	}
	if err := frontmatter.UpdateFile(planPath, updates); err != nil {
		fmt.Fprintf(os.Stderr, "\nWarning: failed to update plan frontmatter: %v\n", err)
		fmt.Fprintf(os.Stderr, "Beads were created successfully, but plan file not marked as decomposed.\n")
	}

	fmt.Printf("\n✓ Created %d bead(s) from plan '%s'\n", len(createdBeads), planName)

	// Offer to chain to 'gromit run' if chaining is enabled
	if !decomposeNoChain && len(createdBeads) > 0 {
		chainAfterDecompose()
	}

	return nil
}

// buildDecomposePrompt constructs the prompt for Claude
func buildDecomposePrompt(planName, planBody, skillContent string) string {
	return fmt.Sprintf(`# Decompose Plan: %s

You are decomposing an implementation plan into bd beads following the gromit-decompose skill.

## Plan Content

%s

## Skill Instructions

%s

## Output

Output ONLY a JSON array of bead definitions. No markdown, no explanations, no wrapper.
Each bead must include: title, description, priority, acceptance_criteria, depends_on_index.

The spec label will be added automatically: spec:%s
`, planName, planBody, skillContent, planName)
}

// parsePriority converts priority string (P0, P1, P2) to int (0, 1, 2)
func parsePriority(p string) int {
	switch strings.ToUpper(p) {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	default:
		return 1 // Default to P1
	}
}

// promptReviewBeads displays proposed beads and asks for confirmation
func promptReviewBeads(beadDefs []beadDef) bool {
	fmt.Println("Proposed beads:")
	fmt.Println()

	for i, def := range beadDefs {
		fmt.Printf("Bead %d: %s\n", i+1, def.Title)
		fmt.Printf("  Priority: %s\n", def.Priority)
		fmt.Printf("  Description: %s\n", def.Description)
		fmt.Printf("  Acceptance Criteria:\n")
		for _, ac := range def.AcceptanceCriteria {
			fmt.Printf("    - %s\n", ac)
		}
		if len(def.DependsOnIndex) > 0 {
			fmt.Printf("  Depends on: %v\n", def.DependsOnIndex)
		}
		fmt.Println()
	}

	fmt.Print("Create these beads? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// chainAfterDecompose offers to run 'gromit run' after beads are created.
// Default is no [y/N] because the user may want to review beads first.
func chainAfterDecompose() {
	reader := bufio.NewReader(os.Stdin)
	if confirmPrompt(reader, "Run 'gromit run'?", false) {
		if err := execGromit("run"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to execute run: %v\n", err)
		}
	}
}

// filterUndecomposedPlans scans the plans directory for plan files and returns
// those that haven't been decomposed yet (or all plans if force is true).
// Returns a sorted slice of planInfo structs with name, title, and path.
func filterUndecomposedPlans(plansDir string, force bool) ([]planInfo, error) {
	// Read directory
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []planInfo{}, nil
		}
		return nil, fmt.Errorf("reading plans directory: %w", err)
	}

	var plans []planInfo
	for _, entry := range entries {
		// Skip directories and non-.md files
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		planPath := filepath.Join(plansDir, entry.Name())
		planName := strings.TrimSuffix(entry.Name(), ".md")

		// Read frontmatter to check decomposed status
		planFrontmatter, _, err := frontmatter.ReadFile(planPath)
		if err != nil {
			// Skip files that can't be read
			continue
		}

		// Filter by decomposed status (unless force is true)
		if !force {
			if decomposed, ok := planFrontmatter["decomposed"].(bool); ok && decomposed {
				continue
			}
		}

		// Extract title from plan file
		title := extractSpecTitle(planPath)
		if title == "" {
			title = planName // Fallback to name if no title found
		}

		plans = append(plans, planInfo{
			Name:  planName,
			Title: title,
			Path:  planPath,
		})
	}

	// Sort by name for consistent ordering
	// Using a simple bubble sort since we expect few plans
	for i := 0; i < len(plans); i++ {
		for j := i + 1; j < len(plans); j++ {
			if plans[i].Name > plans[j].Name {
				plans[i], plans[j] = plans[j], plans[i]
			}
		}
	}

	return plans, nil
}
