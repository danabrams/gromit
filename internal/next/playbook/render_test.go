package playbook

import (
	"strings"
	"testing"
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

// Tests for RenderPlaybookSection - verifies filtering to active entries only

func TestRenderPlaybookSection_EmptySlice(t *testing.T) {
	result := RenderPlaybookSection([]Entry{})

	if result != "" {
		t.Errorf("RenderPlaybookSection empty slice: got %q, want empty string", result)
	}
}

func TestRenderPlaybookSection_NoActiveEntries(t *testing.T) {
	entries := []Entry{
		{
			ID:      "pb-11111111",
			Type:    "pattern",
			Title:   "Inactive Pattern",
			Content: "This is inactive",
			Status:  "archived",
		},
		{
			ID:           "pb-22222222",
			Type:         "insight",
			Title:        "Superseded Insight",
			Content:      "This is superseded",
			Status:       "active",
			SupersededBy: "pb-33333333",
		},
	}

	result := RenderPlaybookSection(entries)

	if result != "" {
		t.Errorf("RenderPlaybookSection with no active entries: got %q, want empty string", result)
	}
}

func TestRenderPlaybookSection_FiltersInactiveEntries(t *testing.T) {
	entries := []Entry{
		{
			ID:      "pb-11111111",
			Type:    "pattern",
			Title:   "Active Pattern",
			Content: "This is active",
			Status:  "active",
		},
		{
			ID:      "pb-22222222",
			Type:    "insight",
			Title:   "Archived Insight",
			Content: "This is archived",
			Status:  "archived",
		},
		{
			ID:      "pb-33333333",
			Type:    "pattern",
			Title:   "Another Active",
			Content: "Also active",
			Status:  "active",
		},
	}

	result := RenderPlaybookSection(entries)

	// Should include active entries
	if !strings.Contains(result, "Active Pattern") {
		t.Errorf("Active entry not included: %q", result)
	}
	if !strings.Contains(result, "Another Active") {
		t.Errorf("Second active entry not included: %q", result)
	}
	// Should exclude archived entries
	if strings.Contains(result, "Archived Insight") {
		t.Errorf("Archived entry was included but should be filtered: %q", result)
	}
}

func TestRenderPlaybookSection_FiltersSupersededEntries(t *testing.T) {
	entries := []Entry{
		{
			ID:      "pb-11111111",
			Type:    "pattern",
			Title:   "Active Pattern",
			Content: "This is active",
			Status:  "active",
		},
		{
			ID:           "pb-22222222",
			Type:         "pattern",
			Title:        "Superseded Pattern",
			Content:      "This is superseded",
			Status:       "active",
			SupersededBy: "pb-11111111",
		},
	}

	result := RenderPlaybookSection(entries)

	// Should include non-superseded active entry
	if !strings.Contains(result, "Active Pattern") {
		t.Errorf("Active non-superseded entry not included: %q", result)
	}
	// Should exclude superseded entry even if status is active
	if strings.Contains(result, "Superseded Pattern") {
		t.Errorf("Superseded entry was included but should be filtered: %q", result)
	}
}

func TestRenderPlaybookSection_SingleActiveEntry(t *testing.T) {
	entries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "Test Pattern",
			Content:   "This is test content",
			Rationale: "Why this matters",
			Status:    "active",
		},
	}

	result := RenderPlaybookSection(entries)

	expected := "- **Test Pattern**: This is test content\n  Rationale: Why this matters"
	if result != expected {
		t.Errorf("Single active entry:\ngot:  %q\nwant: %q", result, expected)
	}
}

func TestRenderPlaybookSection_MultipleActiveEntries(t *testing.T) {
	entries := []Entry{
		{
			ID:        "pb-11111111",
			Type:      "pattern",
			Title:     "First Pattern",
			Content:   "First content",
			Rationale: "First reason",
			Status:    "active",
		},
		{
			ID:      "pb-22222222",
			Type:    "insight",
			Title:   "Inactive Insight",
			Content: "This is inactive",
			Status:  "archived",
		},
		{
			ID:        "pb-33333333",
			Type:      "pattern",
			Title:     "Second Pattern",
			Content:   "Second content",
			Rationale: "Second reason",
			Status:    "active",
		},
	}

	result := RenderPlaybookSection(entries)

	expected := "- **First Pattern**: First content\n  Rationale: First reason\n\n- **Second Pattern**: Second content\n  Rationale: Second reason"
	if result != expected {
		t.Errorf("Multiple entries with filtering:\ngot:  %q\nwant: %q", result, expected)
	}
}
