package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/spf13/cobra"
)

var (
	planForce   bool
	planNoChain bool
)

const planSessionCommand = "plan"

var planSessionLauncherFn = runWithSessionWorktreeWithConflictSettings

type planExecutor interface {
	Plan(context.Context, pipeline.PlanInput) (*pipeline.PlanSession, error)
}

var createPlanPipelineFn = createPlanPipeline

var planCmd = &cobra.Command{
	Use:   "plan [spec-name]",
	Short: "Create an implementation plan from a spec",
	Long: `Start an interactive agent session to create an implementation plan from a spec.

Two input modes:
  gromit plan                    # Interactive picker for available specs
  gromit plan <spec-name>        # Plan a specific spec

The command launches the selected agent with:
- Full spec content as context
- Plans directory path for output
- Spec name for naming the plan file
- Open beads (current tasks) as project context
- References the gromit-plan skill for architecture and test planning

Plan refuses to run if a plan already exists for that spec unless --force is passed.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPlan,
}

func init() {
	planCmd.Flags().BoolVar(&planForce, "force", false, "Regenerate plan even if it already exists")
	planCmd.Flags().BoolVar(&planNoChain, "no-chain", false, "Skip offering to run next command in pipeline")
	planCmd.Flags().MarkHidden("no-chain")
	planCmd.Flags().String("agent", "", "Override the default agent for this plan session")
	planCmd.Flags().Bool("choose-agent", false, "Show interactive picker to choose agent")
	rootCmd.AddCommand(planCmd)
}

func runPlan(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("loading config: %w", err)
		}
		cfg = nil
	}

	gromitDir := resolveGromitDir(cfg)
	specsDir := resolveSpecsDir(cfg)
	plansDir := resolvePlansDir(cfg)

	if err := os.MkdirAll(plansDir, 0755); err != nil {
		return fmt.Errorf("creating plans directory: %w", err)
	}

	specName, err := determinePlanSpecName(cmd, args, specsDir, plansDir)
	if err != nil {
		return err
	}
	if specName == "" {
		return nil
	}

	specPath := filepath.Join(specsDir, specName+".md")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		return fmt.Errorf("spec not found: %s\nLooking for: %s", specName, specPath)
	}

	planPath := filepath.Join(plansDir, specName+".md")

	agentFlag, _ := cmd.Flags().GetString("agent")
	chooseAgent, _ := cmd.Flags().GetBool("choose-agent")

	executor, err := createPlanPipelineFn(cfg, gromitDir, specsDir, plansDir)
	if err != nil {
		return fmt.Errorf("creating plan pipeline: %w", err)
	}

	input := pipeline.PlanInput{
		SpecName:    specName,
		AgentName:   agentFlag,
		Force:       planForce,
		ChooseAgent: chooseAgent,
	}

	session, err := runPlanInSession(cmd.Context(), cfg, gromitDir, executor, input)
	if err != nil {
		return err
	}
	if session != nil {
		session.Cleanup()
	}

	planCreated := false
	if _, err := os.Stat(planPath); err == nil {
		fmt.Printf("\n✓ Plan created: %s\n", planPath)
		planCreated = true
	}

	if !planNoChain && planCreated {
		chainAfterPlan(specName, plansDir)
	}

	return nil
}

// filterUnplannedSpecs returns only specs that don't have a corresponding plan file
func filterUnplannedSpecs(specs []string, plansDir string) []string {
	unplanned := []string{}
	for _, specPath := range specs {
		specName := strings.TrimSuffix(filepath.Base(specPath), ".md")
		planPath := filepath.Join(plansDir, specName+".md")
		if _, err := os.Stat(planPath); os.IsNotExist(err) {
			unplanned = append(unplanned, specPath)
		}
	}
	return unplanned
}

func createPlanPipeline(cfg *config.Config, gromitDir, specsDir, plansDir string) (planExecutor, error) {
	deps, err := NewPipelineDeps(cfg, gromitDir)
	if err != nil {
		return nil, err
	}

	paths := &pipeline.Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
		PlansDir:  plansDir,
	}

	return pipeline.New(deps, paths), nil
}

func runPlanInSession(
	ctx context.Context,
	cfg *config.Config,
	gromitDir string,
	executor planExecutor,
	input pipeline.PlanInput,
) (*pipeline.PlanSession, error) {
	if executor == nil {
		return nil, fmt.Errorf("plan executor is nil")
	}
	var session *pipeline.PlanSession

	callback := func(sessionDir string) error {
		input.LaunchDir = sessionDir
		var err error
		session, err = executor.Plan(ctx, input)
		return err
	}
	fallback := func() error {
		input.LaunchDir = ""
		var err error
		session, err = executor.Plan(ctx, input)
		return err
	}

	if err := launchInSessionIfEnabled(cfg, gromitDir, planSessionCommand, planSessionLauncherFn, callback, fallback); err != nil {
		return nil, err
	}

	return session, nil
}

func determinePlanSpecName(cmd *cobra.Command, args []string, specsDir, plansDir string) (string, error) {
	if len(args) == 1 {
		return strings.TrimSuffix(args[0], ".md"), nil
	}

	specs, err := getSpecFiles(specsDir)
	if err != nil {
		return "", fmt.Errorf("scanning specs directory: %w", err)
	}

	specs = filterUnplannedSpecs(specs, plansDir)
	if len(specs) == 0 {
		fmt.Println("No unplanned specs found.")
		fmt.Println("\nAll specs already have plans. Use 'gromit plan <spec-name> --force' to re-plan an existing spec.")
		return "", nil
	}

	fmt.Println("Select a spec to plan:")
	fmt.Println()

	specNames := make([]string, 0, len(specs))
	for i, specPath := range specs {
		name := strings.TrimSuffix(filepath.Base(specPath), ".md")
		specNames = append(specNames, name)
		fmt.Printf("  %d. %s\n", i+1, name)
	}

	fmt.Printf("\nChoice [1-%d]: ", len(specs))
	reader := bufio.NewReader(os.Stdin)
	choiceStr, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("reading choice: %w", err)
	}

	var choice int
	if _, err := fmt.Sscanf(strings.TrimSpace(choiceStr), "%d", &choice); err != nil || choice < 1 || choice > len(specs) {
		return "", fmt.Errorf("invalid choice")
	}

	specName := specNames[choice-1]
	fmt.Printf("\nPlanning: %s\n\n", specName)
	return specName, nil
}

// chainAfterPlan offers to run 'gromit decompose' after plan is created.
// Default is yes [Y/n] because decompose is a natural continuation of the pipeline.
func chainAfterPlan(planName string, plansDir string) {
	if isPlanDecomposed(plansDir, planName) {
		return
	}
	reader := bufio.NewReader(os.Stdin)
	prompt := fmt.Sprintf("Run 'gromit decompose %s'?", planName)
	if confirmPrompt(reader, prompt, true) {
		if err := execGromit("decompose", planName); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to execute decompose: %v\n", err)
		}
	}
}
