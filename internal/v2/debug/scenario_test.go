package debug

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestScenario_NilPointerDetectionAndLearning covers the nil pointer dereference scenario.
func TestScenario_NilPointerDetectionAndLearning(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a code file with a hypothetical issue
	codeFile := filepath.Join(tmpDir, "handler.go")
	if err := os.WriteFile(codeFile, []byte("package main\n\nfunc handle(p *Request) {\n  // process p\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a fix context for a nil pointer error
	fixCtx := &FixContext{
		FailedStage:   "validate",
		ErrorMsg:      "nil pointer dereference when accessing p.Body",
		FilesInvolved: []string{"handler.go"},
		WorktreeRoot:  tmpDir,
	}

	// Apply the fix
	fixResult, err := ApplyFix(ctx, fixCtx)
	if err != nil {
		t.Fatalf("ApplyFix failed: %v", err)
	}

	if !fixResult.Applied {
		t.Error("fix should be applied")
	}

	// Extract a learning from the same failure
	learnCtx := &LearnContext{
		FailedStage:  "validate",
		ErrorMsg:     "nil pointer dereference when accessing p.Body",
		BeadTitle:    "add nil safety checks",
		WorktreeRoot: tmpDir,
		Pattern:      "nil pointer dereference",
		IsAutonomous: true,
	}

	learnResult, err := ExtractLearning(ctx, learnCtx)
	if err != nil {
		t.Fatalf("ExtractLearning failed: %v", err)
	}

	if !learnResult.Extracted {
		t.Error("learning should be extracted")
	}

	if learnResult.LearningEntry == "" {
		t.Error("learning entry should be non-empty for autonomous fix")
	}

	// Persist the learning
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	if err := os.WriteFile(learningsPath, []byte("# Learnings\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := PersistLearning(learningsPath, learnResult.LearningEntry); err != nil {
		t.Fatalf("PersistLearning failed: %v", err)
	}

	// Verify learning was persisted
	content, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(content) <= len("# Learnings\n\n") {
		t.Error("learning should be persisted to file")
	}
}

// TestScenario_PromptAmbiguityRecommendation covers the systemic change scenario.
func TestScenario_PromptAmbiguityRecommendation(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a learning context for a systemic issue
	learnCtx := &LearnContext{
		FailedStage:  "build",
		ErrorMsg:     "conflicting interpretations from ambiguous prompt fragment",
		BeadTitle:    "clarify API documentation stage",
		WorktreeRoot: tmpDir,
		Pattern:      "ambiguous prompt language",
		IsAutonomous: false, // Requires human judgment
	}

	// Extract learning - should create recommendation
	learnResult, err := ExtractLearning(ctx, learnCtx)
	if err != nil {
		t.Fatalf("ExtractLearning failed: %v", err)
	}

	if !learnResult.Extracted {
		t.Error("learning should be extracted even for systemic changes")
	}

	if learnResult.Recommendation == "" {
		t.Error("recommendation should be non-empty for systemic change")
	}

	if learnResult.LearningEntry != "" {
		t.Error("learning entry should be empty for systemic change (recommendation instead)")
	}
}
