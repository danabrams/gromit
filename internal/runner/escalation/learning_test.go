package escalation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// --- Learning extraction tests ---

func TestExtractLearning_SavesAnalysisLearning(t *testing.T) {
	// When an analysis contains a learning string, ExtractLearning should
	// persist it to the learnings file via lf.Add().
	dir := t.TempDir()
	lf, err := learnings.NewFile(dir)
	if err != nil {
		t.Fatalf("failed to create learnings file: %v", err)
	}

	learningText := "Always check for nil pointer before accessing struct fields"
	analysis := &analyzer.Analysis{
		Category:    "logic_error",
		Recoverable: true,
		RootCause:   "nil pointer dereference",
		Learning:    &learningText,
	}

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-learn-001", Title: "Fix crash"},
		Result: &runtypes.IterationResult{},
	}

	ExtractLearning(bc, analysis, lf)

	// Read back the learnings file and verify the learning was persisted
	content, err := os.ReadFile(filepath.Join(dir, "LEARNINGS.md"))
	if err != nil {
		t.Fatalf("failed to read learnings file: %v", err)
	}
	if !strings.Contains(string(content), learningText) {
		t.Errorf("learnings file should contain %q, got:\n%s", learningText, string(content))
	}
}

func TestExtractLearning_NilLearningIsNoOp(t *testing.T) {
	// When analysis.Learning is nil, ExtractLearning should not write anything.
	dir := t.TempDir()
	lf, err := learnings.NewFile(dir)
	if err != nil {
		t.Fatalf("failed to create learnings file: %v", err)
	}

	analysis := &analyzer.Analysis{
		Category: "logic_error",
		Learning: nil,
	}

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-learn-002", Title: "No learning"},
		Result: &runtypes.IterationResult{},
	}

	ExtractLearning(bc, analysis, lf)

	content, err := os.ReadFile(filepath.Join(dir, "LEARNINGS.md"))
	if os.IsNotExist(err) {
		// File was never created — correct for a no-op
		return
	}
	if err != nil {
		t.Fatalf("failed to read learnings file: %v", err)
	}
	// File should be empty or contain only the default header
	if strings.Contains(string(content), "test-learn-002") {
		t.Error("learnings file should not contain bead ID when learning is nil")
	}
}

func TestExtractLearning_NilBeadContextIsNoOp(t *testing.T) {
	// When bc or bc.Bead is nil, ExtractLearning should not panic.
	dir := t.TempDir()
	lf, err := learnings.NewFile(dir)
	if err != nil {
		t.Fatalf("failed to create learnings file: %v", err)
	}

	analysis := &analyzer.Analysis{Category: "logic_error"}
	learningText := "some learning"
	analysis.Learning = &learningText

	// Should not panic with nil BeadContext
	ExtractLearning(nil, analysis, lf)

	// Should not panic with nil Bead
	bc := &runtypes.BeadContext{Bead: nil, Result: &runtypes.IterationResult{}}
	ExtractLearning(bc, analysis, lf)
}

func TestExtractSyntheticLearning_PersistsMessage(t *testing.T) {
	// ExtractSyntheticLearning should save a custom message string to the
	// learnings file with the "patterns" category.
	dir := t.TempDir()
	lf, err := learnings.NewFile(dir)
	if err != nil {
		t.Fatalf("failed to create learnings file: %v", err)
	}

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "test-synth-001", Title: "Synthetic test"},
		Result: &runtypes.IterationResult{},
	}

	message := "Custom synthetic learning about error handling patterns"
	ExtractSyntheticLearning(bc, message, lf)

	content, err := os.ReadFile(filepath.Join(dir, "LEARNINGS.md"))
	if err != nil {
		t.Fatalf("failed to read learnings file: %v", err)
	}
	if !strings.Contains(string(content), message) {
		t.Errorf("learnings file should contain %q, got:\n%s", message, string(content))
	}
}

