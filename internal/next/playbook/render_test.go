package playbook

import (
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/doctrine"
)

func TestFormatPlaybookForPrompt_EmptySlice(t *testing.T) {
	result := FormatPlaybookForPrompt([]Entry{})

	if result != "" {
		t.Errorf("FormatPlaybookForPrompt empty slice: got %q, want empty string", result)
	}
}

func TestFormatPlaybookForPrompt_SingleEntry_WithoutRationale(t *testing.T) {
	entries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Test Pattern",
			Content:   "This is test content",
			Rationale: "",
			Status:    "active",
		},
	}

	result := FormatPlaybookForPrompt(entries)

	expected := "- **Test Pattern**: This is test content"
	if result != expected {
		t.Errorf("Single entry without rationale:\ngot:  %q\nwant: %q", result, expected)
	}
}

func TestFormatPlaybookForPrompt_SingleEntry_WithRationale(t *testing.T) {
	entries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Test Pattern",
			Content:   "This is test content",
			Rationale: "This pattern improves code clarity",
			Status:    "active",
		},
	}

	result := FormatPlaybookForPrompt(entries)

	expected := "- **Test Pattern**: This is test content\n  Rationale: This pattern improves code clarity"
	if result != expected {
		t.Errorf("Single entry with rationale:\ngot:  %q\nwant: %q", result, expected)
	}
}

func TestFormatPlaybookForPrompt_MultipleEntries_NoRationale(t *testing.T) {
	entries := []Entry{
		{
			ID:      "pb-11111111",
			Type:    "pattern",
			Title:   "Pattern One",
			Content: "Content one",
		},
		{
			ID:      "pb-22222222",
			Type:    "pattern",
			Title:   "Pattern Two",
			Content: "Content two",
		},
		{
			ID:      "pb-33333333",
			Type:    "insight",
			Title:   "Insight Three",
			Content: "Content three",
		},
	}

	result := FormatPlaybookForPrompt(entries)

	expected := "- **Pattern One**: Content one\n\n- **Pattern Two**: Content two\n\n- **Insight Three**: Content three"
	if result != expected {
		t.Errorf("Multiple entries without rationale:\ngot:  %q\nwant: %q", result, expected)
	}
}

func TestFormatPlaybookForPrompt_MultipleEntries_WithRationale(t *testing.T) {
	entries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Pattern One",
			Content:   "Content one",
			Rationale: "Why pattern one",
		},
		{
			ID:        "pb-22222222",
			Type:      "insight",
			Title:     "Insight Two",
			Content:   "Content two",
			Rationale: "Why insight two",
		},
	}

	result := FormatPlaybookForPrompt(entries)

	expected := "- **Pattern One**: Content one\n  Rationale: Why pattern one\n\n- **Insight Two**: Content two\n  Rationale: Why insight two"
	if result != expected {
		t.Errorf("Multiple entries with rationale:\ngot:  %q\nwant: %q", result, expected)
	}
}

func TestFormatPlaybookForPrompt_MixedRationale(t *testing.T) {
	entries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Pattern One",
			Content:   "Content one",
			Rationale: "Has rationale",
		},
		{
			ID:        "pb-22222222",
			Type:      "pattern",
			Title:     "Pattern Two",
			Content:   "Content two",
			Rationale: "",
		},
		{
			ID:        "pb-33333333",
			Type:      "insight",
			Title:     "Insight Three",
			Content:   "Content three",
			Rationale: "Another rationale",
		},
	}

	result := FormatPlaybookForPrompt(entries)

	expected := "- **Pattern One**: Content one\n  Rationale: Has rationale\n\n- **Pattern Two**: Content two\n\n- **Insight Three**: Content three\n  Rationale: Another rationale"
	if result != expected {
		t.Errorf("Mixed rationale:\ngot:  %q\nwant: %q", result, expected)
	}
}

func TestFormatPlaybookForPrompt_SpecialCharacters(t *testing.T) {
	entries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Pattern **with** markdown",
			Content:   "Content with special: !@#$%^&*()",
			Rationale: "Rationale with 'quotes' and \"double quotes\"",
		},
	}

	result := FormatPlaybookForPrompt(entries)

	// Should preserve special characters exactly as input
	if !strings.Contains(result, "Pattern **with** markdown") {
		t.Errorf("Title special characters not preserved: %q", result)
	}
	if !strings.Contains(result, "!@#$%^&*()") {
		t.Errorf("Content special characters not preserved: %q", result)
	}
	if !strings.Contains(result, "'quotes'") {
		t.Errorf("Rationale special characters not preserved: %q", result)
	}
}

func TestFormatPlaybookForPrompt_MultilineContent(t *testing.T) {
	entries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Multiline Pattern",
			Content:   "Line one\nLine two\nLine three",
			Rationale: "Multiline\nrationale",
		},
	}

	result := FormatPlaybookForPrompt(entries)

	// Should preserve newlines in content and rationale
	if !strings.Contains(result, "Line one\nLine two\nLine three") {
		t.Errorf("Multiline content not preserved: %q", result)
	}
	if !strings.Contains(result, "Multiline\nrationale") {
		t.Errorf("Multiline rationale not preserved: %q", result)
	}
}

