package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/config"
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

	// Determine input mode
	input, err := determineRefineInput(cmd, args, gromitDir)
	if err != nil {
		return err
	}

	// Create pipeline
	p, err := createRefinePipelineFn(cfg, gromitDir, specsDir, plansDir)
	if err != nil {
		return fmt.Errorf("creating refine pipeline: %w", err)
	}

	// Execute refine
	result, err := p.Refine(cmd.Context(), *input)
	if err != nil {
		return err
	}

	// Output results and chain
	return handleRefineOutput(result, specsDir, plansDir)
}

func determineRefineInput(cmd *cobra.Command, args []string, gromitDir string) (*pipeline.RefineInput, error) {
	agentFlag, _ := cmd.Flags().GetString("agent")

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

		// Filter to unrefined
		var unrefined []*backlog.Idea
		for _, idea := range ideas {
			if idea.Status != "refined" {
				unrefined = append(unrefined, idea)
			}
		}

		// Empty backlog: blank session
		if len(unrefined) == 0 {
			fmt.Println("No unrefined backlog items. Starting a blank refinement session...")
			fmt.Println()
			return &pipeline.RefineInput{AgentName: agentFlag}, nil
		}

		// Show picker
		choice := showRefinePicker(unrefined, os.Stdin)
		if choice == len(unrefined) {
			// "Something new..." selected
			fmt.Println("\nStarting a blank refinement session...")
			fmt.Println()
			return &pipeline.RefineInput{AgentName: agentFlag}, nil
		}

		// Existing idea selected
		idea := unrefined[choice]
		fmt.Printf("\nRefining: %s\n\n", idea.Text)
		return &pipeline.RefineInput{IdeaID: idea.ID, AgentName: agentFlag}, nil
	}

	// One arg: backlog ID or ad-hoc text
	arg := args[0]
	if strings.HasPrefix(arg, "idea-") {
		return &pipeline.RefineInput{IdeaID: arg, AgentName: agentFlag}, nil
	}
	return &pipeline.RefineInput{IdeaText: arg, AgentName: agentFlag}, nil
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
	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		return nil, err
	}

	deps := &pipeline.Deps{
		AgentResolver: &agentResolverAdapter{cfg: cfg},
		BacklogClient: &backlogAdapter{file: bf},
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

// Adapters to bridge old interfaces to new

type agentResolverAdapter struct {
	cfg *config.Config
}

func (a *agentResolverAdapter) Resolve(phase string, flagOverride string, choosePicker bool) (pipeline.Agent, error) {
	return agent.Resolve(a.cfg, phase, flagOverride, choosePicker, os.Stdin, os.Stdout)
}

type backlogAdapter struct {
	file *backlog.File
}

func toPipelineIdea(idea *backlog.Idea) *pipeline.Idea {
	if idea == nil {
		return nil
	}

	return &pipeline.Idea{
		ID:       idea.ID,
		Text:     idea.Text,
		Type:     idea.Type,
		Context:  idea.Context,
		Status:   idea.Status,
		SpecName: idea.SpecName,
	}
}

func applyPipelineIdeaFields(dst *backlog.Idea, src *pipeline.Idea) {
	dst.Status = src.Status
	dst.SpecName = src.SpecName
}

func (b *backlogAdapter) List() ([]*pipeline.Idea, error) {
	ideas, err := b.file.List()
	if err != nil {
		return nil, err
	}
	result := make([]*pipeline.Idea, len(ideas))
	for i, idea := range ideas {
		result[i] = toPipelineIdea(idea)
	}
	return result, nil
}

func (b *backlogAdapter) Get(id string) (*pipeline.Idea, error) {
	idea, err := b.file.Get(id)
	if err != nil {
		return nil, err
	}
	return toPipelineIdea(idea), nil
}

func (b *backlogAdapter) Add(item *pipeline.Idea) error {
	// Note: CreatedAt and ID generation handled by caller
	return b.file.Add(&backlog.Idea{
		ID:       item.ID,
		Text:     item.Text,
		Type:     item.Type,
		Context:  item.Context,
		Status:   item.Status,
		SpecName: item.SpecName,
	})
}

func (b *backlogAdapter) Update(id string, fn func(*pipeline.Idea)) error {
	return b.file.Update(id, func(idea *backlog.Idea) {
		pipelineIdea := toPipelineIdea(idea)
		fn(pipelineIdea)
		applyPipelineIdeaFields(idea, pipelineIdea)
	})
}
