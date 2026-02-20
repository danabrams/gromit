package prompt

import (
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/learnings"
)

const (
	successLearningCategoryConventions = "conventions"
	successLearningCategoryGotchas     = "gotchas"
	successLearningCategoryPatterns    = "patterns"
)

func formatTaskIdentity(b *bead.Bead, parent *bead.Bead, iteration int, model string) string {
	if b == nil {
		return ""
	}

	parentID := ""
	parentTitle := ""
	if parent != nil {
		parentID = parent.ID
		parentTitle = parent.Title
	}

	return fmt.Sprintf(
		"bead_id=%s\nbead_title=%s\nbead_description=%s\nparent_id=%s\nparent_title=%s\niteration=%d\nmodel=%s",
		b.ID,
		b.Title,
		b.Description,
		parentID,
		parentTitle,
		iteration,
		model,
	)
}

func formatBeadIdentity(id string, title string, description string, model string, iteration int) string {
	return fmt.Sprintf(
		"bead_id=%s\nbead_title=%s\nbead_description=%s\niteration=%d\nmodel=%s",
		id,
		title,
		description,
		iteration,
		model,
	)
}

func formatLearningsForDiagnostics(items []learnings.Learning) string {
	if len(items) == 0 {
		return ""
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("[%s] %s", item.Category, item.Content))
	}
	return strings.Join(lines, "\n")
}

// ValidateSpecName checks that a spec name doesn't contain path traversal characters
func ValidateSpecName(name string) error {
	if name == "" {
		return fmt.Errorf("empty spec name")
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid spec name %q: contains path traversal characters", name)
	}
	return nil
}

// ParseScopeEstimate parses Claude's JSON scope estimate output into a ScopeEstimate struct
func ParseScopeEstimate(output string) (*ScopeEstimate, error) {
	if output == "" {
		return nil, fmt.Errorf("scope estimate output is empty")
	}

	var estimate ScopeEstimate
	if err := jsonutil.ExtractJSON(output, &estimate); err != nil {
		return nil, fmt.Errorf("parsing scope estimate JSON: %w", err)
	}

	estimate.normalizeNilFields()

	return &estimate, nil
}

// ParseSuccessLearning parses Claude's JSON success learning output into a SuccessLearning struct
func ParseSuccessLearning(output string) (*SuccessLearning, error) {
	if output == "" {
		return nil, fmt.Errorf("success learning output is empty")
	}

	var learning SuccessLearning
	if err := jsonutil.ExtractObject(output, &learning); err != nil {
		return nil, fmt.Errorf("parsing success learning JSON: %w", err)
	}

	learning.Category = normalizeSuccessLearningCategory(learning.Category)

	return &learning, nil
}

func normalizeSuccessLearningCategory(category string) string {
	switch category {
	case successLearningCategoryConventions, successLearningCategoryGotchas, successLearningCategoryPatterns:
		return category
	default:
		return successLearningCategoryGotchas
	}
}