func TestFormatPlaybookForPrompt_NoTrailingNewline(t *testing.T) {
	entries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Test",
			Content:   "Content",
			Rationale: "Rationale",
		},
		{
			ID:        "pb-22222222",
			Type:      "pattern",
			Title:     "Test 2",
			Content:   "Content 2",
			Rationale: "",
		},
	}

	result := FormatPlaybookForPrompt(entries)

	// Should not end with newline
	if strings.HasSuffix(result, "\n") {
		t.Errorf("Result ends with newline, should not: %q", result)
	}
}

func TestFormatDoctrineForPrompt_EmptySlice(t *testing.T) {
	result := FormatDoctrineForPrompt([]doctrine.Rule{})

	if result != "" {
		t.Errorf("FormatDoctrineForPrompt empty slice: got %q, want empty string", result)
	}
}

func TestFormatDoctrineForPrompt_SingleRule(t *testing.T) {
	rules := []doctrine.Rule{
		{
			ID:        "rule-001",
			Summary:   "Use explicit error handling",
			Scope:     "code",
			Source:    "declared",
			CreatedAt: time.Now(),
			Status:    "active",
		},
	}

	result := FormatDoctrineForPrompt(rules)

	expected := "- **Use explicit error handling** (scope: code)"
	if result != expected {
		t.Errorf("Single rule:\ngot:  %q\nwant: %q", result, expected)
	}
}

func TestFormatDoctrineForPrompt_MultipleRules(t *testing.T) {
	rules := []doctrine.Rule{
		{
			ID:        "rule-001",
			Summary:   "Use explicit error handling",
			Scope:     "code",
			Status:    "active",
		},
		{
			ID:        "rule-002",
			Summary:   "Write tests for all public functions",
			Scope:     "tests",
			Status:    "active",
		},
		{
			ID:        "rule-003",
			Summary:   "Document all API endpoints",
			Scope:     "*",
			Status:    "active",
		},
	}

	result := FormatDoctrineForPrompt(rules)

	// Check all rules are present
	if !strings.Contains(result, "- **Use explicit error handling** (scope: code)") {
		t.Errorf("Rule 1 not found in result: %q", result)
	}
	if !strings.Contains(result, "- **Write tests for all public functions** (scope: tests)") {
		t.Errorf("Rule 2 not found in result: %q", result)
	}
	if !strings.Contains(result, "- **Document all API endpoints** (scope: *)") {
		t.Errorf("Rule 3 not found in result: %q", result)
	}

	// Check proper formatting with newlines
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %q", len(lines), result)
	}
}

func TestFormatDoctrineForPrompt_SpecialCharactersInSummary(t *testing.T) {
	rules := []doctrine.Rule{
		{
			ID:      "rule-001",
			Summary: "Use **bold** and `code` in summaries",
			Scope:   "code",
			Status:  "active",
		},
	}

	result := FormatDoctrineForPrompt(rules)

	// Should preserve special characters
	if !strings.Contains(result, "Use **bold** and `code` in summaries") {
		t.Errorf("Special characters in summary not preserved: %q", result)
	}
}

func TestFormatDoctrineForPrompt_SpecialCharactersInScope(t *testing.T) {
	rules := []doctrine.Rule{
		{
			ID:      "rule-001",
			Summary: "Test rule",
			Scope:   "tests/*",
			Status:  "active",
		},
	}

	result := FormatDoctrineForPrompt(rules)

	expected := "- **Test rule** (scope: tests/*)"
	if result != expected {
		t.Errorf("Special characters in scope:\ngot:  %q\nwant: %q", result, expected)
	}
}

func TestFormatDoctrineForPrompt_NoTrailingNewline(t *testing.T) {
	rules := []doctrine.Rule{
		{
			ID:      "rule-001",
			Summary: "Rule one",
			Scope:   "code",
		},
		{
			ID:      "rule-002",
			Summary: "Rule two",
			Scope:   "tests",
		},
	}

	result := FormatDoctrineForPrompt(rules)

	// Should not end with newline
	if strings.HasSuffix(result, "\n") {
		t.Errorf("Result ends with newline, should not: %q", result)
	}
}

func TestFormatDoctrineForPrompt_LongSummary(t *testing.T) {
	longSummary := "This is a very long summary that explains a complex rule about error handling and validation in great detail"
	rules := []doctrine.Rule{
		{
			ID:      "rule-001",
			Summary: longSummary,
			Scope:   "code",
		},
	}

	result := FormatDoctrineForPrompt(rules)

	if !strings.Contains(result, longSummary) {
		t.Errorf("Long summary not preserved: %q", result)
	}
}
