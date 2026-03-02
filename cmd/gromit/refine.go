package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/frontmatter"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/spf13/cobra"
)

var refineCmd = &cobra.Command{
	Use:   "refine [backlog-id or idea text]",
	Short: "Refine ideas into structured specs",
	Long: `Start an interactive agent session to refine ideas into structured specifications.

Three input modes:
  gromit refine                    # Interactive picker for unrefined backlog items
  gromit refine <backlog-id>       # Refine a specific backlog item
  gromit refine "some idea text"   # Refine an ad-hoc idea (not in backlog)

The command launches the selected agent with:
- The idea text as context
- Specs directory path for output
- References the gromit-refine skill for conversational refinement

After the session exits, scans for new spec files and marks backlog items as refined.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRefine,
}

var createRefinePipelineFn = createRefinePipeline
var refineSessionLauncherFn = runWithSessionWorktreeWithConflictSettings
var refineRunInDirFn = runInDir

const refineSessionCommand = "refine"

func init() {
	rootCmd.AddCommand(refineCmd)
	refineCmd.Flags().String("agent", "", "Override the default agent for this refine session")
	refineCmd.Flags().Bool("choose-agent", false, "Show interactive picker to choose agent")
}

func runRefine(cmd *cobra.Command, args []string) error {
	// Get config and directories
	cfg, err := loadConfig()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("loading config: %w", err)
	}
	gromitDir := resolveGromitDir(cfg)
	specsDir := resolveSpecsDir(cfg)
	plansDir := resolvePlansDir(cfg)
	if gromitDir, err = absPath(gromitDir, "gromit directory"); err != nil {
		return err
	}
	if specsDir, err = absPath(specsDir, "specs directory"); err != nil {
		return err
	}
	if plansDir, err = absPath(plansDir, "plans directory"); err != nil {
		return err
	}

	// Determine input mode
	input, err := determineRefineInput(cmd, args, gromitDir, specsDir)
	if err != nil {
		return err
	}

	// Create pipeline
	p, err := createRefinePipelineFn(cfg, gromitDir, specsDir, plansDir)
	if err != nil {
		return fmt.Errorf("creating refine pipeline: %w", err)
	}

	// Execute refine
	result, err := runRefineInSession(cmd.Context(), cfg, gromitDir, p, *input)
	if err != nil {
		return err
	}

	// Output results and chain
	return handleRefineOutput(result, specsDir, plansDir)
}

func runRefineInSession(
	ctx context.Context,
	cfg *config.Config,
	gromitDir string,
	p *pipeline.Pipeline,
	input pipeline.RefineInput,
) (*pipeline.RefineResult, error) {
	if p == nil {
		return nil, fmt.Errorf("pipeline is nil")
	}

	var result *pipeline.RefineResult
	fallback := func() error {
		var err error
		result, err = p.Refine(ctx, input)
		return err
	}

	if err := launchInSessionIfEnabledWithContext(ctx, cfg, gromitDir, refineSessionCommand, refineSessionLauncherFn, func(sessionDir string) error {
		return refineRunInDirFn(sessionDir, func() error {
			var runErr error
			result, runErr = p.Refine(ctx, input)
			return runErr
		})
	}, fallback); err != nil {
		return nil, err
	}

	return result, nil
}

func determineRefineInput(cmd *cobra.Command, args []string, gromitDir, specsDir string) (*pipeline.RefineInput, error) {
	agentFlag, _ := cmd.Flags().GetString("agent")
	chooseAgent := parseRefineChooseAgentFlag(cmd)

	// No args: interactive picker or blank session
	if len(args) == 0 {
		bf, err := backlog.NewFile(gromitDir)
		if err != nil {
			return nil, fmt.Errorf("creating backlog file: %w", err)
		}

		ideas, err := bf.List()
		if err != nil {
			return nil, fmt.Errorf("loading backlog: %w", err)
		}

		refinedFromSpecs, err := loadRefinedIdeaIDsFromSpecs(specsDir)
		if err != nil {
			return nil, fmt.Errorf("loading refined source_ideas from specs: %w", err)
		}
		unrefined := filterRefineCandidates(ideas, refinedFromSpecs)

		// Empty backlog: blank session
		if len(unrefined) == 0 {
			fmt.Println("No unrefined backlog items. Starting a blank refinement session...")
			fmt.Println()
			return &pipeline.RefineInput{AgentName: agentFlag, ChooseAgent: chooseAgent}, nil
		}

		// Show picker
		choice := showRefinePicker(unrefined, os.Stdin)
		if choice == len(unrefined) {
			// "Something new..." selected
			fmt.Println("\nStarting a blank refinement session...")
			fmt.Println()
			return &pipeline.RefineInput{AgentName: agentFlag, ChooseAgent: chooseAgent}, nil
		}

		// Existing idea selected
		idea := unrefined[choice]
		fmt.Printf("\nRefining: %s\n\n", idea.Text)
		return &pipeline.RefineInput{IdeaID: idea.ID, AgentName: agentFlag, ChooseAgent: chooseAgent}, nil
	}

	// One arg: backlog ID or ad-hoc text
	arg := args[0]
	if strings.HasPrefix(arg, "idea-") {
		return &pipeline.RefineInput{IdeaID: arg, AgentName: agentFlag, ChooseAgent: chooseAgent}, nil
	}
	return &pipeline.RefineInput{IdeaText: arg, AgentName: agentFlag, ChooseAgent: chooseAgent}, nil
}

func filterRefineCandidates(ideas []*backlog.Idea, refinedBySpec map[string]struct{}) []*backlog.Idea {
	unrefined := make([]*backlog.Idea, 0, len(ideas))
	for _, idea := range ideas {
		if idea == nil {
			continue
		}

		status := strings.ToLower(strings.TrimSpace(idea.Status))
		if status == "refined" || status == "closed" || status == "rejected" {
			continue
		}
		if strings.TrimSpace(idea.SpecName) != "" {
			continue
		}
		if _, ok := refinedBySpec[strings.TrimSpace(idea.ID)]; ok {
			continue
		}

		// Any other status (or empty status) is eligible.
		unrefined = append(unrefined, idea)
	}
	return unrefined
}

func loadRefinedIdeaIDsFromSpecs(specsDir string) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	specFiles, err := getSpecFiles(specsDir)
	if err != nil {
		return nil, err
	}

	for _, specPath := range specFiles {
		fm, _, err := frontmatter.ReadFile(specPath)
		if err != nil {
			continue
		}
		raw, ok := fm["source_ideas"]
		if !ok {
			continue
		}

		addRefinedIdeaID(result, raw)
	}

	return result, nil
}

func addRefinedIdeaID(ids map[string]struct{}, raw interface{}) {
	if ids == nil {
		return
	}

	switch v := raw.(type) {
	case string:
		recordRefinedIdeaID(ids, v)
	case int:
		recordRefinedIdeaID(ids, strconv.Itoa(v))
	case int8:
		recordRefinedIdeaID(ids, strconv.FormatInt(int64(v), 10))
	case int16:
		recordRefinedIdeaID(ids, strconv.FormatInt(int64(v), 10))
	case int32:
		recordRefinedIdeaID(ids, strconv.FormatInt(int64(v), 10))
	case int64:
		recordRefinedIdeaID(ids, strconv.FormatInt(v, 10))
	case uint:
		recordRefinedIdeaID(ids, strconv.FormatUint(uint64(v), 10))
	case uint8:
		recordRefinedIdeaID(ids, strconv.FormatUint(uint64(v), 10))
	case uint16:
		recordRefinedIdeaID(ids, strconv.FormatUint(uint64(v), 10))
	case uint32:
		recordRefinedIdeaID(ids, strconv.FormatUint(uint64(v), 10))
	case uint64:
		recordRefinedIdeaID(ids, strconv.FormatUint(v, 10))
	case float64:
		if !math.IsNaN(v) && !math.IsInf(v, 0) && math.Trunc(v) == v {
			recordRefinedIdeaID(ids, strconv.FormatInt(int64(v), 10))
		}
	case []interface{}:
		for _, item := range v {
			addRefinedIdeaID(ids, item)
		}
	case []string:
		for _, id := range v {
			recordRefinedIdeaID(ids, id)
		}
	}
}

func recordRefinedIdeaID(ids map[string]struct{}, id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}

	ids[id] = struct{}{}
	if digitsOnly(id) {
		ids["idea-"+id] = struct{}{}
		return
	}

	const ideaPrefix = "idea-"
	if strings.HasPrefix(id, ideaPrefix) {
		suffix := strings.TrimPrefix(id, ideaPrefix)
		if digitsOnly(suffix) {
			ids[suffix] = struct{}{}
		}
	}
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseRefineChooseAgentFlag(cmd *cobra.Command) bool {
	chooseAgent, _ := cmd.Flags().GetBool("choose-agent")
	return chooseAgent
}

func showRefinePicker(unrefined []*backlog.Idea, reader io.Reader) int {
	newIdeaIndex := len(unrefined)
	maxChoice := newIdeaIndex + 1

	fmt.Println("Select an idea to refine:")
	fmt.Println()
	for i, idea := range unrefined {
		typeLabel := formatTypeLabel(idea.Type)
		fmt.Printf("  %d. %s %s\n", i+1, typeLabel, idea.Text)
		if idea.Context != "" {
			fmt.Printf("     Context: %s\n", idea.Context)
		}
	}
	fmt.Printf("  %d. [new]     Something new...\n", maxChoice)

	fmt.Printf("\nChoice [1-%d]: ", maxChoice)
	lineReader := bufio.NewReader(reader)
	choiceStr, err := lineReader.ReadString('\n')
	if err != nil {
		return newIdeaIndex
	}

	var choice int
	n, _ := fmt.Sscanf(strings.TrimSpace(choiceStr), "%d", &choice)
	if n != 1 || choice < 1 || choice > maxChoice {
		return newIdeaIndex
	}
	return choice - 1
}

func createRefinePipeline(cfg *config.Config, gromitDir, specsDir, plansDir string) (*pipeline.Pipeline, error) {
	// Construct pipeline dependencies using dependency injection
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

func handleRefineOutput(result *pipeline.RefineResult, specsDir, plansDir string) error {
	if len(result.CreatedSpecs) > 0 {
		fmt.Printf("\nSpec files created:\n")
		for _, spec := range result.CreatedSpecs {
			fmt.Printf("  - %s\n", spec)
		}

		// Extract spec names for chaining
		var specNames []string
		for _, specPath := range result.CreatedSpecs {
			specName := strings.TrimSuffix(filepath.Base(specPath), ".md")
			specNames = append(specNames, specName)
		}

		// Chain to next stages
		reader := bufio.NewReader(os.Stdin)
		chainAfterRefine(specNames, plansDir, func(prompt string, defaultYes bool) bool {
			return confirmPrompt(reader, prompt, defaultYes)
		}, execGromit)
	}

	return nil
}
