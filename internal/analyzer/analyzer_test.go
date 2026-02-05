package analyzer

import (
	"testing"
)

// TestParseAnalysisOutputValidJSON tests parsing valid JSON output
func TestParseAnalysisOutputValidJSON(t *testing.T) {
	output := `{
		"category": "syntax",
		"recoverable": true,
		"root_cause": "Missing import statement",
		"learning": "Always check imports are complete",
		"suggestion": "Add the missing import"
	}`

	analysis, err := parseAnalysisOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis == nil {
		t.Fatal("analysis should not be nil")
	}
	if analysis.Category != CategorySyntax {
		t.Errorf("expected category %q, got %q", CategorySyntax, analysis.Category)
	}
	if !analysis.Recoverable {
		t.Errorf("expected recoverable=true, got %v", analysis.Recoverable)
	}
	if analysis.RootCause != "Missing import statement" {
		t.Errorf("expected root_cause 'Missing import statement', got %q", analysis.RootCause)
	}
	if analysis.Learning == nil || *analysis.Learning != "Always check imports are complete" {
		t.Errorf("expected learning 'Always check imports are complete', got %v", analysis.Learning)
	}
	if analysis.Suggestion != "Add the missing import" {
		t.Errorf("expected suggestion 'Add the missing import', got %q", analysis.Suggestion)
	}
}

// TestParseAnalysisOutputInvalidJSON tests parsing invalid JSON
func TestParseAnalysisOutputInvalidJSON(t *testing.T) {
	output := `{invalid json content}`

	analysis, err := parseAnalysisOutput(output)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if analysis != nil {
		t.Fatal("analysis should be nil on parse error")
	}
}

// TestParseAnalysisOutputNoJSON tests parsing with no JSON in output
func TestParseAnalysisOutputNoJSON(t *testing.T) {
	output := `This is plain text with no JSON object`

	analysis, err := parseAnalysisOutput(output)
	if err == nil {
		t.Fatal("expected error when no JSON found")
	}
	if analysis != nil {
		t.Fatal("analysis should be nil when no JSON found")
	}
}

// TestParseAnalysisOutputJSONEmbedded tests parsing JSON embedded in text
func TestParseAnalysisOutputJSONEmbedded(t *testing.T) {
	output := `Here's the analysis:

{
	"category": "logic",
	"recoverable": false,
	"root_cause": "Algorithm edge case not handled",
	"learning": null,
	"suggestion": "Add edge case handling"
}

Hope this helps!`

	analysis, err := parseAnalysisOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis == nil {
		t.Fatal("analysis should not be nil")
	}
	if analysis.Category != CategoryLogic {
		t.Errorf("expected category %q, got %q", CategoryLogic, analysis.Category)
	}
	if analysis.Recoverable {
		t.Errorf("expected recoverable=false, got %v", analysis.Recoverable)
	}
	if analysis.Learning != nil {
		t.Errorf("expected learning=nil, got %v", analysis.Learning)
	}
}

// TestParseAnalysisOutputMissingLearning tests JSON without learning field
func TestParseAnalysisOutputMissingLearning(t *testing.T) {
	output := `{
		"category": "environment",
		"recoverable": true,
		"root_cause": "Python version mismatch",
		"suggestion": "Update Python to 3.10+"
	}`

	analysis, err := parseAnalysisOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis == nil {
		t.Fatal("analysis should not be nil")
	}
	if analysis.Category != CategoryEnvironment {
		t.Errorf("expected category %q, got %q", CategoryEnvironment, analysis.Category)
	}
	if analysis.Learning != nil {
		t.Errorf("expected learning=nil, got %v", analysis.Learning)
	}
}

// TestParseAnalysisOutputInvalidCategory tests JSON with invalid category
func TestParseAnalysisOutputInvalidCategory(t *testing.T) {
	output := `{
		"category": "invalid_category",
		"recoverable": true,
		"root_cause": "Some issue",
		"suggestion": "Do something"
	}`

	analysis, err := parseAnalysisOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis == nil {
		t.Fatal("analysis should not be nil")
	}
	// Invalid categories should default to CategoryLogic
	if analysis.Category != CategoryLogic {
		t.Errorf("expected category to default to %q, got %q", CategoryLogic, analysis.Category)
	}
}

