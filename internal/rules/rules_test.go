package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBasicRules(t *testing.T) {
	content := `# Rules

These are non-negotiable constraints for gromit development.

## Code Style

- This is a Go project - use idiomatic Go patterns
- Use descriptive variable names

## Architecture

- CLI commands go in cmd/gromit/
- Internal packages go in internal/
`

	r, err := Parse(content)
	if err != nil {
		t.Fatalf("parsing rules: %v", err)
	}

	if len(r.Sections) != 2 {
		t.Errorf("expected 2 sections, got %d", len(r.Sections))
	}

	// Check first section
	if r.Sections[0].Name != "Code Style" {
		t.Errorf("expected section name 'Code Style', got %q", r.Sections[0].Name)
	}
	if len(r.Sections[0].Rules) != 2 {
		t.Errorf("expected 2 rules in Code Style, got %d", len(r.Sections[0].Rules))
	}
	if r.Sections[0].Rules[0] != "This is a Go project - use idiomatic Go patterns" {
		t.Errorf("unexpected rule: %q", r.Sections[0].Rules[0])
	}

	// Check second section
	if r.Sections[1].Name != "Architecture" {
		t.Errorf("expected section name 'Architecture', got %q", r.Sections[1].Name)
	}
	if len(r.Sections[1].Rules) != 2 {
		t.Errorf("expected 2 rules in Architecture, got %d", len(r.Sections[1].Rules))
	}
}

func TestParseEmptyContent(t *testing.T) {
	r, err := Parse("")
	if err != nil {
		t.Fatalf("parsing empty content: %v", err)
	}

	if len(r.Sections) != 0 {
		t.Errorf("expected 0 sections for empty content, got %d", len(r.Sections))
	}
}

func TestParseEmptyContentSectionsNotNil(t *testing.T) {
	r, err := Parse("")
	if err != nil {
		t.Fatalf("parsing empty content: %v", err)
	}

	if r.Sections == nil {
		t.Error("expected Sections to be non-nil (empty slice) for empty content")
	}
}

func TestParseWhitespaceContentSectionsNotNil(t *testing.T) {
	r, err := Parse("  \n\n  \t\n")
	if err != nil {
		t.Fatalf("parsing whitespace content: %v", err)
	}

	if r.Sections == nil {
		t.Error("expected Sections to be non-nil (empty slice) for whitespace content")
	}
}

func TestParseSectionWithoutRules(t *testing.T) {
	content := `## Empty Section

## Another Section
- This one has a rule
`

	r, err := Parse(content)
	if err != nil {
		t.Fatalf("parsing rules: %v", err)
	}

	if len(r.Sections) != 2 {
		t.Errorf("expected 2 sections, got %d", len(r.Sections))
	}

	if len(r.Sections[0].Rules) != 0 {
		t.Errorf("expected 0 rules in first section, got %d", len(r.Sections[0].Rules))
	}

	if len(r.Sections[1].Rules) != 1 {
		t.Errorf("expected 1 rule in second section, got %d", len(r.Sections[1].Rules))
	}
}

func TestParseRulesWithoutSection(t *testing.T) {
	// Rules without a section header should be ignored
	content := `- Orphan rule 1
- Orphan rule 2

## Valid Section
- Valid rule
`

	r, err := Parse(content)
	if err != nil {
		t.Fatalf("parsing rules: %v", err)
	}

	if len(r.Sections) != 1 {
		t.Errorf("expected 1 section (orphan rules ignored), got %d", len(r.Sections))
	}

	if r.Sections[0].Name != "Valid Section" {
		t.Errorf("expected section name 'Valid Section', got %q", r.Sections[0].Name)
	}
}

