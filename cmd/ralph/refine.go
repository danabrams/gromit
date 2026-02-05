package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/danabrams/ralph-runner/internal/backlog"
	"github.com/danabrams/ralph-runner/internal/bead"
	"github.com/spf13/cobra"
)

var refineCmd = &cobra.Command{
	Use:   "refine [idea-id]",
	Short: "Review backlog ideas and promote to tasks",
	Long: `Interactively review backlog ideas and promote them into real beads (tasks).

Examples:
  ralph refine              # Interactive refine session
  ralph refine idea-123     # Refine a specific idea`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRefine,
}

func init() {
	rootCmd.AddCommand(refineCmd)
}

func runRefine(cmd *cobra.Command, args []string) error {
	// Get .ralph directory from config or default
	ralphDir := ".ralph"
	if cfg, err := loadConfig(); err == nil {
		if cfg.Paths.RalphDir != "" {
			ralphDir = cfg.Paths.RalphDir
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("loading config: %w", err)
	}

	bf := backlog.NewFile(ralphDir)
	reader := bufio.NewReader(os.Stdin)

	// If a specific idea ID was given, refine just that one
	if len(args) == 1 {
		idea, err := bf.Get(args[0])
		if err != nil {
			return fmt.Errorf("loading idea: %w", err)
		}
		if idea == nil {
			return fmt.Errorf("idea not found: %s", args[0])
		}
		return refineIdea(bf, idea, reader)
	}

	// Otherwise, interactive session over all ideas
	ideas, err := bf.List()
	if err != nil {
		return fmt.Errorf("loading backlog: %w", err)
	}

	if len(ideas) == 0 {
		fmt.Println("Backlog is empty. Use 'ralph add <idea>' to capture ideas.")
		return nil
	}

	fmt.Printf("Starting refine session (%d ideas)\n\n", len(ideas))

	for _, idea := range ideas {
		if err := refineIdea(bf, idea, reader); err != nil {
			return err
		}

		fmt.Print("\nNext idea? [y/n]: ")
		answer, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("\nRefine session ended.")
			return nil
		}
		fmt.Println()
	}

	fmt.Println("\nAll ideas refined!")
	return nil
}

func refineIdea(bf *backlog.File, idea *backlog.Idea, reader *bufio.Reader) error {
	// Display the idea
	typeLabel := formatType(idea.Type)
	fmt.Printf("Refining: %s %s %s\n", idea.ID, typeLabel, idea.Text)
	if idea.Context != "" {
		fmt.Printf("  Context: %s\n", idea.Context)
	}
	fmt.Println()

	// 1. Refine into actionable task
	fmt.Print("What needs to happen to complete this?\n> ")
	taskTitle, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	taskTitle = strings.TrimSpace(taskTitle)
	if taskTitle == "" {
		taskTitle = idea.Text // Use original text if nothing entered
	}

	// 2. Priority
	fmt.Print("\nPriority? [0/1/2, default=1]: ")
	priorityStr, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	priorityStr = strings.TrimSpace(priorityStr)
	priority := 1
	if priorityStr != "" {
		p, err := strconv.Atoi(priorityStr)
		if err != nil || p < 0 || p > 2 {
			fmt.Println("Invalid priority, using default (1)")
		} else {
			priority = p
		}
	}

	// 3. Dependencies
	fmt.Print("\nAny dependencies? (bead IDs, comma-separated, or skip): ")
	depsStr, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	depsStr = strings.TrimSpace(depsStr)

	// 4. Expected outputs
	fmt.Print("\nExpected outputs? (files, comma-separated, or skip): ")
	outputsStr, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	outputsStr = strings.TrimSpace(outputsStr)

	// Parse expected outputs
	var expectedOutputs []string
	if outputsStr != "" && strings.ToLower(outputsStr) != "skip" {
		for _, o := range strings.Split(outputsStr, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				expectedOutputs = append(expectedOutputs, o)
			}
		}
	}

	// Build labels from idea type
	var labels []string
	if idea.Type != "" && idea.Type != "unknown" {
		labels = append(labels, idea.Type)
	}

	// Create bead via bd
	bdClient := bead.NewClient()
	created, err := bdClient.Create(taskTitle, priority, labels, expectedOutputs)
	if err != nil {
		return fmt.Errorf("creating bead: %w", err)
	}

	// Add dependencies as comments if provided
	if depsStr != "" && strings.ToLower(depsStr) != "skip" {
		comment := fmt.Sprintf("Dependencies: %s", depsStr)
		if err := bdClient.AddComment(created.ID, comment); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not add dependency comment: %v\n", err)
		}
	}

	// Remove from backlog
	if err := bf.Delete(idea.ID); err != nil {
		return fmt.Errorf("removing from backlog: %w", err)
	}

	fmt.Printf("\n✓ Created bead %s: %s\n", created.ID, taskTitle)
	fmt.Printf("✓ Removed %s from backlog\n", idea.ID)

	return nil
}
