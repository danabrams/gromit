package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
)

// getEpicFiles returns a list of .md files in the epics directory.
// Creates the directory if it doesn't exist.
func getEpicFiles(epicsDir string) ([]string, error) {
	return listMarkdownFiles(epicsDir)
}

// buildExplorePrompt constructs the system prompt for the exploration session
func buildExplorePrompt(cfg *config.Config, gromitDir string, args []string) (string, error) {
	// Load project context
	templatesDir := resolveTemplatesDir(cfg)
	specsDir := resolveSpecsDir(cfg)
	claudeMDPath := resolveProjectClaudeMD(cfg)

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, claudeMDPath, gromitDir)
	if err != nil {
		return "", fmt.Errorf("creating prompt renderer: %w", err)
	}

	claudeMD, err := renderer.LoadClaudeMD()
	if err != nil {
		return "", fmt.Errorf("loading CLAUDE.md: %w", err)
	}

	rules, err := renderer.LoadRules()
	if err != nil {
		return "", fmt.Errorf("loading RULES.md: %w", err)
	}

	// Load learnings
	lf := renderer.GetLearningsFile()
	var confirmedLearnings, recentLearnings string
	if lf != nil {
		confirmed := lf.GetConfirmed()
		recent := lf.GetRecent(24)

		if len(confirmed) > 0 {
			var sb strings.Builder
			for _, l := range confirmed {
				sb.WriteString(fmt.Sprintf("- **[%s]** %s\n", l.Category, l.Content))
			}
			confirmedLearnings = sb.String()
		} else {
			confirmedLearnings = "*None*"
		}

		if len(recent) > 0 {
			var sb strings.Builder
			for _, l := range recent {
				sb.WriteString(fmt.Sprintf("- **[%s]** %s\n", l.Category, l.Content))
			}
			recentLearnings = sb.String()
		} else {
			recentLearnings = "*None*"
		}
	} else {
		confirmedLearnings = "*None*"
		recentLearnings = "*None*"
	}

	// Get working directory
	workDir, _ := os.Getwd()

	// Build the system prompt
	var sb strings.Builder

	// Optional: pre-seeded topic
	if len(args) > 0 {
		sb.WriteString(fmt.Sprintf(`Topic for exploration:

%s

`, args[0]))
	}

	// Context section
	sb.WriteString(fmt.Sprintf(`## Context

Working directory: %s
Epics directory: %s/epics
Specs directory: %s

`, workDir, gromitDir, specsDir))

	// Project context
	if claudeMD != "" {
		sb.WriteString(fmt.Sprintf(`### Project Context

%s

`, claudeMD))
	}

	if rules != "" {
		sb.WriteString(fmt.Sprintf(`### Rules (Non-Negotiable)

%s

`, rules))
	}

	// Learnings
	sb.WriteString(fmt.Sprintf(`### Learnings

#### Confirmed Patterns

%s

#### Recent Learnings

%s

`, confirmedLearnings, recentLearnings))

	return sb.String(), nil
}
