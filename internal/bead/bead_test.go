package bead

import (
	"strings"
	"testing"
)

func TestValidate_ValidBead(t *testing.T) {
	b := &Bead{
		ID:          "abc-123",
		Title:       "Implement feature X",
		Description: "Build the thing\nwith multiple lines",
		Priority:    1,
		Labels:      []string{"complexity:high", "spec:auth"},
		Parent:      "parent-456",
	}

	if err := b.Validate(); err != nil {
		t.Errorf("valid bead should not fail validation: %v", err)
	}
}

func TestValidate_EmptyID(t *testing.T) {
	b := &Bead{ID: "", Title: "Test"}
	err := b.Validate()
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if !strings.Contains(err.Error(), "empty ID") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_IDWithShellMetachars(t *testing.T) {
	badIDs := []string{
		"id; rm -rf /",
		"id$(whoami)",
		"id`whoami`",
		"id | cat /etc/passwd",
		"id && echo pwned",
		"../../../etc/passwd",
		"id with spaces",
		"id\nnewline",
		"id\x00null",
	}

	for _, id := range badIDs {
		b := &Bead{ID: id, Title: "Test"}
		if err := b.Validate(); err == nil {
			t.Errorf("expected error for ID %q, got nil", id)
		}
	}
}

func TestValidate_IDTooLong(t *testing.T) {
	b := &Bead{ID: strings.Repeat("a", maxIDLength+1), Title: "Test"}
	err := b.Validate()
	if err == nil {
		t.Fatal("expected error for oversized ID")
	}
	if !strings.Contains(err.Error(), "max length") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_ValidIDs(t *testing.T) {
	validIDs := []string{
		"abc-123",
		"ABC_DEF",
		"simple",
		"a",
		"123",
		"task-with-many-hyphens",
		"UPPER_CASE_ID",
	}

	for _, id := range validIDs {
		b := &Bead{ID: id, Title: "Test"}
		if err := b.Validate(); err != nil {
			t.Errorf("valid ID %q should not fail: %v", id, err)
		}
	}
}

func TestValidate_TitleTooLong(t *testing.T) {
	b := &Bead{
		ID:    "test-1",
		Title: strings.Repeat("x", maxTitleLength+1),
	}
	err := b.Validate()
	if err == nil {
		t.Fatal("expected error for oversized title")
	}
	if !strings.Contains(err.Error(), "title") && !strings.Contains(err.Error(), "max length") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_DescriptionTooLong(t *testing.T) {
	b := &Bead{
		ID:          "test-1",
		Title:       "Test",
		Description: strings.Repeat("x", maxDescriptionLength+1),
	}
	err := b.Validate()
	if err == nil {
		t.Fatal("expected error for oversized description")
	}
	if !strings.Contains(err.Error(), "description") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_ControlCharsInTitle(t *testing.T) {
	b := &Bead{
		ID:    "test-1",
		Title: "Title with \x00 null byte",
	}
	err := b.Validate()
	if err == nil {
		t.Fatal("expected error for control char in title")
	}
	if !strings.Contains(err.Error(), "control character") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_ControlCharsInDescription(t *testing.T) {
	b := &Bead{
		ID:          "test-1",
		Title:       "Test",
		Description: "Desc with \x07 bell",
	}
	err := b.Validate()
	if err == nil {
		t.Fatal("expected error for control char in description")
	}
}

func TestValidate_AllowedWhitespaceInDescription(t *testing.T) {
	b := &Bead{
		ID:          "test-1",
		Title:       "Test",
		Description: "Line 1\nLine 2\r\nLine 3\tTabbed",
	}
	if err := b.Validate(); err != nil {
		t.Errorf("newlines and tabs should be allowed in description: %v", err)
	}
}

func TestValidate_LabelTooLong(t *testing.T) {
	b := &Bead{
		ID:     "test-1",
		Title:  "Test",
		Labels: []string{strings.Repeat("x", maxLabelLength+1)},
	}
	err := b.Validate()
	if err == nil {
		t.Fatal("expected error for oversized label")
	}
	if !strings.Contains(err.Error(), "label") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_TooManyLabels(t *testing.T) {
	labels := make([]string, maxLabelCount+1)
	for i := range labels {
		labels[i] = "label"
	}
	b := &Bead{
		ID:     "test-1",
		Title:  "Test",
		Labels: labels,
	}
	err := b.Validate()
	if err == nil {
		t.Fatal("expected error for too many labels")
	}
	if !strings.Contains(err.Error(), "too many labels") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_ControlCharsInLabel(t *testing.T) {
	b := &Bead{
		ID:     "test-1",
		Title:  "Test",
		Labels: []string{"good-label", "bad\x00label"},
	}
	err := b.Validate()
	if err == nil {
		t.Fatal("expected error for control char in label")
	}
}

func TestValidate_InvalidParentID(t *testing.T) {
	b := &Bead{
		ID:     "test-1",
		Title:  "Test",
		Parent: "parent; rm -rf /",
	}
	err := b.Validate()
	if err == nil {
		t.Fatal("expected error for invalid parent ID")
	}
	if !strings.Contains(err.Error(), "parent ID") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_EmptyParentIsOK(t *testing.T) {
	b := &Bead{
		ID:    "test-1",
		Title: "Test",
	}
	if err := b.Validate(); err != nil {
		t.Errorf("empty parent should be OK: %v", err)
	}
}

func TestRejectControlChars(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"clean text", "Hello world", false},
		{"newlines allowed", "Line 1\nLine 2", false},
		{"tabs allowed", "Col1\tCol2", false},
		{"carriage return allowed", "Line\r\n", false},
		{"null byte", "before\x00after", true},
		{"bell", "ring\x07ring", true},
		{"escape", "esc\x1b[31m", true},
		{"backspace", "back\x08space", true},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rejectControlChars(tt.input, "test")
			if (err != nil) != tt.wantErr {
				t.Errorf("rejectControlChars(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
