package debug

import (
	"fmt"

	"github.com/danabrams/gromit/internal/learnings"
)

// FailureContext captures information about a failed stage for pattern detection.
type FailureContext struct {
	Message string
	Stage   string
	BeadID  string
}

// LearningPattern describes a known pattern that can be fixed autonomously.
type LearningPattern struct {
	ID              string
	Category        string
	BeadID          string
	LearningContent string
	FixPlan         FixPlan
	Trigger         func(FailureContext) bool
}

// DetectLearnablePattern returns the first pattern whose trigger matches the context.
func DetectLearnablePattern(ctx FailureContext, patterns []LearningPattern) *LearningPattern {
	for i := range patterns {
		if patterns[i].Trigger == nil {
			continue
		}
		if patterns[i].Trigger(ctx) {
			return &patterns[i]
		}
	}
	return nil
}

// ApplyLearning applies the fix plan and records the learning entry.
func ApplyLearning(repoRoot string, pattern LearningPattern) (*FixResult, error) {
	result, err := ApplyPlan(repoRoot, pattern.FixPlan)
	if err != nil {
		return nil, err
	}

	file, err := learnings.NewFile(repoRoot)
	if err != nil {
		return result, fmt.Errorf("create learnings file: %w", err)
	}
	if err := file.Load(); err != nil {
		return result, fmt.Errorf("load learnings: %w", err)
	}
	if _, err := file.Add(pattern.BeadID, pattern.LearningContent, pattern.Category); err != nil {
		return result, fmt.Errorf("add learning: %w", err)
	}
	return result, nil
}