func TestLoadAndSave(t *testing.T) {
	dir := t.TempDir()
	originalPath := filepath.Join(dir, "original.md")
	savedPath := filepath.Join(dir, "saved.md")

	// Create original file
	originalContent := `# Rules

These are non-negotiable constraints for gromit development.

## Code Style

- Rule 1
- Rule 2

## Architecture

- Rule 3
`

	if err := os.WriteFile(originalPath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("writing original file: %v", err)
	}

	// Load the file
	r, err := Load(originalPath)
	if err != nil {
		t.Fatalf("loading rules: %v", err)
	}

	// Save to new location
	if err := r.Save(savedPath); err != nil {
		t.Fatalf("saving rules: %v", err)
	}

	// Load the saved file
	r2, err := Load(savedPath)
	if err != nil {
		t.Fatalf("loading saved rules: %v", err)
	}

	// Compare
	if len(r2.Sections) != len(r.Sections) {
		t.Errorf("expected %d sections, got %d", len(r.Sections), len(r2.Sections))
	}

	for i := range r.Sections {
		if r2.Sections[i].Name != r.Sections[i].Name {
			t.Errorf("section %d: expected name %q, got %q", i, r.Sections[i].Name, r2.Sections[i].Name)
		}
		if len(r2.Sections[i].Rules) != len(r.Sections[i].Rules) {
			t.Errorf("section %d: expected %d rules, got %d", i, len(r.Sections[i].Rules), len(r2.Sections[i].Rules))
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/rules.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "reading rules file") {
		t.Errorf("expected 'reading rules file' in error, got: %v", err)
	}
}

func TestAddRuleToExistingSection(t *testing.T) {
	r := &Rules{
		Sections: []Section{
			{Name: "Code Style", Rules: []string{"Rule 1"}},
		},
	}

	r.AddRule("Code Style", "Rule 2")

	if len(r.Sections) != 1 {
		t.Errorf("expected 1 section, got %d", len(r.Sections))
	}

	if len(r.Sections[0].Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(r.Sections[0].Rules))
	}

	if r.Sections[0].Rules[1] != "Rule 2" {
		t.Errorf("expected rule 'Rule 2', got %q", r.Sections[0].Rules[1])
	}
}

func TestAddRuleToNewSection(t *testing.T) {
	r := &Rules{
		Sections: []Section{
			{Name: "Code Style", Rules: []string{"Rule 1"}},
		},
	}

	r.AddRule("Architecture", "New rule")

	if len(r.Sections) != 2 {
		t.Errorf("expected 2 sections, got %d", len(r.Sections))
	}

	if r.Sections[1].Name != "Architecture" {
		t.Errorf("expected section name 'Architecture', got %q", r.Sections[1].Name)
	}

	if len(r.Sections[1].Rules) != 1 {
		t.Errorf("expected 1 rule in new section, got %d", len(r.Sections[1].Rules))
	}

	if r.Sections[1].Rules[0] != "New rule" {
		t.Errorf("expected rule 'New rule', got %q", r.Sections[1].Rules[0])
	}
}

func TestAddRuleToEmptyRules(t *testing.T) {
	r := &Rules{}

	r.AddRule("First Section", "First rule")

	if len(r.Sections) != 1 {
		t.Errorf("expected 1 section, got %d", len(r.Sections))
	}

	if r.Sections[0].Name != "First Section" {
		t.Errorf("expected section name 'First Section', got %q", r.Sections[0].Name)
	}

	if len(r.Sections[0].Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(r.Sections[0].Rules))
	}
}

func TestModifyRuleSuccess(t *testing.T) {
	r := &Rules{
		Sections: []Section{
			{
				Name:  "Code Style",
				Rules: []string{"Old rule", "Another rule"},
			},
		},
	}

	err := r.ModifyRule("Code Style", "Old rule", "New rule")
	if err != nil {
		t.Fatalf("modifying rule: %v", err)
	}

	if r.Sections[0].Rules[0] != "New rule" {
		t.Errorf("expected 'New rule', got %q", r.Sections[0].Rules[0])
	}

	if r.Sections[0].Rules[1] != "Another rule" {
		t.Errorf("expected second rule unchanged, got %q", r.Sections[0].Rules[1])
	}
}

