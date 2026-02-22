package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/frontmatter"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/spf13/cobra"
)

var (
	decomposeReview         bool
	decomposeForce          bool
	decomposeNoChain        bool
	decomposeSkipValidation bool
	decomposeMaxRetries     int
)

const decomposeSessionCommand = "decompose"

var decomposeSessionLauncherFn = runWithSessionWorktreeWithConflictSettings
var decomposeSinglePlanInDirFn = decomposeSinglePlanInCurrentDir
var decomposeRunInDirFn = runInDir

var decomposeCmd = &cobra.Command{
	Use:   "decompose [plan-name]",
	Short: "Decompose a plan into bd beads",
	Long: `Decompose an implementation plan into bd beads automatically.

This command reads a plan file, invokes the configured decomposition provider non-interactively to extract
tasks and map them to beads following bead sizing rules, then creates the
beads via bd create.

Usage:
  gromit decompose                       # Interactive picker for undecomposed plans
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
  - Priority and acceptance criteria from the decomposition analysis`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDecompose,
}

func init() {
	decomposeCmd.Flags().BoolVar(&decomposeReview, "review", false, "Show proposed beads before creating")
	decomposeCmd.Flags().BoolVar(&decomposeForce, "force", false, "Re-decompose even if already done")
	decomposeCmd.Flags().BoolVar(&decomposeSkipValidation, "skip-validation", false, "Skip bead validation and retry loop")
	decomposeCmd.Flags().IntVar(&decomposeMaxRetries, "max-retries", -1, "Max validation retries (overrides gromit.yaml)")
	decomposeCmd.Flags().BoolVar(&decomposeNoChain, "no-chain", false, "Skip offering to run next command in pipeline")
	decomposeCmd.Flags().MarkHidden("no-chain")
	rootCmd.AddCommand(decomposeCmd)
}

// beadDef represents a bead definition for display purposes (review mode).
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
	// Load config
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Dispatch based on number of arguments
	if len(args) == 1 {
		// Single plan specified - decompose it directly
		planName := args[0]
		// Remove .md suffix if provided
		planName = strings.TrimSuffix(planName, ".md")
		return decomposeSinglePlan(planName, cfg)
	}

	// No arguments - show picker
	plansDir := resolvePlansDir(cfg)
	plans, err := filterUndecomposedPlans(plansDir, decomposeForce)
	if err != nil {
		return fmt.Errorf("scanning plans directory: %w", err)
	}

	if len(plans) == 0 {
		fmt.Println("No undecomposed plans found. Create one with 'gromit plan'.")
		return nil
	}

	// Display picker
	fmt.Println("Select a plan to decompose:")
	fmt.Println()
	for i, plan := range plans {
		fmt.Printf("  %d. %s - %s\n", i+1, plan.Name, plan.Title)
	}

	// Add "Decompose all" option if 2+ plans
	decomposeAllOption := -1
	if len(plans) >= 2 {
		decomposeAllOption = len(plans) + 1
		fmt.Printf("  %d. Decompose all\n", decomposeAllOption)
	}

	// Prompt for choice
	maxChoice := len(plans)
	if decomposeAllOption != -1 {
		maxChoice = decomposeAllOption
	}
	fmt.Printf("\nChoice [1-%d]: ", maxChoice)
	reader := bufio.NewReader(os.Stdin)
	choiceStr, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading choice: %w", err)
	}

	var choice int
	if _, err := fmt.Sscanf(strings.TrimSpace(choiceStr), "%d", &choice); err != nil || choice < 1 || choice > maxChoice {
		return fmt.Errorf("invalid choice")
	}

	// Handle "Decompose all" selection
	if decomposeAllOption != -1 && choice == decomposeAllOption {
		return decomposeAll(plans, cfg)
	}

	// Single plan selected
	selectedPlan := plans[choice-1]
	fmt.Printf("\nDecomposing: %s\n\n", selectedPlan.Name)
	return decomposeSinglePlan(selectedPlan.Name, cfg)
}

// decomposeSinglePlan decomposes a single plan file into bd beads.
// Delegates business logic to pipeline.Decompose() and handles CLI interactions
// (review confirmation, output formatting, chaining).
// Respects package-level flags: decomposeReview, decomposeForce, decomposeNoChain.
func decomposeSinglePlan(planName string, cfg *config.Config) error {
	if decomposeReview {
		return runDecomposeReviewInSession(planName, cfg)
	}

	return decomposeSinglePlanInDirFn(planName, cfg)
}

func runDecomposeReviewInSession(planName string, cfg *config.Config) error {
	gromitDir := resolveGromitDir(cfg)
	conflictSettings := sessionConflictSettingsFromConfig(cfg)
	_, err := decomposeSessionLauncherFn(gromitDir, decomposeSessionCommand, conflictSettings, func(sessionDir string) error {
		return decomposeRunInDirFn(sessionDir, func() error {
			return decomposeSinglePlanInDirFn(planName, cfg)
		})
	})
	return err
}

