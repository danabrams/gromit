package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <idea>",
	Short: "Add an idea to the backlog",
	Long: `Quickly capture ideas to the backlog without creating beads yet.

Examples:
  gromit add "Add cost tracking"
  gromit add "Fix the auth bug"
  gromit add "What if we had a web dashboard?"

The command will auto-categorize ideas when obvious (feature/bug/chore)
or ask for clarification when needed.`,
	Args: cobra.ExactArgs(1),
	RunE: runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

var addHandler = defaultAddHandler

func runAdd(cmd *cobra.Command, args []string) error {
	ideaText := args[0]

	cfg, err := loadConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("loading config: %w", err)
		}
		cfg = nil
	}
	gromitDir := resolveGromitDir(cfg)

	fmt.Print("\nAny additional context? (optional, press Enter to skip): ")
	var contextText string
	inputReader := bufio.NewReader(os.Stdin)
	if line, err := inputReader.ReadString('\n'); err == nil || (err != nil && strings.TrimSpace(line) != "") {
		contextText = strings.TrimSpace(line)
	}

	input := pipeline.AddInput{
		Text:    ideaText,
		Context: contextText,
	}

	result, err := addHandler(cmd.Context(), cfg, gromitDir, input)
	if errors.Is(err, pipeline.ErrUnknownIdeaType) {
		input.Type = promptIdeaType(inputReader)
		result, err = addHandler(cmd.Context(), cfg, gromitDir, input)
	}
	if err != nil {
		return err
	}

	fmt.Printf("\n✓ Added to backlog (%s): %s\n", result.Type, result.Idea.Text)
	if contextText != "" {
		fmt.Printf("  Context: %s\n", contextText)
	}
	fmt.Printf("  Saved to: %s\n", filepath.Join(gromitDir, "backlog.jsonl"))

	return nil
}

func promptIdeaType(reader io.Reader) string {
	fmt.Println("What type of idea is this?")
	fmt.Println("  1. Feature - New functionality")
	fmt.Println("  2. Bug - Something broken to fix")
	fmt.Println("  3. Chore - Refactor, update, or maintenance")
	fmt.Print("\nChoice [1-3]: ")

	lineReader, ok := reader.(*bufio.Reader)
	if !ok {
		lineReader = bufio.NewReader(reader)
	}
	choice, err := lineReader.ReadString('\n')
	if err != nil {
		fmt.Println("Invalid choice, defaulting to 'feature'")
		return "feature"
	}

	switch strings.TrimSpace(choice) {
	case "1":
		return "feature"
	case "2":
		return "bug"
	case "3":
		return "chore"
	default:
		fmt.Println("Invalid choice, defaulting to 'feature'")
		return "feature"
	}
}

func defaultAddHandler(ctx context.Context, cfg *config.Config, gromitDir string, input pipeline.AddInput) (*pipeline.AddResult, error) {
	p, err := newPipeline(cfg, gromitDir)
	if err != nil {
		return nil, fmt.Errorf("creating pipeline: %w", err)
	}
	return p.Add(ctx, input)
}
