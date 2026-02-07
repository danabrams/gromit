package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/frontmatter"
)

// confirmPrompt prints a yes/no prompt and reads the user's response.
// Returns true for yes, false for no.
// If the user provides empty input, returns defaultYes.
// Accepts y/yes/n/no case-insensitively.
func confirmPrompt(reader *bufio.Reader, prompt string, defaultYes bool) bool {
	// Build prompt suffix based on default
	suffix := " [y/N]: "
	if defaultYes {
		suffix = " [Y/n]: "
	}

	// Print prompt
	fmt.Print(prompt + suffix)

	// Read line
	line, err := reader.ReadString('\n')
	if err != nil {
		// On error, return default
		return defaultYes
	}

	// Trim whitespace and convert to lowercase
	response := strings.ToLower(strings.TrimSpace(line))

	// Empty input uses default
	if response == "" {
		return defaultYes
	}

	// Check for yes/no
	if response == "y" || response == "yes" {
		return true
	}
	if response == "n" || response == "no" {
		return false
	}

	// Invalid input: return default
	return defaultYes
}

// execGromit executes the current gromit binary as a subprocess with the given arguments.
// It connects the subprocess's stdin/stdout/stderr to the parent process's stdio.
// Returns nil if the subprocess exits successfully or exits with an error code
// (subprocess printed its own errors). Returns non-nil only if the subprocess
// cannot be launched.
func execGromit(args ...string) error {
	// Find the current binary path
	binary, err := os.Executable()
	if err != nil {
		// Fallback to os.Args[0] if os.Executable() fails
		binary = os.Args[0]
	}

	// Create command
	cmd := exec.Command(binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the subprocess
	if err := cmd.Run(); err != nil {
		// Treat exec.ExitError as nil - subprocess printed its own errors
		if _, ok := err.(*exec.ExitError); ok {
			return nil
		}
		return err
	}

	return nil
}

// isPlanDecomposed reads the plan frontmatter from <plansDir>/<planName>.md
// and returns true only if the decomposed field is true.
// Returns false if the file doesn't exist, has no frontmatter, or decomposed is not true.
func isPlanDecomposed(plansDir, planName string) bool {
	planPath := filepath.Join(plansDir, planName+".md")
	fm, _, err := frontmatter.ReadFile(planPath)
	if err != nil {
		return false
	}

	decomposed, ok := fm["decomposed"].(bool)
	return ok && decomposed
}

// chainAfterRefine orchestrates the three-phase multi-spec pipeline flow after refine.
// Phase 1: Offer to plan each spec (with --no-chain), track which plans were created.
// Phase 2: Offer to decompose each successfully planned spec (with --no-chain), count successes.
// Phase 3: If any specs were decomposed, offer to run gromit run (default: no).
// Declining at any point skips remaining items in that phase and moves to the next phase.
func chainAfterRefine(specNames []string, plansDir string) {
	if len(specNames) == 0 {
		return
	}

	reader := bufio.NewReader(os.Stdin)

	// Phase 1: Planning (interactive, sequential)
	plannedNames := []string{}
	for _, specName := range specNames {
		prompt := fmt.Sprintf("Run 'gromit plan %s'?", specName)
		if !confirmPrompt(reader, prompt, true) {
			// User declined, skip remaining plans
			break
		}

		// Run plan with --no-chain
		if err := execGromit("plan", specName, "--no-chain"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to execute plan: %v\n", err)
			// Don't break - continue offering remaining plans
			continue
		}

		// Check if plan file was created
		planPath := filepath.Join(plansDir, specName+".md")
		if _, err := os.Stat(planPath); err == nil {
			plannedNames = append(plannedNames, specName)
		}
	}

	// Phase 2: Decomposition (interactive, sequential)
	decomposedCount := 0
	for _, planName := range plannedNames {
		// Skip if plan is already decomposed
		if isPlanDecomposed(plansDir, planName) {
			decomposedCount++
			continue
		}

		prompt := fmt.Sprintf("Run 'gromit decompose %s'?", planName)
		if !confirmPrompt(reader, prompt, true) {
			// User declined, skip remaining decomposes
			break
		}

		// Run decompose with --no-chain
		if err := execGromit("decompose", planName, "--no-chain"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to execute decompose: %v\n", err)
			// Don't break - continue offering remaining decomposes
			continue
		}

		// Verify decompose actually succeeded by checking the frontmatter
		if isPlanDecomposed(plansDir, planName) {
			decomposedCount++
		}
	}

	// Phase 3: Run (only if at least one decompose succeeded)
	if decomposedCount > 0 {
		if confirmPrompt(reader, "Run 'gromit run'?", false) {
			if err := execGromit("run"); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to execute run: %v\n", err)
			}
		}
	}
}