func decomposeSinglePlanInCurrentDir(planName string, cfg *config.Config) error {
	decomposeClient, err := buildDecomposeClient(cfg)
	if err != nil {
		return fmt.Errorf("creating decompose client: %w", err)
	}

	// Create Bead client
	beadClient, err := bead.NewClient()
	if err != nil {
		return fmt.Errorf("creating bead client: %w", err)
	}

	// Create pipeline
	deps := &pipeline.Deps{
		ClaudeClient: decomposeClient,
		BeadClient: &beadClientAdapter{Client: beadClient},
	}
	paths := &pipeline.Paths{
		GromitDir: resolveGromitDir(cfg),
		SpecsDir:  resolveSpecsDir(cfg),
		PlansDir:  resolvePlansDir(cfg),
	}

	p := pipeline.New(deps, paths)

	// Execute decompose workflow
	fmt.Printf("Decomposing plan '%s' into beads...\n", planName)
	ctx := context.Background()
	maxRetries := cfg.Validation.MaxValidationRetries
	if decomposeMaxRetries >= 0 {
		maxRetries = decomposeMaxRetries
	}
	input := pipeline.DecomposeInput{
		PlanName:             planName,
		Force:                decomposeForce,
		Review:               decomposeReview,
		SkipValidation:       decomposeSkipValidation,
		MaxValidationRetries: maxRetries,
	}

	result, err := p.Decompose(ctx, input)
	if err != nil {
		return err
	}

	// Review mode: show proposed beads and prompt for confirmation
	if decomposeReview {
		beadDefs := convertToBeadDefs(result.CreatedBeads)
		if !promptReviewBeads(beadDefs) {
			fmt.Println("\nDecomposition cancelled.")
			return nil
		}

		// User approved - run decompose again without review mode
		input.Review = false
		result, err = p.Decompose(ctx, input)
		if err != nil {
			return err
		}
	}

	// Display results
	fmt.Printf("\nExtracted %d bead(s) from plan\n\n", len(result.CreatedBeads))
	for _, bead := range result.CreatedBeads {
		fmt.Printf("  ✓ Created: %s (%s)\n", bead.ID, bead.Title)
	}

	fmt.Printf("\n✓ Created %d bead(s) from plan '%s'\n", len(result.CreatedBeads), planName)

	if result.ValidationStats != nil && result.ValidationStats.Attempts > 1 {
		stats := result.ValidationStats
		improved := ""
		if stats.Improved {
			improved = " (improved)"
		}
		fmt.Printf("  Validation: %d attempt(s), %d violation(s)%s\n", stats.Attempts, stats.ViolationCount, improved)
	}

	// Offer to chain to 'gromit run' if chaining is enabled
	if !decomposeNoChain && len(result.CreatedBeads) > 0 {
		chainAfterDecompose()
	}

	return nil
}

func buildDecomposeClient(cfg *config.Config) (pipeline.ClaudeClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	if cfg.HasProviders() {
		timeoutSecs := cfg.Claude.PipelineTimeout
		if timeoutSecs == 0 {
			timeoutSecs = config.DefaultPipelineTimeoutSeconds
		}
		timeout := time.Duration(timeoutSecs) * time.Second
		router, err := provider.BuildRouterFromConfig(cfg)
		if err != nil {
			return nil, err
		}
		return &providerRouterClientAdapter{
			Router:  router,
			Timeout: timeout,
			Phase:   decomposeSessionCommand,
		}, nil
	}

	claudeClient, err := claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout)
	if err != nil {
		return nil, err
	}

	timeoutSecs := cfg.Claude.PipelineTimeout
	if timeoutSecs == 0 {
		timeoutSecs = config.DefaultPipelineTimeoutSeconds
	}

	return &claudeClientAdapter{
		Client:  claudeClient,
		Timeout: time.Duration(timeoutSecs) * time.Second,
	}, nil
}

func runInDir(dir string, fn func() error) error {
	if fn == nil {
		return fmt.Errorf("callback is nil")
	}

	if dir == "" {
		return fn()
	}

	originalDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}

	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("changing directory to %q: %w", dir, err)
	}
	// Best-effort restore; callback return value remains the function result.
	defer os.Chdir(originalDir)

	return fn()
}

// convertToBeadDefs converts pipeline.CreatedBead to CLI beadDef for display.
// Note: Review mode doesn't include acceptance criteria or description in result,
// so those fields are empty in the conversion.
func convertToBeadDefs(beads []pipeline.CreatedBead) []beadDef {
	defs := make([]beadDef, len(beads))
	for i, bead := range beads {
		defs[i] = beadDef{
			Title:    bead.Title,
			Priority: fmt.Sprintf("P%d", bead.Priority),
		}
	}
	return defs
}

// promptReviewBeads displays proposed beads and asks for confirmation.
// Uses confirmPrompt for consistent yes/no handling with injected reader for testability.
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

	reader := bufio.NewReader(os.Stdin)
	return confirmPrompt(reader, "Create these beads?", false)
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

// decomposeAll processes all undecomposed plans sequentially.
// Shows progress for each plan, continues on errors, and summarizes results.
// Respects the --review flag for each individual plan.
func decomposeAll(plans []planInfo, cfg *config.Config) error {
	if len(plans) == 0 {
		return nil
	}

	fmt.Printf("\nDecomposing %d plan(s)...\n\n", len(plans))

	successCount := 0
	failedPlans := []string{}

	for i, plan := range plans {
		fmt.Printf("[%d/%d] Decomposing %s...\n", i+1, len(plans), plan.Name)

		err := decomposeSinglePlan(plan.Name, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			failedPlans = append(failedPlans, plan.Name)
		} else {
			successCount++
		}

		// Add spacing between plans (but not after last one)
		if i < len(plans)-1 {
			fmt.Println()
		}
	}

	// Summary
	fmt.Printf("\n✓ Decomposed %d/%d plan(s) successfully.\n", successCount, len(plans))
	if len(failedPlans) > 0 {
		fmt.Println("\nFailed plans:")
		for _, name := range failedPlans {
			fmt.Printf("  - %s\n", name)
		}
	}

	// Offer to chain to 'gromit run' if chaining is enabled and at least one plan succeeded
	if !decomposeNoChain && successCount > 0 {
		chainAfterDecompose()
	}

	return nil
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

	// Sort by name for consistent ordering.
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].Name < plans[j].Name
	})

	return plans, nil
}