func TestExtractScopeTooLargeLearning_FormatsCorrectly(t *testing.T) {
	// ExtractScopeTooLargeLearning should create a learning message that
	// includes the bead title and model name, then persist it.
	dir := t.TempDir()
	lf, err := learnings.NewFile(dir)
	if err != nil {
		t.Fatalf("failed to create learnings file: %v", err)
	}

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "scope-001", Title: "Big refactor bead"},
		Model:  "sonnet",
		Result: &runtypes.IterationResult{},
	}

	ExtractScopeTooLargeLearning(bc, "touches 15 files", lf)

	content, err := os.ReadFile(filepath.Join(dir, "LEARNINGS.md"))
	if err != nil {
		t.Fatalf("failed to read learnings file: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "Big refactor bead") {
		t.Errorf("learning should mention bead title, got:\n%s", s)
	}
	if !strings.Contains(s, "sonnet") {
		t.Errorf("learning should mention model name, got:\n%s", s)
	}
}

func TestExtractTimeoutLearning_FormatsCorrectly(t *testing.T) {
	// ExtractTimeoutLearning should create a learning message that includes
	// the bead title and model name, then persist it.
	dir := t.TempDir()
	lf, err := learnings.NewFile(dir)
	if err != nil {
		t.Fatalf("failed to create learnings file: %v", err)
	}

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "timeout-001", Title: "Slow bead"},
		Model:  "haiku",
		Result: &runtypes.IterationResult{},
	}

	ExtractTimeoutLearning(bc, lf)

	content, err := os.ReadFile(filepath.Join(dir, "LEARNINGS.md"))
	if err != nil {
		t.Fatalf("failed to read learnings file: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "Slow bead") {
		t.Errorf("learning should mention bead title, got:\n%s", s)
	}
	if !strings.Contains(s, "haiku") {
		t.Errorf("learning should mention model name, got:\n%s", s)
	}
}

func TestExtractSuccessLearning_SkipsForLowTier(t *testing.T) {
	// ExtractSuccessLearning should skip learning extraction for haiku/low-tier
	// beads, since they don't produce novel enough patterns to learn from.
	dir := t.TempDir()
	lf, err := learnings.NewFile(dir)
	if err != nil {
		t.Fatalf("failed to create learnings file: %v", err)
	}

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "low-001", Title: "Simple bead"},
		Tier:   provider.TierLow,
		Model:  "haiku",
		Result: &runtypes.IterationResult{},
	}

	// Should not call the router or persist any learning
	ExtractSuccessLearning(context.Background(), bc, cfg, lf, nil, nil, nil)

	content, err := os.ReadFile(filepath.Join(dir, "LEARNINGS.md"))
	if os.IsNotExist(err) {
		// File was never created — correct for a no-op
		return
	}
	if err != nil {
		t.Fatalf("failed to read learnings file: %v", err)
	}
	if strings.Contains(string(content), "low-001") {
		t.Error("should not extract success learning for low-tier beads")
	}
}

// TestExtractLearning_NeverSkippedForLowTier verifies that failure learning extraction
// bypasses the tier filter that is applied to success learning. Failure paths must always
// extract and persist learning, regardless of tier (low-tier haiku beads).
func TestExtractLearning_NeverSkippedForLowTier(t *testing.T) {
	dir := t.TempDir()
	lf, err := learnings.NewFile(dir)
	if err != nil {
		t.Fatalf("failed to create learnings file: %v", err)
	}

	learningText := "Low-tier bead failure pattern that must be learned from"
	analysis := &analyzer.Analysis{
		Category:    "logic_error",
		Recoverable: true,
		RootCause:   "missed edge case",
		Learning:    &learningText,
	}

	bc := &runtypes.BeadContext{
		Bead:   &bead.Bead{ID: "fail-low-001", Title: "Low-tier failure"},
		Tier:   provider.TierLow,
		Model:  "haiku",
		Result: &runtypes.IterationResult{},
	}

	// Extract failure learning for a low-tier bead
	ExtractLearning(bc, analysis, lf)

	// Verify the learning was persisted despite the low tier
	content, err := os.ReadFile(filepath.Join(dir, "LEARNINGS.md"))
	if err != nil {
		t.Fatalf("failed to read learnings file: %v", err)
	}
	if !strings.Contains(string(content), learningText) {
		t.Errorf("failure learning should be extracted even for low-tier beads, got:\n%s", string(content))
	}
}
