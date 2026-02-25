package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize gromit in the current project",
	Long: `Bootstrap gromit configuration and templates in the current directory.

Creates:
  gromit.yaml           - Configuration file
  CLAUDE.md             - Project documentation (with Gromit patterns)
  .gromit/
    templates/         - Prompt templates
      PROMPT_build.md
      PROMPT_validate.md
      PROMPT_analyze.md
      PROMPT_retro.md
      PROMPT_scope.md
      PROMPT_decompose.md
      PROMPT_review.md
      PROMPT_thorough_review.md
      PROMPT_learn.md
      PROMPT_acceptance_tests.md
      PROMPT_atdd_diagnostic.md
      PROMPT_atdd_build.md
      PROMPT_tdd_build.md
      PROMPT_refactor.md
      PROMPT_precheck.md
    specs/             - Specification files (empty)
    plans/             - Plan files (empty)`,
	RunE: runInit,
}

var forceInit bool

func init() {
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Overwrite existing files")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	fmt.Printf("Initializing gromit in %s\n", cwd)

	// Create .gromit directory structure
	dirs := []string{
		".gromit/templates",
		".gromit/specs",
		".gromit/plans",
	}

	for _, dir := range dirs {
		path := filepath.Join(cwd, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
		fmt.Printf("  Created %s/\n", dir)
	}

	// Write config file
	configPath := filepath.Join(cwd, "gromit.yaml")
	if err := writeFileIfNotExists(configPath, defaultConfig, forceInit); err != nil {
		return err
	}

	// Write CLAUDE.md
	claudeMDPath := filepath.Join(cwd, "CLAUDE.md")
	if err := writeFileIfNotExists(claudeMDPath, defaultClaudeMD, forceInit); err != nil {
		return err
	}

	// Write templates
	buildPath := filepath.Join(cwd, ".gromit/templates/PROMPT_build.md")
	if err := writeFileIfNotExists(buildPath, defaultBuildTemplate, forceInit); err != nil {
		return err
	}

	validatePath := filepath.Join(cwd, ".gromit/templates/PROMPT_validate.md")
	if err := writeFileIfNotExists(validatePath, defaultValidateTemplate, forceInit); err != nil {
		return err
	}

	retroPath := filepath.Join(cwd, ".gromit/templates/PROMPT_retro.md")
	if err := writeFileIfNotExists(retroPath, defaultRetroTemplate, forceInit); err != nil {
		return err
	}

	analyzePath := filepath.Join(cwd, ".gromit/templates/PROMPT_analyze.md")
	if err := writeFileIfNotExists(analyzePath, defaultAnalyzeTemplate, forceInit); err != nil {
		return err
	}

	scopePath := filepath.Join(cwd, ".gromit/templates/PROMPT_scope.md")
	if err := writeFileIfNotExists(scopePath, defaultScopeTemplate, forceInit); err != nil {
		return err
	}

	decomposePath := filepath.Join(cwd, ".gromit/templates/PROMPT_decompose.md")
	if err := writeFileIfNotExists(decomposePath, defaultDecomposeTemplate, forceInit); err != nil {
		return err
	}

	reviewPath := filepath.Join(cwd, ".gromit/templates/PROMPT_review.md")
	if err := writeFileIfNotExists(reviewPath, defaultReviewTemplate, forceInit); err != nil {
		return err
	}

	thoroughReviewPath := filepath.Join(cwd, ".gromit/templates/PROMPT_thorough_review.md")
	if err := writeFileIfNotExists(thoroughReviewPath, defaultThoroughReviewTemplate, forceInit); err != nil {
		return err
	}

	learnPath := filepath.Join(cwd, ".gromit/templates/PROMPT_learn.md")
	if err := writeFileIfNotExists(learnPath, defaultLearnTemplate, forceInit); err != nil {
		return err
	}

	acceptanceTestsPath := filepath.Join(cwd, ".gromit/templates/PROMPT_acceptance_tests.md")
	if err := writeFileIfNotExists(acceptanceTestsPath, defaultAcceptanceTestsTemplate, forceInit); err != nil {
		return err
	}

	atddDiagnosticPath := filepath.Join(cwd, ".gromit/templates/PROMPT_atdd_diagnostic.md")
	if err := writeFileIfNotExists(atddDiagnosticPath, defaultATDDDiagnosticTemplate, forceInit); err != nil {
		return err
	}

	atddBuildPath := filepath.Join(cwd, ".gromit/templates/PROMPT_atdd_build.md")
	if err := writeFileIfNotExists(atddBuildPath, defaultAtddBuildTemplate, forceInit); err != nil {
		return err
	}

	refactorPath := filepath.Join(cwd, ".gromit/templates/PROMPT_refactor.md")
	if err := writeFileIfNotExists(refactorPath, defaultRefactorTemplate, forceInit); err != nil {
		return err
	}

	tddBuildPath := filepath.Join(cwd, ".gromit/templates/PROMPT_tdd_build.md")
	if err := writeFileIfNotExists(tddBuildPath, defaultTDDBuildTemplate, forceInit); err != nil {
		return err
	}

	precheckPath := filepath.Join(cwd, ".gromit/templates/PROMPT_precheck.md")
	if err := writeFileIfNotExists(precheckPath, defaultPrecheckTemplate, forceInit); err != nil {
		return err
	}

	// Write RULES.md
	rulesPath := filepath.Join(cwd, ".gromit/RULES.md")
	if err := writeFileIfNotExists(rulesPath, defaultRules, forceInit); err != nil {
		return err
	}

	// Write LEARNINGS.md
	learningsPath := filepath.Join(cwd, ".gromit/LEARNINGS.md")
	if err := writeFileIfNotExists(learningsPath, defaultLearnings, forceInit); err != nil {
		return err
	}

	// Add to .gitignore if it exists
	gitignorePath := filepath.Join(cwd, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		appendToGitignore(gitignorePath)
	}

	fmt.Println("\nDone! Next steps:")
	fmt.Println("  1. Edit gromit.yaml to customize validation commands")
	fmt.Println("  2. Edit .gromit/RULES.md to add project-specific rules")
	fmt.Println("  3. Create specs in .gromit/specs/ and plans in .gromit/plans/")
	fmt.Println("  4. Create beads with: bd create \"Task title\" --priority 1")
	fmt.Println("  5. Run: gromit run --dry-run")
	fmt.Println("\nPeriodically run 'gromit retro' to analyze and consolidate learnings.")

	return nil
}

func writeFileIfNotExists(path, content string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("  Skipped %s (already exists, use --force to overwrite)\n", filepath.Base(path))
			return nil
		}
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	fmt.Printf("  Created %s\n", filepath.Base(path))
	return nil
}

func appendToGitignore(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	current := string(content)
	entries := []string{
		".gromit/logs/",
		".codex-home/",
	}

	var missing []string
	for _, entry := range entries {
		if !strings.Contains(current, entry) {
			missing = append(missing, entry)
		}
	}

	if len(missing) == 0 {
		return
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	f.WriteString("\n# Gromit runner\n")
	for _, entry := range missing {
		f.WriteString(entry + "\n")
	}
	fmt.Println("  Updated .gitignore")
}

