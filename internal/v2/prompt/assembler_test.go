package prompt

import (
	"strings"
	"testing"
)

func TestPromptAssemblerAddsLayerMarkers(t *testing.T) {
	assembler := NewPromptAssembler("base layer", "project layer", "instance layer", "fragment layer")
	output := assembler.Assemble("", BeadInfo{})

	expectedSequence := []string{
		"=== BASE ===",
		"base layer",
		"=== PROJECT ===",
		"project layer",
		"=== INSTANCE ===",
		"instance layer",
		"=== FRAGMENT ===",
		"fragment layer",
	}

	lastIndex := 0
	for _, fragment := range expectedSequence {
		idx := strings.Index(output, fragment)
		if idx == -1 {
			t.Fatalf("output missing %q", fragment)
		}
		if idx < lastIndex {
			t.Fatalf("%q appears out of order", fragment)
		}
		lastIndex = idx
	}
}

func TestPromptAssemblerSkipsEmptyLayers(t *testing.T) {
	assembler := NewPromptAssembler("base layer", "", "instance layer", "")
	output := assembler.Assemble("", BeadInfo{})

	if !strings.Contains(output, "=== BASE ===") {
		t.Fatalf("base section missing")
	}
	if strings.Contains(output, "=== PROJECT ===") {
		t.Fatalf("project section should be omitted when empty")
	}
	if !strings.Contains(output, "=== INSTANCE ===") {
		t.Fatalf("instance section missing")
	}
	if strings.Contains(output, "=== FRAGMENT ===") {
		t.Fatalf("fragment section should be omitted when empty")
	}
}

func TestPromptAssemblerSkipsWhitespaceOnlyLayers(t *testing.T) {
	assembler := NewPromptAssembler("base layer", "  \n\t", "instance layer", "fragment layer")
	output := assembler.Assemble("", BeadInfo{})

	if strings.Contains(output, "=== PROJECT ===") {
		t.Fatalf("project section should be omitted when it contains only whitespace")
	}
	if !strings.Contains(output, "=== BASE ===") {
		t.Fatalf("base section missing")
	}
	if !strings.Contains(output, "=== INSTANCE ===") {
		t.Fatalf("instance section missing")
	}
	if !strings.Contains(output, "=== FRAGMENT ===") {
		t.Fatalf("fragment section missing")
	}
}

func TestShapeBudgetSmallBeadGets50Percent(t *testing.T) {
	content := strings.Repeat("x", 10000)
	shaped, report := ShapeBudget(content, 1, 10000)

	if len(shaped) > 5000 {
		t.Fatalf("shaped len = %d, want <= 5000 (50%% of 10000)", len(shaped))
	}
	if !report.Trimmed {
		t.Fatal("expected Trimmed = true")
	}
	if report.AdjustedBudget != 5000 {
		t.Fatalf("AdjustedBudget = %d, want 5000", report.AdjustedBudget)
	}
}

func TestShapeBudgetMediumBeadGets75Percent(t *testing.T) {
	content := strings.Repeat("x", 10000)
	shaped, report := ShapeBudget(content, 3, 10000)

	if len(shaped) > 7500 {
		t.Fatalf("shaped len = %d, want <= 7500 (75%% of 10000)", len(shaped))
	}
	if !report.Trimmed {
		t.Fatal("expected Trimmed = true")
	}
	if report.AdjustedBudget != 7500 {
		t.Fatalf("AdjustedBudget = %d, want 7500", report.AdjustedBudget)
	}
}

func TestShapeBudgetLargeBeadGets100Percent(t *testing.T) {
	content := strings.Repeat("x", 10000)
	shaped, report := ShapeBudget(content, 6, 10000)

	if len(shaped) != 10000 {
		t.Fatalf("shaped len = %d, want 10000 (100%% budget, no trim needed)", len(shaped))
	}
	if report.Trimmed {
		t.Fatal("expected Trimmed = false for large bead within budget")
	}
	if report.AdjustedBudget != 10000 {
		t.Fatalf("AdjustedBudget = %d, want 10000", report.AdjustedBudget)
	}
}

func TestShapeBudgetReportsTrimmingDetails(t *testing.T) {
	content := strings.Repeat("x", 10000)
	_, report := ShapeBudget(content, 2, 10000)

	if report.OriginalSize != 10000 {
		t.Fatalf("OriginalSize = %d, want 10000", report.OriginalSize)
	}
	if report.ShapedSize != 5000 {
		t.Fatalf("ShapedSize = %d, want 5000", report.ShapedSize)
	}
	if report.TrimmedBytes != 5000 {
		t.Fatalf("TrimmedBytes = %d, want 5000", report.TrimmedBytes)
	}
	if report.FileCount != 2 {
		t.Fatalf("FileCount = %d, want 2", report.FileCount)
	}
}

func TestAssembleWithPhaseFiltersBase(t *testing.T) {
	base := "## build\nbuild instructions\n\n## validate\nvalidate instructions"
	assembler := NewPromptAssembler(base, "project context", "", "")
	output := assembler.Assemble("build", BeadInfo{FileCount: 10})

	if !strings.Contains(output, "build instructions") {
		t.Fatal("output should contain build instructions")
	}
	if strings.Contains(output, "validate instructions") {
		t.Fatal("output should NOT contain validate instructions when phase=build")
	}
}

func TestAssembleStoresLastReport(t *testing.T) {
	assembler := NewPromptAssembler("base", "project", "", "")
	assembler.Assemble("", BeadInfo{FileCount: 1})

	report := assembler.LastReport()
	if report == nil {
		t.Fatal("LastReport should not be nil after Assemble")
	}
	if report.FileCount != 1 {
		t.Fatalf("report.FileCount = %d, want 1", report.FileCount)
	}
}
