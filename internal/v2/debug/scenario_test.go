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

	learning := ExtractLearning(autonomousLearningInput("Always guard pointer access in handlers before dereferencing request fields."))
	if learning.LearningsEntry == "" {
		t.Error("learning entry should be non-empty for autonomous fix")
	}
	if !learning.Autonomous {
		t.Error("autonomous learning should be marked as such")
	}
	if learning.SystemicRecommendation != "" {
		t.Error("autonomous learning should not produce a systemic recommendation")
	}

	// Persist the learning
	learningsPath := filepath.Join(tmpDir, "LEARNINGS.md")
	if err := os.WriteFile(learningsPath, []byte("# Learnings\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := PersistLearning(learningsPath, learning.LearningsEntry); err != nil {
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
	learning := ExtractLearning(systemicLearningInput(RootCauseUnclearBead, "Prompt fragment is ambiguous and needs clarity"))
	if learning.SystemicRecommendation == "" {
		t.Error("recommendation should be non-empty for systemic change")
	}

	if learning.LearningsEntry != "" {
		t.Error("learning entry should be empty for systemic change (recommendation instead)")
	}
	if learning.Autonomous {
		t.Error("systemic recommendation should not be marked autonomous")
	}
}

func autonomousLearningInput(entry string) LearningExtractionInput {
	return LearningExtractionInput{
		LearningsEntry: entry,
	}
}

func systemicLearningInput(rootCause RootCause, signal string) LearningExtractionInput {
	return LearningExtractionInput{
		RootCause:      rootCause,
		LearningsEntry: signal,
	}
}
