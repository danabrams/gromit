package promptrender

import (
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/doctrine"
)

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
			ID:      "rule-001",
			Summary: "Use explicit error handling",
			Scope:   "code",
			Status:  "active",
		},
		{
			ID:      "rule-002",
			Summary: "Write tests for all public functions",
			Scope:   "tests",
			Status:  "active",
		},
		{
			ID:      "rule-003",
			Summary: "Document all API endpoints",
			Scope:   "*",
			Status:  "active",
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