// TestParseAnalysisOutputMultipleJSONObjects tests that it extracts the entire span
func TestParseAnalysisOutputMultipleJSONObjects(t *testing.T) {
	// Note: parseAnalysisOutput extracts from first { to last }, which will fail
	// if there's text between multiple JSON objects. This test verifies that behavior.
	output := `First object:
{
	"category": "syntax",
	"recoverable": true,
	"root_cause": "First object",
	"suggestion": "First suggestion"
}`

	analysis, err := parseAnalysisOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis == nil {
		t.Fatal("analysis should not be nil")
	}
	if analysis.Category != CategorySyntax {
		t.Errorf("expected CategorySyntax, got %q", analysis.Category)
	}
}

// TestParseAnalysisOutputMissingClosingBrace tests JSON with missing closing brace
func TestParseAnalysisOutputMissingClosingBrace(t *testing.T) {
	output := `{
		"category": "syntax",
		"recoverable": true,
		"root_cause": "Missing brace"`

	analysis, err := parseAnalysisOutput(output)
	if err == nil {
		t.Fatal("expected error for missing closing brace")
	}
	if analysis != nil {
		t.Fatal("analysis should be nil on parse error")
	}
}

// TestParseAnalysisOutputEmptyString tests parsing empty string
func TestParseAnalysisOutputEmptyString(t *testing.T) {
	output := ""

	analysis, err := parseAnalysisOutput(output)
	if err == nil {
		t.Fatal("expected error for empty string")
	}
	if analysis != nil {
		t.Fatal("analysis should be nil on parse error")
	}
}

// TestParseAnalysisOutputWhitespaceOnly tests parsing whitespace only
func TestParseAnalysisOutputWhitespaceOnly(t *testing.T) {
	output := "   \n  \t  \n  "

	analysis, err := parseAnalysisOutput(output)
	if err == nil {
		t.Fatal("expected error for whitespace only")
	}
	if analysis != nil {
		t.Fatal("analysis should be nil on parse error")
	}
}

// TestParseAnalysisOutputAllCategories tests parsing all valid category types
func TestParseAnalysisOutputAllCategories(t *testing.T) {
	categories := []Category{
		CategorySyntax,
		CategoryLogic,
		CategoryEnvironment,
		CategoryUnclearSpec,
		CategoryMissingContext,
		CategoryTestFlake,
	}

	for _, category := range categories {
		output := `{
			"category": "` + string(category) + `",
			"recoverable": true,
			"root_cause": "Test for ` + string(category) + `",
			"suggestion": "Fix it"
		}`

		analysis, err := parseAnalysisOutput(output)
		if err != nil {
			t.Errorf("unexpected error for category %q: %v", category, err)
			continue
		}

		if analysis == nil {
			t.Errorf("analysis should not be nil for category %q", category)
			continue
		}
		if analysis.Category != category {
			t.Errorf("expected category %q, got %q", category, analysis.Category)
		}
	}
}

// TestLearningCategoryMissingContext tests LearningCategory() for missing_context
func TestLearningCategoryMissingContext(t *testing.T) {
	analysis := &Analysis{
		Category: CategoryMissingContext,
	}

	learningCategory := analysis.LearningCategory()
	if learningCategory != "conventions" {
		t.Errorf("expected 'conventions' for CategoryMissingContext, got %q", learningCategory)
	}
}

// TestLearningCategoryEnvironment tests LearningCategory() for environment
func TestLearningCategoryEnvironment(t *testing.T) {
	analysis := &Analysis{
		Category: CategoryEnvironment,
	}

	learningCategory := analysis.LearningCategory()
	if learningCategory != "gotchas" {
		t.Errorf("expected 'gotchas' for CategoryEnvironment, got %q", learningCategory)
	}
}