func TestModifyRuleSectionNotFound(t *testing.T) {
	r := &Rules{
		Sections: []Section{
			{Name: "Code Style", Rules: []string{"Rule 1"}},
		},
	}

	err := r.ModifyRule("Nonexistent", "Rule 1", "New rule")
	if err == nil {
		t.Error("expected error for nonexistent section")
	}

	if !strings.Contains(err.Error(), "section not found") {
		t.Errorf("expected 'section not found' in error, got: %v", err)
	}
}

func TestModifyRuleNotFound(t *testing.T) {
	r := &Rules{
		Sections: []Section{
			{Name: "Code Style", Rules: []string{"Rule 1"}},
		},
	}

	err := r.ModifyRule("Code Style", "Nonexistent rule", "New rule")
	if err == nil {
		t.Error("expected error for nonexistent rule")
	}

	if !strings.Contains(err.Error(), "rule not found") {
		t.Errorf("expected 'rule not found' in error, got: %v", err)
	}
}

func TestGetSectionSuccess(t *testing.T) {
	r := &Rules{
		Sections: []Section{
			{Name: "Code Style", Rules: []string{"Rule 1"}},
			{Name: "Architecture", Rules: []string{"Rule 2"}},
		},
	}

	section := r.GetSection("Architecture")
	if section == nil {
		t.Fatal("expected section, got nil")
	}

	if section.Name != "Architecture" {
		t.Errorf("expected 'Architecture', got %q", section.Name)
	}

	if len(section.Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(section.Rules))
	}
}

func TestGetSectionNotFound(t *testing.T) {
	r := &Rules{
		Sections: []Section{
			{Name: "Code Style", Rules: []string{"Rule 1"}},
		},
	}

	section := r.GetSection("Nonexistent")
	if section != nil {
		t.Errorf("expected nil for nonexistent section, got %v", section)
	}
}

func TestRemoveRuleSuccess(t *testing.T) {
	r := &Rules{
		Sections: []Section{
			{
				Name:  "Code Style",
				Rules: []string{"Rule 1", "Rule 2", "Rule 3"},
			},
		},
	}

	err := r.RemoveRule("Code Style", "Rule 2")
	if err != nil {
		t.Fatalf("removing rule: %v", err)
	}

	if len(r.Sections[0].Rules) != 2 {
		t.Errorf("expected 2 rules after removal, got %d", len(r.Sections[0].Rules))
	}

	if r.Sections[0].Rules[0] != "Rule 1" {
		t.Errorf("expected 'Rule 1' at index 0, got %q", r.Sections[0].Rules[0])
	}

	if r.Sections[0].Rules[1] != "Rule 3" {
		t.Errorf("expected 'Rule 3' at index 1, got %q", r.Sections[0].Rules[1])
	}
}

func TestRemoveRuleSectionNotFound(t *testing.T) {
	r := &Rules{
		Sections: []Section{
			{Name: "Code Style", Rules: []string{"Rule 1"}},
		},
	}

	err := r.RemoveRule("Nonexistent", "Rule 1")
	if err == nil {
		t.Error("expected error for nonexistent section")
	}

	if !strings.Contains(err.Error(), "section not found") {
		t.Errorf("expected 'section not found' in error, got: %v", err)
	}
}

func TestRemoveRuleNotFound(t *testing.T) {
	r := &Rules{
		Sections: []Section{
			{Name: "Code Style", Rules: []string{"Rule 1"}},
		},
	}

	err := r.RemoveRule("Code Style", "Nonexistent rule")
	if err == nil {
		t.Error("expected error for nonexistent rule")
	}

	if !strings.Contains(err.Error(), "rule not found") {
		t.Errorf("expected 'rule not found' in error, got: %v", err)
	}
}

func TestSavePreservesOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.md")

	r := &Rules{
		Sections: []Section{
			{Name: "First", Rules: []string{"Rule A", "Rule B"}},
			{Name: "Second", Rules: []string{"Rule C"}},
			{Name: "Third", Rules: []string{"Rule D", "Rule E", "Rule F"}},
		},
	}

	if err := r.Save(path); err != nil {
		t.Fatalf("saving rules: %v", err)
	}

	r2, err := Load(path)
	if err != nil {
		t.Fatalf("loading saved rules: %v", err)
	}

	if len(r2.Sections) != 3 {
		t.Errorf("expected 3 sections, got %d", len(r2.Sections))
	}

	// Check order
	if r2.Sections[0].Name != "First" {
		t.Errorf("expected first section 'First', got %q", r2.Sections[0].Name)
	}
	if r2.Sections[1].Name != "Second" {
		t.Errorf("expected second section 'Second', got %q", r2.Sections[1].Name)
	}
	if r2.Sections[2].Name != "Third" {
		t.Errorf("expected third section 'Third', got %q", r2.Sections[2].Name)
	}

	// Check rule order in third section
	if len(r2.Sections[2].Rules) != 3 {
		t.Errorf("expected 3 rules in third section, got %d", len(r2.Sections[2].Rules))
	}
	if r2.Sections[2].Rules[0] != "Rule D" {
		t.Errorf("expected 'Rule D', got %q", r2.Sections[2].Rules[0])
	}
	if r2.Sections[2].Rules[1] != "Rule E" {
		t.Errorf("expected 'Rule E', got %q", r2.Sections[2].Rules[1])
	}
	if r2.Sections[2].Rules[2] != "Rule F" {
		t.Errorf("expected 'Rule F', got %q", r2.Sections[2].Rules[2])
	}
}

func TestParseIgnoresNonRuleContent(t *testing.T) {
	content := `# Title

Some introductory text.

## Section One

More text here.

- Actual rule

Some text between rules.

- Another rule

## Section Two

Random content.

- Rule in section two
`

	r, err := Parse(content)
	if err != nil {
		t.Fatalf("parsing rules: %v", err)
	}

	if len(r.Sections) != 2 {
		t.Errorf("expected 2 sections, got %d", len(r.Sections))
	}

	if len(r.Sections[0].Rules) != 2 {
		t.Errorf("expected 2 rules in section 1, got %d", len(r.Sections[0].Rules))
	}

	if len(r.Sections[1].Rules) != 1 {
		t.Errorf("expected 1 rule in section 2, got %d", len(r.Sections[1].Rules))
	}
}

func TestParseRuleWithSpecialCharacters(t *testing.T) {
	content := "## Code Style\n\n" +
		"- Use `fmt.Errorf(\"context: %w\", err)` for error wrapping\n" +
		"- Rules can have \"quotes\" and 'apostrophes'\n" +
		"- Handle edge cases: [brackets], {braces}, (parentheses)\n"

	r, err := Parse(content)
	if err != nil {
		t.Fatalf("parsing rules: %v", err)
	}

	if len(r.Sections[0].Rules) != 3 {
		t.Errorf("expected 3 rules, got %d", len(r.Sections[0].Rules))
	}

	if !strings.Contains(r.Sections[0].Rules[0], "fmt.Errorf") {
		t.Errorf("expected backticks preserved, got: %q", r.Sections[0].Rules[0])
	}

	if !strings.Contains(r.Sections[0].Rules[1], "quotes") {
		t.Errorf("expected quotes preserved, got: %q", r.Sections[0].Rules[1])
	}

	if !strings.Contains(r.Sections[0].Rules[2], "[brackets]") {
		t.Errorf("expected brackets preserved, got: %q", r.Sections[0].Rules[2])
	}
}

func TestAddMultipleRulesToNewSection(t *testing.T) {
	r := &Rules{}

	r.AddRule("New Section", "Rule 1")
	r.AddRule("New Section", "Rule 2")
	r.AddRule("New Section", "Rule 3")

	if len(r.Sections) != 1 {
		t.Errorf("expected 1 section, got %d", len(r.Sections))
	}

	if len(r.Sections[0].Rules) != 3 {
		t.Errorf("expected 3 rules, got %d", len(r.Sections[0].Rules))
	}
}

