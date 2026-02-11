package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// TestRenderDecomposeGuidelinesStructure verifies that the decompose template
// contains the updated bead sizing guidelines with proper structure.
func TestRenderDecomposeGuidelinesStructure(t *testing.T) {
	// Use the real PROMPT_decompose.md template from the project
	templatesDir := filepath.Join("..", "..", ".gromit", "templates")

	// Verify template exists
	templatePath := filepath.Join(templatesDir, "PROMPT_decompose.md")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skipf("skipping: real template not found at %s", templatePath)
	}

	r := &Renderer{templatesDir: templatesDir}

	testBead := &bead.Bead{
		ID:              "test-123",
		Title:           "Test task for decomposition",
		Priority:        1,
		Description:     "A task that needs decomposition",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	ctx := &DecomposeContext{
		Bead:       testBead,
		ATDDActive: false,
	}

	result, err := r.RenderDecompose(ctx)
	if err != nil {
		t.Fatalf("RenderDecompose() error = %v", err)
	}

	t.Run("contains deliverable behavior guidance", func(t *testing.T) {
		// Expected failure: Template does not contain "one deliverable behavior" phrasing yet
		if !strings.Contains(result, "one deliverable behavior") {
			t.Error("expected 'one deliverable behavior' guidance in Guidelines section")
		}
		if !strings.Contains(result, "a single observable change that a caller or user could verify") {
			t.Error("expected explanation of deliverable behavior in output")
		}
	})

	t.Run("contains soft file limit guidance", func(t *testing.T) {
		// Expected failure: Template still has old "max 2 files" guidance instead of "4-5 files"
		if !strings.Contains(result, "Soft file limit of 4-5") {
			t.Error("expected 'Soft file limit of 4-5' in Guidelines section")
		}
		if !strings.Contains(result, "6+ files across unrelated packages") {
			t.Error("expected '6+ files across unrelated packages' threshold in output")
		}
	})

	t.Run("contains grouping rules subsection", func(t *testing.T) {
		// Expected failure: Grouping rules subsection does not exist as a structured section yet
		if !strings.Contains(result, "Never split these natural units") {
			t.Error("expected 'Never split these natural units' subsection header")
		}
	})

	t.Run("contains all five never-split patterns", func(t *testing.T) {
		// Expected failure: Not all five patterns are documented in the template yet
		patterns := []string{
			"Interface + implementation + mock updates",
			"Implementation + its tests",
			"Companion methods in same package",
			"Command flags + wiring",
			"Template + registration",
		}

		for _, pattern := range patterns {
			if !strings.Contains(result, pattern) {
				t.Errorf("expected never-split pattern %q in output", pattern)
			}
		}
	})

	t.Run("contains splitting rules subsection", func(t *testing.T) {
		// Expected failure: Splitting rules subsection does not exist in current template
		if !strings.Contains(result, "When to Split") || !strings.Contains(result, "split by") {
			t.Error("expected 'When to Split' or splitting rules subsection")
		}
	})

	t.Run("splitting rules includes acceptance criteria threshold", func(t *testing.T) {
		// Expected failure: Template does not include "4+ acceptance criteria" splitting rule yet
		if !strings.Contains(result, "4+ acceptance criteria") {
			t.Error("expected '4+ acceptance criteria' as a splitting threshold")
		}
	})

	t.Run("splitting rules includes file count threshold", func(t *testing.T) {
		// Expected failure: Splitting rules subsection with specific thresholds doesn't exist yet
		if !strings.Contains(result, "6+ files") && !strings.Contains(result, "split by package boundary") {
			t.Error("expected '6+ files' or 'split by package boundary' in splitting rules")
		}
	})

	t.Run("splitting rules includes independence criterion", func(t *testing.T) {
		// Expected failure: Independence-based splitting rule not documented yet
		if !strings.Contains(result, "independently useful") || !strings.Contains(result, "don't need each other to compile") {
			t.Error("expected independence criterion in splitting rules")
		}
	})

	t.Run("splitting rules includes design decision boundary", func(t *testing.T) {
		// Expected failure: Design decision splitting criterion not documented yet
		if !strings.Contains(result, "design decision") && !strings.Contains(result, "decision boundary") {
			t.Error("expected design decision boundary criterion in splitting rules")
		}
	})

	t.Run("preserves Avoiding Sibling Overlap section", func(t *testing.T) {
		// This should pass - the spec says to preserve this section
		if !strings.Contains(result, "Avoiding Sibling Overlap") {
			t.Error("expected 'Avoiding Sibling Overlap' section to be preserved")
		}
		if !strings.Contains(result, "would this task's acceptance criteria still fail") {
			t.Error("expected cross-check question in Avoiding Sibling Overlap section")
		}
	})
}