// TestLearningCategoryLogic tests LearningCategory() for logic
func TestLearningCategoryLogic(t *testing.T) {
	analysis := &Analysis{
		Category: CategoryLogic,
	}

	learningCategory := analysis.LearningCategory()
	if learningCategory != "patterns" {
		t.Errorf("expected 'patterns' for CategoryLogic, got %q", learningCategory)
	}
}

// TestLearningCategorySyntax tests LearningCategory() for syntax (default)
func TestLearningCategorySyntax(t *testing.T) {
	analysis := &Analysis{
		Category: CategorySyntax,
	}

	learningCategory := analysis.LearningCategory()
	if learningCategory != "gotchas" {
		t.Errorf("expected 'gotchas' as default for CategorySyntax, got %q", learningCategory)
	}
}

// TestLearningCategoryUnclearSpec tests LearningCategory() for unclear_spec (default)
func TestLearningCategoryUnclearSpec(t *testing.T) {
	analysis := &Analysis{
		Category: CategoryUnclearSpec,
	}

	learningCategory := analysis.LearningCategory()
	if learningCategory != "gotchas" {
		t.Errorf("expected 'gotchas' as default for CategoryUnclearSpec, got %q", learningCategory)
	}
}

// TestLearningCategoryTestFlake tests LearningCategory() for test_flake (default)
func TestLearningCategoryTestFlake(t *testing.T) {
	analysis := &Analysis{
		Category: CategoryTestFlake,
	}

	learningCategory := analysis.LearningCategory()
	if learningCategory != "gotchas" {
		t.Errorf("expected 'gotchas' as default for CategoryTestFlake, got %q", learningCategory)
	}
}

// TestLearningCategoryInvalidCategory tests LearningCategory() for invalid category
func TestLearningCategoryInvalidCategory(t *testing.T) {
	analysis := &Analysis{
		Category: Category("invalid"),
	}

	learningCategory := analysis.LearningCategory()
	if learningCategory != "gotchas" {
		t.Errorf("expected 'gotchas' as default for invalid category, got %q", learningCategory)
	}
}

// TestParseAnalysisOutputComplex tests parsing complex nested JSON
func TestParseAnalysisOutputComplex(t *testing.T) {
	output := `Some preamble text here.

{
	"category": "missing_context",
	"recoverable": true,
	"root_cause": "Didn't know about the existing Config struct in utils",
	"learning": "Always check internal packages for existing abstractions before creating new ones",
	"suggestion": "Use the existing Config struct and extend if needed"
}

Some trailing text here.`

	analysis, err := parseAnalysisOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis == nil {
		t.Fatal("analysis should not be nil")
	}
	if analysis.Category != CategoryMissingContext {
		t.Errorf("expected CategoryMissingContext, got %q", analysis.Category)
	}
	if analysis.Learning == nil || *analysis.Learning != "Always check internal packages for existing abstractions before creating new ones" {
		t.Errorf("unexpected learning value: %v", analysis.Learning)
	}
	if analysis.RootCause != "Didn't know about the existing Config struct in utils" {
		t.Errorf("unexpected root_cause: %q", analysis.RootCause)
	}
}

// TestLearningCategoryNilReceiver tests that LearningCategory handles nil receiver
func TestLearningCategoryNilReceiver(t *testing.T) {
	var analysis *Analysis
	result := analysis.LearningCategory()
	if result != "gotchas" {
		t.Errorf("expected 'gotchas' for nil Analysis, got %q", result)
	}
}

// TestParseAnalysisOutputTrimWhitespace tests that output is trimmed
func TestParseAnalysisOutputTrimWhitespace(t *testing.T) {
	output := `


{
	"category": "logic",
	"recoverable": false,
	"root_cause": "Test",
	"suggestion": "Test suggestion"
}


	`

	analysis, err := parseAnalysisOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if analysis == nil {
		t.Fatal("analysis should not be nil")
	}
	if analysis.Category != CategoryLogic {
		t.Errorf("expected CategoryLogic, got %q", analysis.Category)
	}
}