func TestModifyRuleInMiddleOfList(t *testing.T) {
	r := &Rules{
		Sections: []Section{
			{
				Name:  "Test",
				Rules: []string{"Rule 1", "Rule 2", "Rule 3", "Rule 4"},
			},
		},
	}

	err := r.ModifyRule("Test", "Rule 2", "Modified Rule 2")
	if err != nil {
		t.Fatalf("modifying rule: %v", err)
	}

	if r.Sections[0].Rules[1] != "Modified Rule 2" {
		t.Errorf("expected 'Modified Rule 2', got %q", r.Sections[0].Rules[1])
	}

	// Ensure other rules unchanged
	if r.Sections[0].Rules[0] != "Rule 1" {
		t.Errorf("expected 'Rule 1' unchanged, got %q", r.Sections[0].Rules[0])
	}
	if r.Sections[0].Rules[2] != "Rule 3" {
		t.Errorf("expected 'Rule 3' unchanged, got %q", r.Sections[0].Rules[2])
	}
	if r.Sections[0].Rules[3] != "Rule 4" {
		t.Errorf("expected 'Rule 4' unchanged, got %q", r.Sections[0].Rules[3])
	}
}

func TestRemoveFirstRule(t *testing.T) {
	r := &Rules{
		Sections: []Section{
			{
				Name:  "Test",
				Rules: []string{"Rule 1", "Rule 2", "Rule 3"},
			},
		},
	}

	err := r.RemoveRule("Test", "Rule 1")
	if err != nil {
		t.Fatalf("removing rule: %v", err)
	}

	if len(r.Sections[0].Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(r.Sections[0].Rules))
	}

	if r.Sections[0].Rules[0] != "Rule 2" {
		t.Errorf("expected 'Rule 2' at index 0, got %q", r.Sections[0].Rules[0])
	}
}

func TestRemoveLastRule(t *testing.T) {
	r := &Rules{
		Sections: []Section{
			{
				Name:  "Test",
				Rules: []string{"Rule 1", "Rule 2", "Rule 3"},
			},
		},
	}

	err := r.RemoveRule("Test", "Rule 3")
	if err != nil {
		t.Fatalf("removing rule: %v", err)
	}

	if len(r.Sections[0].Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(r.Sections[0].Rules))
	}

	if r.Sections[0].Rules[1] != "Rule 2" {
		t.Errorf("expected 'Rule 2' at index 1, got %q", r.Sections[0].Rules[1])
	}
}

func TestEmptyRuleText(t *testing.T) {
	// Test that lines starting with "- " but not followed by "- " are ignored
	content := `## Section
- Valid rule 1
- Valid rule 2
`

	r, err := Parse(content)
	if err != nil {
		t.Fatalf("parsing rules: %v", err)
	}

	if len(r.Sections[0].Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(r.Sections[0].Rules))
	}

	if r.Sections[0].Rules[0] != "Valid rule 1" {
		t.Errorf("expected 'Valid rule 1', got %q", r.Sections[0].Rules[0])
	}

	if r.Sections[0].Rules[1] != "Valid rule 2" {
		t.Errorf("expected 'Valid rule 2', got %q", r.Sections[0].Rules[1])
	}
}

func TestLoadRealRulesFile(t *testing.T) {
	// Test loading the actual RULES.md file from the project
	r, err := Load("../../.gromit/RULES.md")
	if err != nil {
		t.Skipf("skipping test, RULES.md not found: %v", err)
	}

	// Verify we got some sections
	if len(r.Sections) == 0 {
		t.Error("expected at least one section in RULES.md")
	}

	// Verify sections have rules
	hasRules := false
	for _, section := range r.Sections {
		if len(section.Rules) > 0 {
			hasRules = true
			break
		}
	}

	if !hasRules {
		t.Error("expected at least one section to have rules")
	}

	// Test round-trip: save and reload
	dir := t.TempDir()
	testPath := filepath.Join(dir, "test-rules.md")

	if err := r.Save(testPath); err != nil {
		t.Fatalf("saving rules: %v", err)
	}

	r2, err := Load(testPath)
	if err != nil {
		t.Fatalf("reloading saved rules: %v", err)
	}

	if len(r2.Sections) != len(r.Sections) {
		t.Errorf("expected %d sections after reload, got %d", len(r.Sections), len(r2.Sections))
	}
}