// TestRenderDecomposeGuidelinesOrdering verifies that sections appear in the
// correct order within the Guidelines section.
func TestRenderDecomposeGuidelinesOrdering(t *testing.T) {
	templatesDir := filepath.Join("..", "..", ".gromit", "templates")
	templatePath := filepath.Join(templatesDir, "PROMPT_decompose.md")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skipf("skipping: real template not found at %s", templatePath)
	}

	r := &Renderer{templatesDir: templatesDir}

	testBead := &bead.Bead{
		ID:              "test-456",
		Title:           "Another test task",
		Priority:        2,
		Description:     "Task for ordering test",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	ctx := &DecomposeContext{
		Bead:       testBead,
		ATDDActive: false,
	}

	result, err := r.RenderDecompose(ctx)
	if err != nil {
		t.Fatalf("RenderDecompose() error = %v", err)
	}

	// Expected failure: Current template doesn't have proper subsection structure for splitting rules
	// Find positions of key markers
	deliverableBehaviorPos := strings.Index(result, "one deliverable behavior")
	neverSplitPos := strings.Index(result, "Never split these natural units")
	avoidingOverlapPos := strings.Index(result, "Avoiding Sibling Overlap")

	if deliverableBehaviorPos == -1 {
		t.Fatal("cannot find 'one deliverable behavior' in output")
	}
	if neverSplitPos == -1 {
		t.Fatal("cannot find 'Never split these natural units' in output")
	}
	if avoidingOverlapPos == -1 {
		t.Fatal("cannot find 'Avoiding Sibling Overlap' in output")
	}

	// Verify ordering: deliverable behavior guidance comes before grouping rules
	if deliverableBehaviorPos >= neverSplitPos {
		t.Error("expected 'one deliverable behavior' to appear before 'Never split these natural units'")
	}

	// Verify ordering: grouping rules come before Avoiding Sibling Overlap
	if neverSplitPos >= avoidingOverlapPos {
		t.Error("expected 'Never split these natural units' to appear before 'Avoiding Sibling Overlap'")
	}
}

// TestRenderDecomposeGuidelinesNoOldPhrasing verifies that outdated guidance
// has been removed from the template.
func TestRenderDecomposeGuidelinesNoOldPhrasing(t *testing.T) {
	templatesDir := filepath.Join("..", "..", ".gromit", "templates")
	templatePath := filepath.Join(templatesDir, "PROMPT_decompose.md")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skipf("skipping: real template not found at %s", templatePath)
	}

	r := &Renderer{templatesDir: templatesDir}

	testBead := &bead.Bead{
		ID:              "test-789",
		Title:           "Test for old phrasing",
		Priority:        1,
		Description:     "Verify old guidance removed",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	ctx := &DecomposeContext{
		Bead:       testBead,
		ATDDActive: false,
	}

	result, err := r.RenderDecompose(ctx)
	if err != nil {
		t.Fatalf("RenderDecompose() error = %v", err)
	}

	t.Run("does not contain 'single concern' phrasing", func(t *testing.T) {
		// This test verifies the old phrasing is gone - it may already pass
		// if changes have been made, but would fail on the old template
		if strings.Contains(result, "focused on a single concern") {
			t.Error("expected old 'focused on a single concern' phrasing to be removed")
		}
	})

	t.Run("does not contain max 2 files guidance", func(t *testing.T) {
		// This may already pass if file limit was updated
		if strings.Contains(result, "max 2 files") || strings.Contains(result, "maximum of 2 files") {
			t.Error("expected old 'max 2 files' guidance to be removed")
		}
		if strings.Contains(result, "more than 2 files") && !strings.Contains(result, "test") {
			// "more than 2 files" might appear in test names, but shouldn't be in actual guidance
			t.Error("expected old 'more than 2 files' threshold to be removed from guidelines")
		}
	})
}

// TestRenderDecomposeExampleIntegrity verifies that the JSON example and
// instructions remain intact after guideline updates.
func TestRenderDecomposeExampleIntegrity(t *testing.T) {
	templatesDir := filepath.Join("..", "..", ".gromit", "templates")
	templatePath := filepath.Join(templatesDir, "PROMPT_decompose.md")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Skipf("skipping: real template not found at %s", templatePath)
	}

	r := &Renderer{templatesDir: templatesDir}

	testBead := &bead.Bead{
		ID:              "test-example",
		Title:           "Verify example structure",
		Priority:        1,
		Description:     "Check that examples remain intact",
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	ctx := &DecomposeContext{
		Bead:       testBead,
		ATDDActive: false,
	}

	result, err := r.RenderDecompose(ctx)
	if err != nil {
		t.Fatalf("RenderDecompose() error = %v", err)
	}

	// Verify the example format structure is still present
	if !strings.Contains(result, "Example format:") {
		t.Error("expected 'Example format:' section to be preserved")
	}

	if !strings.Contains(result, `"title":`) {
		t.Error("expected JSON example with 'title' field")
	}

	if !strings.Contains(result, `"depends_on":`) {
		t.Error("expected JSON example with 'depends_on' field")
	}

	if !strings.Contains(result, `"acceptance_criteria":`) {
		t.Error("expected JSON example with 'acceptance_criteria' field")
	}

	// Verify output format instructions
	if !strings.Contains(result, "Respond with ONLY the JSON array") {
		t.Error("expected instruction to respond with only JSON array")
	}
}
