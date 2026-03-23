package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// RemediationFinding represents a single finding that needs remediation.
type RemediationFinding struct {
	File         string `json:"file"`
	Line         int    `json:"line,omitempty"`
	Category     string `json:"category"`
	Type         string `json:"type"`
	Description  string `json:"description"`
	SuggestedFix string `json:"suggested_fix,omitempty"`
}

// RemediationInput contains the input data for generating a remediation spec.
type RemediationInput struct {
	SpecID    string                          `json:"spec_id"`
	Summary   string                          `json:"summary"`
	Goals     []string                        `json:"goals"`
	NonGoals  []string                        `json:"non_goals"`
	DependsOn string                          `json:"depends_on,omitempty"`
	Findings  map[string][]RemediationFinding `json:"findings"`
}

// generateRemediationSpec produces a remediation spec markdown from RemediationInput.
// The spec follows the template format: spec_id, Depends on, Summary, Goals,
// Non-goals, Architecture (grouped by file), Acceptance Criteria, and Validation.
func generateRemediationSpec(input RemediationInput) string {
	var sb strings.Builder

	// spec_id section
	sb.WriteString("## spec_id\n\n")
	sb.WriteString(input.SpecID)
	sb.WriteString("\n\n")

	// Depends on section (if present)
	if input.DependsOn != "" {
		sb.WriteString("## Depends on\n\n")
		sb.WriteString(input.DependsOn)
		sb.WriteString("\n\n")
	}

	// Summary section
	sb.WriteString("## Summary\n\n")
	sb.WriteString(input.Summary)
	sb.WriteString("\n\n")

	// Goals section
	if len(input.Goals) > 0 {
		sb.WriteString("## Goals\n\n")
		for _, goal := range input.Goals {
			sb.WriteString(fmt.Sprintf("- %s\n", goal))
		}
		sb.WriteString("\n")
	}

	// Non-goals section
	if len(input.NonGoals) > 0 {
		sb.WriteString("## Non-goals\n\n")
		for _, nongoal := range input.NonGoals {
			sb.WriteString(fmt.Sprintf("- %s\n", nongoal))
		}
		sb.WriteString("\n")
	}

	// Architecture section (grouped by file)
	if len(input.Findings) > 0 {
		sb.WriteString("## Architecture\n\n")

		// Flatten findings from all categories into a single slice and group by file
		findingsByFile := make(map[string][]RemediationFinding)
		for _, categoryFindings := range input.Findings {
			for _, finding := range categoryFindings {
				findingsByFile[finding.File] = append(findingsByFile[finding.File], finding)
			}
		}

		// Sort files for consistent output
		var files []string
		for file := range findingsByFile {
			files = append(files, file)
		}
		sort.Strings(files)

		// Write findings grouped by file
		for _, file := range files {
			sb.WriteString(fmt.Sprintf("### %s\n\n", file))
			for _, finding := range findingsByFile[file] {
				if finding.Line > 0 {
					sb.WriteString(fmt.Sprintf("**Line %d:** `%s` (%s)\n\n", finding.Line, finding.Type, finding.Category))
				} else {
					sb.WriteString(fmt.Sprintf("**%s** (%s)\n\n", finding.Type, finding.Category))
				}
				sb.WriteString(finding.Description)
				sb.WriteString("\n\n")
			}
		}
	}

	// Acceptance Criteria section (one per finding)
	if len(input.Findings) > 0 {
		sb.WriteString("## Acceptance Criteria\n\n")
		criteriaIndex := 1

		// Iterate through categories in sorted order for deterministic output
		var categories []string
		for cat := range input.Findings {
			categories = append(categories, cat)
		}
		sort.Strings(categories)

		for _, category := range categories {
			for _, finding := range input.Findings[category] {
				// Use suggested_fix if available, otherwise use description
				criteria := finding.SuggestedFix
				if criteria == "" {
					criteria = finding.Description
				}
				sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", criteriaIndex, finding.File, criteria))
				criteriaIndex++
			}
		}
		sb.WriteString("\n")
	}

	// Validation section
	sb.WriteString("## Validation\n\n")
	sb.WriteString("- go test ./... -count=1\n")
	sb.WriteString("- go vet ./...\n")

	return sb.String()
}

