package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Scope denotes whether a fix is learnable or systemic.
type Scope string

const (
	ScopeLearnable Scope = "learnable"
	ScopeSystemic  Scope = "systemic"
)

// FileEdit represents a replacement of a file's content.
type FileEdit struct {
	Path       string
	NewContent []byte
}

// FixPlan describes what files to update and whether the change is systemic.
type FixPlan struct {
	Scope          Scope
	Edits          []FileEdit
	Recommendation string // optional recommendation for human review when systemic
}

// FixResult describes the outcome of applying a plan.
type FixResult struct {
	Scope                Scope
	AppliedPaths         []string
	RecommendHumanReview string
}

// ApplyPlan writes the edits relative to repoRoot and returns the applied paths.
func ApplyPlan(repoRoot string, plan FixPlan) (*FixResult, error) {
	if len(plan.Edits) == 0 {
		return nil, fmt.Errorf("no edits to apply")
	}

	if plan.Scope == ScopeSystemic && strings.TrimSpace(plan.Recommendation) == "" {
		return nil, fmt.Errorf("systemic fixes require a recommendation for human review")
	}

	applied := make([]string, 0, len(plan.Edits))
	for _, edit := range plan.Edits {
		if edit.Path == "" {
			return nil, fmt.Errorf("empty edit path")
		}
		absPath := filepath.Join(repoRoot, edit.Path)
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return nil, fmt.Errorf("create dir for %s: %w", edit.Path, err)
		}
		if err := os.WriteFile(absPath, edit.NewContent, 0o644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", edit.Path, err)
		}
		applied = append(applied, edit.Path)
	}

	result := &FixResult{
		Scope:        plan.Scope,
		AppliedPaths: applied,
	}
	if plan.Scope == ScopeSystemic {
		result.RecommendHumanReview = plan.Recommendation
	}
	return result, nil
}
