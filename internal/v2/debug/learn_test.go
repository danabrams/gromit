package debug

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectLearnablePattern(t *testing.T) {
	patterns := []LearningPattern{
		{
			ID: "boundary_missing",
			Trigger: func(ctx FailureContext) bool {
				return strings.Contains(ctx.Message, "boundary")
			},
		},
	}
	ctx := FailureContext{Message: "runtime boundary guard failure"}
	pattern := DetectLearnablePattern(ctx, patterns)
	if pattern == nil {
		t.Fatal("expected learnable pattern")
	}
	if pattern.ID != "boundary_missing" {
		t.Fatalf("pattern ID = %q, want %q", pattern.ID, "boundary_missing")
	}
}

func TestApplyLearningRecordsLearning(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(target, []byte("stale"), 0o644); err != nil {
		t.Fatalf("prepare file: %v", err)
	}
	pattern := LearningPattern{
		ID:              "learning-fix",
		BeadID:          "debug-bead",
		Category:        "patterns",
		LearningContent: "Fix guard clause pattern that leaves state uninitialized.",
		FixPlan: FixPlan{
			Scope: ScopeLearnable,
			Edits: []FileEdit{
				{Path: "target.txt", NewContent: []byte("fresh")},
			},
		},
	}
	if _, err := ApplyLearning(root, pattern); err != nil {
		t.Fatalf("ApplyLearning error: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != "fresh" {
		t.Fatalf("target = %q, want %q", string(data), "fresh")
	}
	learningsPath := filepath.Join(root, "LEARNINGS.md")
	content, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("read learnings: %v", err)
	}
	if !strings.Contains(string(content), pattern.LearningContent) {
		t.Fatalf("learning entry missing: %q", pattern.LearningContent)
	}
}