// maybeGenerateRemediationSpec loads review.json from the run's evidence directory,
// filters non-blocking findings, and generates a remediation spec if any exist.
// Returns the written file path, or empty string if no findings / no spec generated.
func maybeGenerateRemediationSpec(runID, storeDir, specsDir string) (string, error) {
	// Default store directory
	if storeDir == "" {
		storeDir = ".gromit-next"
	}

	// Initialize run store
	store := runstore.NewStore(storeDir)

	// Load the run to get SpecID
	run, err := store.Get(runID)
	if err != nil {
		return "", fmt.Errorf("load run %q: %w", runID, err)
	}

	// Read review.json from evidence directory
	evidenceDir := store.RunEvidenceDir(runID)
	reviewPath := filepath.Join(evidenceDir, "review.json")

	data, err := os.ReadFile(reviewPath)
	if err != nil {
		return "", fmt.Errorf("read review.json: %w", err)
	}

	// Parse review.json as map[string]interface{} to handle dynamic categories
	var reviewJSON map[string]interface{}
	if err := json.Unmarshal(data, &reviewJSON); err != nil {
		return "", fmt.Errorf("parse review.json: %w", err)
	}

	// Extract and filter findings from array categories, grouped by category
	findingsByCategory := make(map[string][]RemediationFinding)

	for categoryKey, val := range reviewJSON {
		// Skip non-array values (metadata like diff_unavailable)
		findingsArray, ok := val.([]interface{})
		if !ok {
			continue
		}

		// Parse each finding in the array
		for _, item := range findingsArray {
			// Convert item to JSON for unmarshaling
			itemData, err := json.Marshal(item)
			if err != nil {
				continue
			}

			// Unmarshal as a raw map to extract fields
			var rawFinding map[string]interface{}
			if err := json.Unmarshal(itemData, &rawFinding); err != nil {
				continue
			}

			// Extract and validate required fields
			description, ok := rawFinding["description"].(string)
			if !ok || description == "" {
				continue
			}

			// Check severity - must be warning, info, or suggestion (non-blocking)
			severity, ok := rawFinding["severity"].(string)
			if !ok || (severity != "warning" && severity != "info" && severity != "suggestion") {
				continue
			}

			// Build finding from available fields
			file, _ := rawFinding["file"].(string)
			line, _ := rawFinding["line"].(float64)
			suggestedFix, _ := rawFinding["suggested_fix"].(string)
			facet, _ := rawFinding["facet"].(string)

			finding := RemediationFinding{
				File:         file,
				Line:         int(line),
				Category:     categoryKey,
				Type:         facet,
				Description:  description,
				SuggestedFix: suggestedFix,
			}

			findingsByCategory[categoryKey] = append(findingsByCategory[categoryKey], finding)
		}
	}

	// If no non-blocking findings, return empty string
	if len(findingsByCategory) == 0 {
		return "", nil
	}

	// Count total findings and sort within each category
	totalFindings := 0
	for cat := range findingsByCategory {
		totalFindings += len(findingsByCategory[cat])
		// Sort findings within category by file then line
		sort.Slice(findingsByCategory[cat], func(i, j int) bool {
			if findingsByCategory[cat][i].File != findingsByCategory[cat][j].File {
				return findingsByCategory[cat][i].File < findingsByCategory[cat][j].File
			}
			return findingsByCategory[cat][i].Line < findingsByCategory[cat][j].Line
		})
	}

	// Build remediation spec input
	remedialSpecID := run.SpecID + "-remediation"
	summary := fmt.Sprintf("Cleanup items from the %s review: %d findings across %d categories.", run.SpecID, totalFindings, len(findingsByCategory))

	input := RemediationInput{
		SpecID:    remedialSpecID,
		Summary:   summary,
		Goals:     []string{"Fix all non-blocking review findings"},
		NonGoals:  []string{"No behavior changes"},
		DependsOn: run.SpecID,
		Findings:  findingsByCategory,
	}

	// Generate remediation spec
	specContent := generateRemediationSpec(input)

	// Write to file: specsDir/<spec-id>-remediation.md
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		return "", fmt.Errorf("create specs directory: %w", err)
	}

	specPath := filepath.Join(specsDir, remedialSpecID+".md")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		return "", fmt.Errorf("write remediation spec: %w", err)
	}

	return specPath, nil
}
