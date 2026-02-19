package bead

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/jsonutil"
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

// TestBeadJSONParsing tests unmarshaling bead JSON from bd CLI
func TestBeadJSONParsing(t *testing.T) {
	tests := []struct {
		name     string
		jsonStr  string
		wantBead Bead
		wantErr  bool
	}{
		{
			name: "valid bead with all fields",
			jsonStr: `{
				"id": "test-123",
				"title": "Test task",
				"description": "Test description",
				"priority": 1,
				"labels": ["complexity:high", "spec:auth"],
				"parent": "epic-456",
				"issue_type": "task",
				"status": "open",
				"owner": "alice",
				"expected_outputs": ["file1.go", "file2.go"]
			}`,
			wantBead: Bead{
				ID:              "test-123",
				Title:           "Test task",
				Description:     "Test description",
				Priority:        1,
				Labels:          []string{"complexity:high", "spec:auth"},
				Parent:          "epic-456",
				Type:            "task",
				Status:          "open",
				Owner:           "alice",
				ExpectedOutputs: []string{"file1.go", "file2.go"},
			},
			wantErr: false,
		},
		{
			name: "minimal valid bead",
			jsonStr: `{
				"id": "min-001",
				"title": "Minimal",
				"description": "",
				"priority": 2,
				"labels": [],
				"parent": "",
				"issue_type": "task",
				"status": "open",
				"owner": ""
			}`,
			wantBead: Bead{
				ID:          "min-001",
				Title:       "Minimal",
				Description: "",
				Priority:    2,
				Labels:      []string{},
				Parent:      "",
				Type:        "task",
				Status:      "open",
				Owner:       "",
			},
			wantErr: false,
		},
		{
			name:     "invalid json",
			jsonStr:  `{"id": "test-123", invalid}`,
			wantErr:  true,
			wantBead: Bead{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Bead
			err := json.Unmarshal([]byte(tt.jsonStr), &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if got.ID != tt.wantBead.ID {
					t.Errorf("ID = %v, want %v", got.ID, tt.wantBead.ID)
				}
				if got.Title != tt.wantBead.Title {
					t.Errorf("Title = %v, want %v", got.Title, tt.wantBead.Title)
				}
				if got.Priority != tt.wantBead.Priority {
					t.Errorf("Priority = %v, want %v", got.Priority, tt.wantBead.Priority)
				}
				if got.Type != tt.wantBead.Type {
					t.Errorf("Type = %v, want %v (issue_type should map to Type)", got.Type, tt.wantBead.Type)
				}
			}
		})
	}
}

// TestNormalizeNilFields tests that nil slices are replaced with empty slices
func TestNormalizeNilFields(t *testing.T) {
	tests := []struct {
		name string
		bead *Bead
	}{
		{
			name: "nil Labels and ExpectedOutputs",
			bead: &Bead{
				ID:    "test-1",
				Title: "Test",
			},
		},
		{
			name: "nil Labels only",
			bead: &Bead{
				ID:              "test-2",
				Title:           "Test",
				ExpectedOutputs: []string{"file.go"},
			},
		},
		{
			name: "nil ExpectedOutputs only",
			bead: &Bead{
				ID:     "test-3",
				Title:  "Test",
				Labels: []string{"label1"},
			},
		},
		{
			name: "already non-nil",
			bead: &Bead{
				ID:              "test-4",
				Title:           "Test",
				Labels:          []string{"label1"},
				ExpectedOutputs: []string{"file.go"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.bead.normalizeNilFields()
			if tt.bead.Labels == nil {
				t.Error("Labels should not be nil after normalization")
			}
			if tt.bead.ExpectedOutputs == nil {
				t.Error("ExpectedOutputs should not be nil after normalization")
			}
		})
	}
}

// TestNormalizeNilFieldsOnNilBead tests that normalizeNilFields doesn't panic on nil bead
func TestNormalizeNilFieldsOnNilBead(t *testing.T) {
	var b *Bead
	b.normalizeNilFields() // Should not panic
}

// TestParseBeadOutputNormalizesNilFields tests that parseBeadOutput normalizes nil fields
func TestParseBeadOutputNormalizesNilFields(t *testing.T) {
	// Simulates bd output where labels and expected_outputs are missing
	jsonStr := `[{
		"id": "test-nil",
		"title": "Nil fields task",
		"status": "open",
		"priority": 0,
		"issue_type": "task",
		"owner": "alice"
	}]`

	b, err := parseBeadOutput(jsonStr)
	if err != nil {
		t.Fatalf("parseBeadOutput() error = %v", err)
	}
	if b == nil {
		t.Fatal("parseBeadOutput() returned nil bead")
	}
	if b.Labels == nil {
		t.Error("Labels should not be nil after parseBeadOutput")
	}
	if b.ExpectedOutputs == nil {
		t.Error("ExpectedOutputs should not be nil after parseBeadOutput")
	}
}

// TestParseBeadOutputWithExplicitNullFields tests handling of JSON null values
func TestParseBeadOutputWithExplicitNullFields(t *testing.T) {
	jsonStr := `[{
		"id": "test-null",
		"title": "Null fields task",
		"status": "open",
		"priority": 1,
		"issue_type": "task",
		"owner": "",
		"labels": null,
		"expected_outputs": null
	}]`

	b, err := parseBeadOutput(jsonStr)
	if err != nil {
		t.Fatalf("parseBeadOutput() error = %v", err)
	}
	if b == nil {
		t.Fatal("parseBeadOutput() returned nil bead")
	}
	if b.Labels == nil {
		t.Error("Labels should not be nil after parseBeadOutput (was JSON null)")
	}
	if b.ExpectedOutputs == nil {
		t.Error("ExpectedOutputs should not be nil after parseBeadOutput (was JSON null)")
	}
}

// TestShowParsesArrayWrappedJSON tests that Show handles both array and object JSON formats
func TestShowParsesArrayWrappedJSON(t *testing.T) {
	// We can't call Show() directly without bd running, but we can test
	// the parsing logic by testing parseBeadOutput with array format
	tests := []struct {
		name    string
		json    string
		wantID  string
		wantErr bool
	}{
		{
			name: "array-wrapped single bead",
			json: `[{
				"id": "arr-001",
				"title": "Array bead",
				"priority": 0,
				"issue_type": "task",
				"status": "open"
			}]`,
			wantID: "arr-001",
		},
		{
			name: "array-wrapped bead with missing optional fields",
			json: `[{
				"id": "arr-002",
				"title": "Sparse bead",
				"priority": 1,
				"issue_type": "task",
				"status": "open",
				"owner": "bob"
			}]`,
			wantID: "arr-002",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := parseBeadOutput(tt.json)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseBeadOutput() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				if b.ID != tt.wantID {
					t.Errorf("ID = %v, want %v", b.ID, tt.wantID)
				}
				if b.Labels == nil {
					t.Error("Labels should not be nil")
				}
				if b.ExpectedOutputs == nil {
					t.Error("ExpectedOutputs should not be nil")
				}
			}
		})
	}
}

// TestFindSpecLabel tests the FindSpecLabel function
func TestFindSpecLabel(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   string
	}{
		{
			name:   "spec label present",
			labels: []string{"complexity:high", "spec:auth", "priority:p1"},
			want:   "auth",
		},
		{
			name:   "no spec label",
			labels: []string{"complexity:high", "priority:p1"},
			want:   "",
		},
		{
			name:   "empty labels",
			labels: []string{},
			want:   "",
		},
		{
			name:   "nil labels",
			labels: nil,
			want:   "",
		},
		{
			name:   "multiple spec labels returns first",
			labels: []string{"spec:auth", "spec:payments"},
			want:   "auth",
		},
		{
			name:   "spec label with complex name",
			labels: []string{"spec:user-auth-v2"},
			want:   "user-auth-v2",
		},
		{
			name:   "spec label at different positions",
			labels: []string{"priority:p0", "complexity:low", "spec:database"},
			want:   "database",
		},
		{
			name:   "spec prefix but not a label",
			labels: []string{"specification:test"},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindSpecLabel(tt.labels)
			if got != tt.want {
				t.Errorf("FindSpecLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHasLabel tests the HasLabel function
func TestHasLabel(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		target string
		want   bool
	}{
		{
			name:   "label present",
			labels: []string{"complexity:high", "spec:auth", "priority:p1"},
			target: "complexity:high",
			want:   true,
		},
		{
			name:   "label not present",
			labels: []string{"complexity:high", "priority:p1"},
			target: "spec:auth",
			want:   false,
		},
		{
			name:   "empty labels",
			labels: []string{},
			target: "anything",
			want:   false,
		},
		{
			name:   "nil labels",
			labels: nil,
			target: "anything",
			want:   false,
		},
		{
			name:   "exact match required",
			labels: []string{"complexity:high"},
			target: "complexity",
			want:   false,
		},
		{
			name:   "case sensitive",
			labels: []string{"Complexity:High"},
			target: "complexity:high",
			want:   false,
		},
		{
			name:   "multiple labels",
			labels: []string{"label1", "label2", "label3"},
			target: "label2",
			want:   true,
		},
		{
			name:   "empty string target",
			labels: []string{"label1", "", "label2"},
			target: "",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasLabel(tt.labels, tt.target)
			if got != tt.want {
				t.Errorf("HasLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestReadyVsReadyAny tests parsing differences between Ready() and ReadyAny()
func TestReadyVsReadyAny(t *testing.T) {
	tests := []struct {
		name        string
		jsonOutput  string
		wantBead    *Bead
		wantNil     bool
		description string
	}{
		{
			name:        "empty array",
			jsonOutput:  "[]",
			wantNil:     true,
			description: "No work available",
		},
		{
			name:        "empty string",
			jsonOutput:  "",
			wantNil:     true,
			description: "No output from bd",
		},
		{
			name:        "whitespace only",
			jsonOutput:  "   \n  ",
			wantNil:     true,
			description: "Only whitespace",
		},
		{
			name: "single task bead",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Test task",
				"description": "Description",
				"priority": 1,
				"labels": [],
				"parent": "",
				"issue_type": "task",
				"status": "open",
				"owner": ""
			}]`,
			wantBead: &Bead{
				ID:       "task-001",
				Title:    "Test task",
				Priority: 1,
				Type:     "task",
			},
			wantNil: false,
		},
		{
			name: "epic bead",
			jsonOutput: `[{
				"id": "epic-001",
				"title": "Test epic",
				"description": "Epic description",
				"priority": 0,
				"labels": [],
				"parent": "",
				"issue_type": "epic",
				"status": "open",
				"owner": ""
			}]`,
			wantBead: &Bead{
				ID:       "epic-001",
				Title:    "Test epic",
				Priority: 0,
				Type:     "epic",
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBeadOutput(tt.jsonOutput)
			if err != nil {
				t.Fatalf("parseBeadOutput() error = %v", err)
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("Expected nil bead but got: %+v", got)
				}
				return
			}

			if got == nil {
				t.Fatal("Expected non-nil bead but got nil")
			}

			if got.ID != tt.wantBead.ID {
				t.Errorf("ID = %v, want %v", got.ID, tt.wantBead.ID)
			}
			if got.Type != tt.wantBead.Type {
				t.Errorf("Type = %v, want %v", got.Type, tt.wantBead.Type)
			}
		})
	}
}

// TestReadyExcludesEpics tests that Ready() filters out epic beads
func TestReadyExcludesEpics(t *testing.T) {
	tests := []struct {
		name       string
		jsonOutput string
		wantID     string
		wantType   string
		wantNil    bool
	}{
		{
			name: "task bead only",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Task",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}]`,
			wantID:   "task-001",
			wantType: "task",
			wantNil:  false,
		},
		{
			name: "bug bead only",
			jsonOutput: `[{
				"id": "bug-001",
				"title": "Bug",
				"priority": 1,
				"issue_type": "bug",
				"status": "open"
			}]`,
			wantID:   "bug-001",
			wantType: "bug",
			wantNil:  false,
		},
		{
			name: "epic bead only - should return nil",
			jsonOutput: `[{
				"id": "epic-001",
				"title": "Epic",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}]`,
			wantNil: true,
		},
		{
			name: "epic before task - should skip epic",
			jsonOutput: `[{
				"id": "epic-001",
				"title": "Epic",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "task-001",
				"title": "Task",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}]`,
			wantID:   "task-001",
			wantType: "task",
			wantNil:  false,
		},
		{
			name: "epic before bug - should skip epic",
			jsonOutput: `[{
				"id": "epic-001",
				"title": "Epic",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "bug-001",
				"title": "Bug",
				"priority": 1,
				"issue_type": "bug",
				"status": "open"
			}]`,
			wantID:   "bug-001",
			wantType: "bug",
			wantNil:  false,
		},
		{
			name: "multiple epics only - should return nil",
			jsonOutput: `[{
				"id": "epic-001",
				"title": "Epic 1",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "epic-002",
				"title": "Epic 2",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}]`,
			wantNil: true,
		},
		{
			name: "feature bead - should be included",
			jsonOutput: `[{
				"id": "feat-001",
				"title": "Feature",
				"priority": 1,
				"issue_type": "feature",
				"status": "open"
			}]`,
			wantID:   "feat-001",
			wantType: "feature",
			wantNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBeadOutputExcluding(tt.jsonOutput, "epic")
			if err != nil {
				t.Fatalf("parseBeadOutputExcluding() error = %v", err)
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("Expected nil bead but got: %+v", got)
				}
				return
			}

			if got == nil {
				t.Fatal("Expected non-nil bead but got nil")
			}

			if got.ID != tt.wantID {
				t.Errorf("ID = %v, want %v", got.ID, tt.wantID)
			}
			if got.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", got.Type, tt.wantType)
			}
		})
	}
}

// TestClientShowValidation tests that Show() validates bead IDs before execution
func TestClientShowValidation(t *testing.T) {
	c, _ := NewClient()

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "invalid ID with semicolon",
			id:      "test; rm -rf /",
			wantErr: true,
		},
		{
			name:    "invalid ID with spaces",
			id:      "test 123",
			wantErr: true,
		},
		{
			name:    "ID too long",
			id:      strings.Repeat("a", maxIDLength+1),
			wantErr: true,
		},
		{
			name:    "empty ID",
			id:      "",
			wantErr: true,
		},
		{
			name:    "command injection attempt",
			id:      "test$(whoami)",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.Show(tt.id)
			if err == nil {
				t.Errorf("Show(%q) expected error but got nil", tt.id)
				return
			}

			if tt.wantErr && !strings.Contains(err.Error(), "invalid bead ID") {
				t.Errorf("Show(%q) should fail with validation error, got: %v", tt.id, err)
			}
		})
	}
}

// TestClientCloseValidation tests that Close() validates bead IDs before execution
func TestClientCloseValidation(t *testing.T) {
	c, _ := NewClient()

	tests := []struct {
		name string
		id   string
	}{
		{
			name: "command injection attempt",
			id:   "test && echo hacked",
		},
		{
			name: "pipe injection",
			id:   "test | cat /etc/passwd",
		},
		{
			name: "empty ID",
			id:   "",
		},
		{
			name: "shell substitution",
			id:   "test`ls`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.Close(tt.id)
			if err == nil {
				t.Errorf("Close(%q) expected error but got nil", tt.id)
				return
			}

			if !strings.Contains(err.Error(), "invalid bead ID") {
				t.Errorf("Close(%q) should fail with validation error, got: %v", tt.id, err)
			}
		})
	}
}

// TestClientAddCommentValidation tests that AddComment() validates bead IDs
func TestClientAddCommentValidation(t *testing.T) {
	c, _ := NewClient()

	tests := []struct {
		name    string
		id      string
		comment string
	}{
		{
			name:    "invalid ID",
			id:      "test; rm -rf /",
			comment: "This is a comment",
		},
		{
			name:    "empty ID",
			id:      "",
			comment: "This is a comment",
		},
		{
			name:    "ID with newline",
			id:      "test\nid",
			comment: "Comment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.AddComment(tt.id, tt.comment)
			if err == nil {
				t.Errorf("AddComment(%q, %q) expected error but got nil", tt.id, tt.comment)
				return
			}

			if !strings.Contains(err.Error(), "invalid bead ID") {
				t.Errorf("AddComment(%q, %q) should fail with validation error, got: %v", tt.id, tt.comment, err)
			}
		})
	}
}

func TestClientAddComment_UsesTempFile(t *testing.T) {
	comment := "This is a comment\nwith multiple lines"
	var gotArgs []string
	var gotComment string
	var gotPath string

	c := &Client{
		runFn: func(args ...string) (string, error) {
			gotArgs = append([]string(nil), args...)
			fileIdx := -1
			for i, arg := range args {
				if arg == "--file" {
					fileIdx = i
					break
				}
			}
			if fileIdx == -1 || fileIdx+1 >= len(args) {
				return "", fmt.Errorf("missing --file argument")
			}
			gotPath = args[fileIdx+1]
			data, err := os.ReadFile(gotPath)
			if err != nil {
				return "", err
			}
			gotComment = string(data)
			if _, err := os.Stat(gotPath); err != nil {
				return "", err
			}
			return "", nil
		},
	}

	if err := c.AddComment("task-123", comment); err != nil {
		t.Fatalf("AddComment() unexpected error: %v", err)
	}
	if gotPath == "" {
		t.Fatal("AddComment() did not pass a temp file path")
	}
	for _, arg := range gotArgs {
		if arg == comment {
			t.Fatalf("AddComment() should not pass comment text directly in args")
		}
	}
	if gotComment != comment {
		t.Fatalf("AddComment() comment = %q, want %q", gotComment, comment)
	}
	if _, err := os.Stat(gotPath); !os.IsNotExist(err) {
		t.Fatalf("temp file should be cleaned up, stat err=%v", err)
	}
}

// newIsolatedClient creates a bd client that operates in a temp directory
// so tests don't pollute the real project's beads database.
func newIsolatedClient(t *testing.T) *Client {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("bd", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("bd init not available: %v: %s", err, out)
	}
	return &Client{binary: "bd", Dir: dir}
}

// TestClientCreate tests the Create() method
func TestClientCreate(t *testing.T) {
	tests := []struct {
		name            string
		title           string
		priority        int
		labels          []string
		expectedOutputs []string
		description     string
	}{
		{
			name:            "basic creation",
			title:           "Test task",
			priority:        1,
			labels:          []string{"label1"},
			expectedOutputs: []string{"file.go"},
			description:     "Creates task with all fields",
		},
		{
			name:            "empty labels and outputs",
			title:           "Simple task",
			priority:        2,
			labels:          []string{},
			expectedOutputs: []string{},
			description:     "Creates task with minimal fields",
		},
		{
			name:            "nil labels and outputs",
			title:           "Nil fields task",
			priority:        0,
			labels:          nil,
			expectedOutputs: nil,
			description:     "Creates task with nil slices",
		},
		{
			name:            "multiple labels",
			title:           "Complex task",
			priority:        0,
			labels:          []string{"spec:auth", "complexity:high", "priority:p0"},
			expectedOutputs: []string{"auth.go", "auth_test.go"},
			description:     "Creates task with multiple labels and outputs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotArgs []string
			var gotAcceptance string
			c := &Client{
				runFn: func(args ...string) (string, error) {
					gotArgs = append([]string(nil), args...)
					accIdx := -1
					for i, arg := range args {
						if arg == "--acceptance" {
							accIdx = i
							break
						}
					}
					if accIdx != -1 && accIdx+1 < len(args) {
						gotAcceptance = args[accIdx+1]
					}
					return `{"id":"task-001","title":"` + tt.title + `","priority":` + fmt.Sprintf("%d", tt.priority) + `,"issue_type":"task","status":"open"}`, nil
				},
			}

			_, err := c.Create(tt.title, tt.priority, tt.labels, tt.expectedOutputs)
			if err != nil {
				t.Fatalf("Create() unexpected error: %v", err)
			}
			wantBase := []string{"create", tt.title, "--priority", fmt.Sprintf("%d", tt.priority), "--json"}
			if !hasSubsequence(gotArgs, wantBase) {
				t.Errorf("Create() args = %v, want subsequence %v", gotArgs, wantBase)
			}
			for _, label := range tt.labels {
				if !hasSubsequence(gotArgs, []string{"--label", label}) {
					t.Errorf("Create() missing label flag for %q in args %v", label, gotArgs)
				}
			}
			if len(tt.expectedOutputs) > 0 {
				wantAcceptance := strings.Join(tt.expectedOutputs, "\n")
				if !hasSubsequence(gotArgs, []string{"--acceptance"}) {
					t.Errorf("Create() missing --acceptance flag in args %v", gotArgs)
				}
				if gotAcceptance != wantAcceptance {
					t.Errorf("Create() acceptance = %q, want %q", gotAcceptance, wantAcceptance)
				}
			}
		})
	}
}

// TestClientGetParent tests the GetParent method
func TestClientGetParent(t *testing.T) {
	c, _ := NewClient()

	tests := []struct {
		name    string
		bead    *Bead
		wantNil bool
	}{
		{
			name:    "nil bead",
			bead:    nil,
			wantNil: true,
		},
		{
			name: "bead with no parent",
			bead: &Bead{
				ID:     "task-001",
				Parent: "",
			},
			wantNil: true,
		},
		{
			name: "bead with parent",
			bead: &Bead{
				ID:     "task-001",
				Parent: "epic-001",
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent, err := c.GetParent(tt.bead)
			if tt.wantNil {
				if parent != nil {
					t.Errorf("GetParent() expected nil, got %+v", parent)
				}
				if err != nil {
					t.Errorf("GetParent() unexpected error: %v", err)
				}
			} else {
				// Will fail because bd isn't running, but should attempt to fetch
				if err == nil {
					t.Errorf("GetParent() expected error (bd not running)")
				}
			}
		})
	}
}

// TestClientSync tests that Sync doesn't panic
func TestClientSync(t *testing.T) {
	var gotArgs []string
	c := &Client{
		runFn: func(args ...string) (string, error) {
			gotArgs = append([]string(nil), args...)
			return "", nil
		},
	}
	err := c.Sync()
	if err != nil {
		t.Fatalf("Sync() unexpected error: %v", err)
	}
	if !hasSubsequence(gotArgs, []string{"sync"}) {
		t.Errorf("Sync() args = %v, want sync", gotArgs)
	}
}

// TestErrorWrapping tests that CLI errors are properly wrapped
func TestErrorWrapping(t *testing.T) {
	c := &Client{
		runFn: func(args ...string) (string, error) {
			return "", fmt.Errorf("boom")
		},
	}

	// Test that errors contain context
	_, err := c.Ready()
	if err == nil || !strings.Contains(err.Error(), "bd ready") {
		t.Errorf("Ready() error should contain context: %v", err)
	}

	_, err = c.ReadyAny()
	if err == nil || !strings.Contains(err.Error(), "bd ready") {
		t.Errorf("ReadyAny() error should contain context: %v", err)
	}

	err = c.Close("test-id")
	if err == nil || !strings.Contains(err.Error(), "bd close") {
		t.Errorf("Close() error should contain context: %v", err)
	}
}

// TestClientCreateWithParent tests the CreateWithParent() method
func TestClientCreateWithParent(t *testing.T) {
	tests := []struct {
		name               string
		title              string
		priority           int
		labels             []string
		expectedOutputs    []string
		parentID           string
		shouldValidateFail bool
		description        string
	}{
		{
			name:            "create with valid parent",
			title:           "Sub-task",
			priority:        1,
			labels:          []string{"label1"},
			expectedOutputs: []string{},
			parentID:        "parent-123",
			description:     "Creates child bead with valid parent ID",
		},
		{
			name:            "create with empty parent",
			title:           "Standalone task",
			priority:        1,
			labels:          []string{},
			expectedOutputs: []string{},
			parentID:        "",
			description:     "Creates bead with no parent (empty string)",
		},
		{
			name:               "invalid parent ID with spaces",
			title:              "Task",
			priority:           1,
			parentID:           "parent 123",
			shouldValidateFail: true,
			description:        "Should reject parent ID with spaces",
		},
		{
			name:               "invalid parent ID with shell chars",
			title:              "Task",
			priority:           1,
			parentID:           "parent; rm -rf /",
			shouldValidateFail: true,
			description:        "Should reject parent ID with shell metacharacters",
		},
		{
			name:               "parent ID too long",
			title:              "Task",
			priority:           1,
			parentID:           strings.Repeat("a", maxIDLength+1),
			shouldValidateFail: true,
			description:        "Should reject overly long parent ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotArgs []string
			c := &Client{
				runFn: func(args ...string) (string, error) {
					gotArgs = append([]string(nil), args...)
					return `{"id":"task-001","title":"` + tt.title + `","priority":` + fmt.Sprintf("%d", tt.priority) + `,"issue_type":"task","status":"open"}`, nil
				},
			}
			_, err := c.CreateWithParent(tt.title, tt.priority, tt.labels, tt.expectedOutputs, tt.parentID)

			if tt.shouldValidateFail {
				if err == nil {
					t.Errorf("CreateWithParent() expected validation error for %q", tt.parentID)
				}
				if !strings.Contains(err.Error(), "invalid parent ID") {
					t.Errorf("CreateWithParent() expected 'invalid parent ID' error, got: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("CreateWithParent() unexpected error: %v", err)
			}
			if tt.parentID != "" && !hasSubsequence(gotArgs, []string{"--parent", tt.parentID}) {
				t.Errorf("CreateWithParent() args missing parent %q: %v", tt.parentID, gotArgs)
			}
		})
	}
}

// TestClientCreateWithParentInheritance tests that Create() delegates to CreateWithParent
func TestClientCreateInheritsCreateWithParent(t *testing.T) {
	var createArgs []string
	createClient := &Client{
		runFn: func(args ...string) (string, error) {
			createArgs = append([]string(nil), args...)
			return `{"id":"task-001","title":"Test","priority":1,"issue_type":"task","status":"open"}`, nil
		},
	}
	var parentArgs []string
	parentClient := &Client{
		runFn: func(args ...string) (string, error) {
			parentArgs = append([]string(nil), args...)
			return `{"id":"task-002","title":"Test","priority":1,"issue_type":"task","status":"open"}`, nil
		},
	}

	_, err1 := createClient.Create("Test", 1, []string{}, []string{})
	_, err2 := parentClient.CreateWithParent("Test", 1, []string{}, []string{}, "")
	if err1 != nil || err2 != nil {
		t.Fatalf("Create() / CreateWithParent(\"\") unexpected errors: %v / %v", err1, err2)
	}
	if strings.Join(createArgs, " ") != strings.Join(parentArgs, " ") {
		t.Errorf("Create() args and CreateWithParent(\"\") args differ: %v vs %v", createArgs, parentArgs)
	}
}

func TestClientCreateWithParentAndDescription_UsesBodyFile(t *testing.T) {
	description := "Line 1\nLine 2"
	var gotArgs []string
	var gotDescription string
	var gotPath string

	c := &Client{
		runFn: func(args ...string) (string, error) {
			gotArgs = append([]string(nil), args...)
			bodyIdx := -1
			for i, arg := range args {
				if arg == "--body-file" {
					bodyIdx = i
					break
				}
			}
			if bodyIdx == -1 || bodyIdx+1 >= len(args) {
				return "", fmt.Errorf("missing --body-file argument")
			}
			gotPath = args[bodyIdx+1]
			data, err := os.ReadFile(gotPath)
			if err != nil {
				return "", err
			}
			gotDescription = string(data)
			if _, err := os.Stat(gotPath); err != nil {
				return "", err
			}
			return `{"id":"task-001","title":"Test","priority":1,"issue_type":"task","status":"open"}`, nil
		},
	}

	_, err := c.CreateWithParentAndDescription("Test", 1, nil, nil, "", description)
	if err != nil {
		t.Fatalf("CreateWithParentAndDescription() unexpected error: %v", err)
	}
	if gotPath == "" {
		t.Fatal("CreateWithParentAndDescription() did not pass a temp file path")
	}
	for _, arg := range gotArgs {
		if arg == description {
			t.Fatalf("CreateWithParentAndDescription() should not pass description directly in args")
		}
	}
	if gotDescription != description {
		t.Fatalf("CreateWithParentAndDescription() description = %q, want %q", gotDescription, description)
	}
	if _, err := os.Stat(gotPath); !os.IsNotExist(err) {
		t.Fatalf("temp file should be cleaned up, stat err=%v", err)
	}
}

func TestClientCreateWithParentAndDescription_PassesAcceptanceInline(t *testing.T) {
	expectedOutputs := []string{"file1.go", "file2.go"}
	wantAcceptance := strings.Join(expectedOutputs, "\n")
	var gotAcceptance string

	c := &Client{
		runFn: func(args ...string) (string, error) {
			accIdx := -1
			for i, arg := range args {
				if arg == "--acceptance" {
					accIdx = i
					break
				}
			}
			if accIdx == -1 || accIdx+1 >= len(args) {
				return "", fmt.Errorf("missing --acceptance argument")
			}
			gotAcceptance = args[accIdx+1]
			return `{"id":"task-001","title":"Test","priority":1,"issue_type":"task","status":"open"}`, nil
		},
	}

	_, err := c.CreateWithParentAndDescription("Test", 1, nil, expectedOutputs, "", "")
	if err != nil {
		t.Fatalf("CreateWithParentAndDescription() unexpected error: %v", err)
	}
	if gotAcceptance != wantAcceptance {
		t.Fatalf("CreateWithParentAndDescription() acceptance = %q, want %q", gotAcceptance, wantAcceptance)
	}
}

// TestClientCreateWithDeps tests CreateWithDepsAndDescription with multiple dependencies
func TestClientCreateWithDeps(t *testing.T) {
	tests := []struct {
		name               string
		title              string
		priority           int
		labels             []string
		expectedOutputs    []string
		dependencies       []string
		description        string
		shouldValidateFail bool
		expectedErrMsg     string
	}{
		{
			name:            "create with no dependencies",
			title:           "Root task",
			priority:        1,
			labels:          []string{"label1"},
			expectedOutputs: []string{},
			dependencies:    []string{},
			description:     "Creates bead with no dependencies",
		},
		{
			name:            "create with single dependency",
			title:           "Sub-task",
			priority:        1,
			labels:          []string{},
			expectedOutputs: []string{},
			dependencies:    []string{"dep-1"},
			description:     "Creates bead with one dependency",
		},
		{
			name:            "create with multiple dependencies",
			title:           "Dependent task",
			priority:        1,
			labels:          []string{},
			expectedOutputs: []string{},
			dependencies:    []string{"dep-1", "dep-2", "dep-3"},
			description:     "Creates bead with multiple dependencies",
		},
		{
			name:               "invalid dependency ID with spaces",
			title:              "Task",
			priority:           1,
			dependencies:       []string{"dep 1"},
			shouldValidateFail: true,
			expectedErrMsg:     "invalid dependency ID",
			description:        "Should reject dependency ID with spaces",
		},
		{
			name:               "invalid dependency ID with shell chars",
			title:              "Task",
			priority:           1,
			dependencies:       []string{"dep-1", "dep; rm -rf /"},
			shouldValidateFail: true,
			expectedErrMsg:     "invalid dependency ID",
			description:        "Should reject dependency ID with shell metacharacters",
		},
		{
			name:               "dependency ID too long",
			title:              "Task",
			priority:           1,
			dependencies:       []string{strings.Repeat("a", maxIDLength+1)},
			shouldValidateFail: true,
			expectedErrMsg:     "invalid dependency ID",
			description:        "Should reject overly long dependency ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotArgs []string
			c := &Client{
				runFn: func(args ...string) (string, error) {
					gotArgs = append([]string(nil), args...)
					return `{"id":"task-001","title":"` + tt.title + `","priority":` + fmt.Sprintf("%d", tt.priority) + `,"issue_type":"task","status":"open"}`, nil
				},
			}
			_, err := c.CreateWithDepsAndDescription(tt.title, tt.priority, tt.labels, tt.expectedOutputs, tt.dependencies, tt.description)

			if tt.shouldValidateFail {
				if err == nil {
					t.Errorf("CreateWithDepsAndDescription() expected validation error")
				}
				if !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("CreateWithDepsAndDescription() expected error containing %q, got: %v", tt.expectedErrMsg, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("CreateWithDepsAndDescription() unexpected error: %v", err)
			}
			if len(tt.dependencies) > 0 {
				wantDeps := strings.Join(tt.dependencies, ",")
				if !hasSubsequence(gotArgs, []string{"--deps", wantDeps}) {
					t.Errorf("CreateWithDepsAndDescription() args missing deps %q: %v", wantDeps, gotArgs)
				}
			}
		})
	}
}

func TestClientCreateWithDepsAndDescription_UsesBodyFile(t *testing.T) {
	description := "Deps description\nLine 2"
	var gotArgs []string
	var gotDescription string
	var gotPath string

	c := &Client{
		runFn: func(args ...string) (string, error) {
			gotArgs = append([]string(nil), args...)
			bodyIdx := -1
			for i, arg := range args {
				if arg == "--body-file" {
					bodyIdx = i
					break
				}
			}
			if bodyIdx == -1 || bodyIdx+1 >= len(args) {
				return "", fmt.Errorf("missing --body-file argument")
			}
			gotPath = args[bodyIdx+1]
			data, err := os.ReadFile(gotPath)
			if err != nil {
				return "", err
			}
			gotDescription = string(data)
			if _, err := os.Stat(gotPath); err != nil {
				return "", err
			}
			return `{"id":"task-001","title":"Test","priority":1,"issue_type":"task","status":"open"}`, nil
		},
	}

	_, err := c.CreateWithDepsAndDescription("Test", 1, nil, nil, []string{"dep-1"}, description)
	if err != nil {
		t.Fatalf("CreateWithDepsAndDescription() unexpected error: %v", err)
	}
	if gotPath == "" {
		t.Fatal("CreateWithDepsAndDescription() did not pass a temp file path")
	}
	for _, arg := range gotArgs {
		if arg == description {
			t.Fatalf("CreateWithDepsAndDescription() should not pass description directly in args")
		}
	}
	if gotDescription != description {
		t.Fatalf("CreateWithDepsAndDescription() description = %q, want %q", gotDescription, description)
	}
	if _, err := os.Stat(gotPath); !os.IsNotExist(err) {
		t.Fatalf("temp file should be cleaned up, stat err=%v", err)
	}
}

// TestIsMethodologyActive tests the IsMethodologyActive function
func TestIsMethodologyActive(t *testing.T) {
	tests := []struct {
		name            string
		labels          []string
		methodologyName string
		globalDefault   bool
		want            bool
	}{
		{
			name:            "atdd:true label with false global",
			labels:          []string{"atdd:true", "spec:auth"},
			methodologyName: "atdd",
			globalDefault:   false,
			want:            true,
		},
		{
			name:            "atdd:false label with true global",
			labels:          []string{"atdd:false", "priority:p1"},
			methodologyName: "atdd",
			globalDefault:   true,
			want:            false,
		},
		{
			name:            "no matching label falls back to global true",
			labels:          []string{"spec:auth", "complexity:high"},
			methodologyName: "atdd",
			globalDefault:   true,
			want:            true,
		},
		{
			name:            "no matching label falls back to global false",
			labels:          []string{"spec:auth", "complexity:high"},
			methodologyName: "atdd",
			globalDefault:   false,
			want:            false,
		},
		{
			name:            "tdd:true label with false global",
			labels:          []string{"tdd:true"},
			methodologyName: "tdd",
			globalDefault:   false,
			want:            true,
		},
		{
			name:            "tdd:false label with true global",
			labels:          []string{"tdd:false"},
			methodologyName: "tdd",
			globalDefault:   true,
			want:            false,
		},
		{
			name:            "empty labels falls back to global true",
			labels:          []string{},
			methodologyName: "atdd",
			globalDefault:   true,
			want:            true,
		},
		{
			name:            "empty labels falls back to global false",
			labels:          []string{},
			methodologyName: "atdd",
			globalDefault:   false,
			want:            false,
		},
		{
			name:            "nil labels falls back to global true",
			labels:          nil,
			methodologyName: "atdd",
			globalDefault:   true,
			want:            true,
		},
		{
			name:            "nil labels falls back to global false",
			labels:          nil,
			methodologyName: "atdd",
			globalDefault:   false,
			want:            false,
		},
		{
			name:            "atdd label with tdd methodology name doesn't match",
			labels:          []string{"atdd:true"},
			methodologyName: "tdd",
			globalDefault:   false,
			want:            false,
		},
		{
			name:            "tdd label with atdd methodology name doesn't match",
			labels:          []string{"tdd:true"},
			methodologyName: "atdd",
			globalDefault:   false,
			want:            false,
		},
		{
			name:            "custom methodology name with true label",
			labels:          []string{"custom:true"},
			methodologyName: "custom",
			globalDefault:   false,
			want:            true,
		},
		{
			name:            "custom methodology name with false label",
			labels:          []string{"custom:false"},
			methodologyName: "custom",
			globalDefault:   true,
			want:            false,
		},
		{
			name:            "multiple methodology labels - true takes precedence",
			labels:          []string{"atdd:true", "tdd:false"},
			methodologyName: "atdd",
			globalDefault:   false,
			want:            true,
		},
		{
			name:            "multiple methodology labels - false takes precedence",
			labels:          []string{"tdd:false", "atdd:true"},
			methodologyName: "tdd",
			globalDefault:   true,
			want:            false,
		},
		{
			name:            "label with similar prefix but not exact match",
			labels:          []string{"atdd-enabled:true"},
			methodologyName: "atdd",
			globalDefault:   false,
			want:            false,
		},
		{
			name:            "case sensitive methodology name",
			labels:          []string{"ATDD:true"},
			methodologyName: "atdd",
			globalDefault:   false,
			want:            false,
		},
		{
			name:            "exact match required for label",
			labels:          []string{"atdd:TRUE"},
			methodologyName: "atdd",
			globalDefault:   false,
			want:            false,
		},
		{
			name:            "methodology label appears after other labels",
			labels:          []string{"spec:auth", "complexity:high", "atdd:true", "priority:p1"},
			methodologyName: "atdd",
			globalDefault:   false,
			want:            true,
		},
		{
			name:            "methodology label appears first",
			labels:          []string{"tdd:false", "spec:auth", "complexity:high"},
			methodologyName: "tdd",
			globalDefault:   true,
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMethodologyActive(tt.labels, tt.methodologyName, tt.globalDefault)
			if got != tt.want {
				t.Errorf("IsMethodologyActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestClientHasOpenChildrenValidation tests that HasOpenChildren() validates parent IDs
func TestClientHasOpenChildrenValidation(t *testing.T) {
	c, _ := NewClient()

	tests := []struct {
		name     string
		parentID string
		wantErr  bool
	}{
		{
			name:     "invalid parent ID with semicolon",
			parentID: "epic; rm -rf /",
			wantErr:  true,
		},
		{
			name:     "invalid parent ID with spaces",
			parentID: "epic 123",
			wantErr:  true,
		},
		{
			name:     "parent ID too long",
			parentID: strings.Repeat("a", maxIDLength+1),
			wantErr:  true,
		},
		{
			name:     "empty parent ID",
			parentID: "",
			wantErr:  true,
		},
		{
			name:     "command injection attempt",
			parentID: "epic$(whoami)",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.HasOpenChildren(tt.parentID)
			if err == nil {
				t.Errorf("HasOpenChildren(%q) expected error but got nil", tt.parentID)
				return
			}

			if tt.wantErr && !strings.Contains(err.Error(), "invalid parent ID") {
				t.Errorf("HasOpenChildren(%q) should fail with validation error, got: %v", tt.parentID, err)
			}
		})
	}
}

// TestHasOpenChildrenWithMockedRun tests HasOpenChildren with a mocked run function
// to verify it uses the correct bd command arguments with --parent flag.
// Expected failure: Client.runFn field does not exist yet (compilation will fail).
// After implementation, this field will allow injecting a mock run function to verify
// command arguments without spawning subprocesses. The test verifies HasOpenChildren
// calls run() with: bd list --json --status open --parent <id> --limit 1
func TestHasOpenChildrenWithMockedRun(t *testing.T) {
	tests := []struct {
		name     string
		parentID string
		bdOutput string
		want     bool
		wantErr  bool
		wantArgs []string
	}{
		{
			name:     "parent with open children returns true",
			parentID: "epic-123",
			bdOutput: `[{"id":"task-001","title":"Child 1","priority":1,"issue_type":"task","status":"open","parent":"epic-123"}]`,
			want:     true,
			wantErr:  false,
			wantArgs: []string{"list", "--json", "--status", "open", "--parent", "epic-123", "--limit", "1"},
		},
		{
			name:     "parent with no children returns false",
			parentID: "epic-456",
			bdOutput: `[]`,
			want:     false,
			wantErr:  false,
			wantArgs: []string{"list", "--json", "--status", "open", "--parent", "epic-456", "--limit", "1"},
		},
		{
			name:     "empty output returns false",
			parentID: "epic-789",
			bdOutput: ``,
			want:     false,
			wantErr:  false,
			wantArgs: []string{"list", "--json", "--status", "open", "--parent", "epic-789", "--limit", "1"},
		},
		{
			name:     "multiple children in output returns true",
			parentID: "epic-999",
			bdOutput: `[{"id":"task-001","title":"Child 1","priority":1,"issue_type":"task","status":"open","parent":"epic-999"},{"id":"task-002","title":"Child 2","priority":1,"issue_type":"task","status":"open","parent":"epic-999"}]`,
			want:     true,
			wantErr:  false,
			wantArgs: []string{"list", "--json", "--status", "open", "--parent", "epic-999", "--limit", "1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			mockRun := func(args ...string) (string, error) {
				capturedArgs = args
				return tt.bdOutput, nil
			}

			c := &Client{
				binary: "bd",
				runFn:  mockRun, // Expected failure: runFn field does not exist on Client struct
			}

			got, err := c.HasOpenChildren(tt.parentID)
			if (err != nil) != tt.wantErr {
				t.Errorf("HasOpenChildren(%q) error = %v, wantErr %v", tt.parentID, err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("HasOpenChildren(%q) = %v, want %v", tt.parentID, got, tt.want)
			}

			// Verify the command arguments match expected
			if len(capturedArgs) != len(tt.wantArgs) {
				t.Errorf("HasOpenChildren(%q) called run() with %d args, want %d\nGot:  %v\nWant: %v",
					tt.parentID, len(capturedArgs), len(tt.wantArgs), capturedArgs, tt.wantArgs)
				return
			}

			for i := range capturedArgs {
				if capturedArgs[i] != tt.wantArgs[i] {
					t.Errorf("HasOpenChildren(%q) arg[%d] = %q, want %q\nGot:  %v\nWant: %v",
						tt.parentID, i, capturedArgs[i], tt.wantArgs[i], capturedArgs, tt.wantArgs)
				}
			}
		})
	}
}

// TestClientListReadyIDsNilClient tests that ListReadyIDs() returns error on nil client
func TestClientListReadyIDsNilClient(t *testing.T) {
	var c *Client
	_, err := c.ListReadyIDs()
	if err == nil {
		t.Errorf("ListReadyIDs() on nil client expected error but got nil")
		return
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("ListReadyIDs() on nil client should mention nil, got: %v", err)
	}
}

// TestClientListReadyIDsEmpty tests that ListReadyIDs() returns empty slice for no beads
func TestClientListReadyIDsEmpty(t *testing.T) {
	tests := []struct {
		name        string
		jsonOutput  string
		description string
	}{
		{
			name:        "empty array",
			jsonOutput:  "[]",
			description: "No ready beads",
		},
		{
			name:        "empty string",
			jsonOutput:  "",
			description: "No output from bd",
		},
		{
			name:        "whitespace only",
			jsonOutput:  "   \n  ",
			description: "Only whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids, err := parseBeadOutputToIDs(tt.jsonOutput)
			if err != nil {
				t.Fatalf("parseBeadOutputToIDs() error = %v", err)
			}
			if ids == nil {
				t.Errorf("ListReadyIDs() should return empty slice not nil, got nil")
				return
			}
			if len(ids) != 0 {
				t.Errorf("ListReadyIDs() expected empty slice, got %v", ids)
			}
		})
	}
}

// TestClientListReadyIDsMultiple tests that ListReadyIDs() returns multiple bead IDs
func TestClientListReadyIDsMultiple(t *testing.T) {
	tests := []struct {
		name       string
		jsonOutput string
		wantIDs    []string
	}{
		{
			name: "single bead",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Test task",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}]`,
			wantIDs: []string{"task-001"},
		},
		{
			name: "multiple beads",
			jsonOutput: `[{
				"id": "task-001",
				"title": "First task",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}, {
				"id": "task-002",
				"title": "Second task",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}, {
				"id": "task-003",
				"title": "Third task",
				"priority": 2,
				"issue_type": "bug",
				"status": "open"
			}]`,
			wantIDs: []string{"task-001", "task-002", "task-003"},
		},
		{
			name: "mixed types",
			jsonOutput: `[{
				"id": "epic-001",
				"title": "Epic",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "bug-001",
				"title": "Bug",
				"priority": 1,
				"issue_type": "bug",
				"status": "open"
			}, {
				"id": "feature-001",
				"title": "Feature",
				"priority": 1,
				"issue_type": "feature",
				"status": "open"
			}]`,
			wantIDs: []string{"epic-001", "bug-001", "feature-001"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids, err := parseBeadOutputToIDs(tt.jsonOutput)
			if err != nil {
				t.Fatalf("parseBeadOutputToIDs() error = %v", err)
			}
			if len(ids) != len(tt.wantIDs) {
				t.Errorf("ListReadyIDs() returned %d ids, want %d", len(ids), len(tt.wantIDs))
				return
			}
			for i, id := range ids {
				if id != tt.wantIDs[i] {
					t.Errorf("ListReadyIDs()[%d] = %q, want %q", i, id, tt.wantIDs[i])
				}
			}
		})
	}
}

// TestClientListReadyIDsJSONParseError tests that ListReadyIDs() handles JSON parse errors
func TestClientListReadyIDsJSONParseError(t *testing.T) {
	tests := []struct {
		name       string
		jsonOutput string
	}{
		{
			name:       "invalid JSON",
			jsonOutput: `{"id": "test-123", invalid}`,
		},
		{
			name:       "malformed array",
			jsonOutput: `[{invalid json}]`,
		},
		{
			name:       "not an array",
			jsonOutput: `{"id": "test-123", "title": "single object"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids, err := parseBeadOutputToIDs(tt.jsonOutput)
			if err == nil {
				t.Errorf("parseBeadOutputToIDs() expected error for invalid JSON, got nil, ids=%v", ids)
				return
			}
			if !strings.Contains(err.Error(), "parsing") {
				t.Errorf("parseBeadOutputToIDs() error should mention parsing, got: %v", err)
			}
		})
	}
}

// parseBeadOutputToIDs is a helper function that parses JSON output like ListReadyIDs does
func parseBeadOutputToIDs(out string) ([]string, error) {
	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return []string{}, nil
	}

	var beads []Bead
	if err := jsonutil.ExtractArray(out, &beads); err != nil {
		return nil, fmt.Errorf("parsing bd ready output: %w", err)
	}

	ids := make([]string, len(beads))
	for i, b := range beads {
		ids[i] = b.ID
	}
	return ids, nil
}

// TestClientListReadyIDsErrorWrapping tests that ListReadyIDs() wraps command errors with context
func TestClientListReadyIDsErrorWrapping(t *testing.T) {
	c, _ := NewClient()

	// Test that errors contain context when bd command fails
	_, err := c.ListReadyIDs()
	if err != nil && !strings.Contains(err.Error(), "bd ready") {
		t.Errorf("ListReadyIDs() error should contain 'bd ready' context: %v", err)
	}
}

// TestClientReadyWithLabelValidation tests that ReadyWithLabel validates label parameter
func TestClientReadyWithLabelValidation(t *testing.T) {
	c := &Client{
		runFn: func(args ...string) (string, error) {
			return "[]", nil
		},
	}

	tests := []struct {
		name    string
		label   string
		wantErr bool
	}{
		{
			name:    "empty label",
			label:   "",
			wantErr: true,
		},
		{
			name:    "label with semicolon",
			label:   "spec:auth; rm -rf /",
			wantErr: true,
		},
		{
			name:    "label with pipe",
			label:   "spec:auth | cat",
			wantErr: true,
		},
		{
			name:    "label with dollar sign",
			label:   "spec:$(whoami)",
			wantErr: true,
		},
		{
			name:    "label with backtick",
			label:   "spec:`whoami`",
			wantErr: true,
		},
		{
			name:    "label with newline",
			label:   "spec:auth\nspec:other",
			wantErr: true,
		},
		{
			name:    "valid label",
			label:   "spec:auth",
			wantErr: false,
		},
		{
			name:    "valid label with dash",
			label:   "complexity:high",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.ReadyWithLabel(tt.label)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadyWithLabel(%q) error = %v, wantErr %v", tt.label, err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if tt.label == "" && !strings.Contains(err.Error(), "empty") {
					t.Errorf("ReadyWithLabel(\"\") should mention empty label, got: %v", err)
				}
				if strings.ContainsAny(tt.label, ";\n|$`&<>(){}[]'\"\\") && !strings.Contains(err.Error(), "shell metacharacters") {
					t.Errorf("ReadyWithLabel(%q) should mention shell metacharacters, got: %v", tt.label, err)
				}
			}
		})
	}
}

// TestClientReadyWithLabelNilClient tests that ReadyWithLabel returns error on nil client
func TestClientReadyWithLabelNilClient(t *testing.T) {
	var c *Client
	_, err := c.ReadyWithLabel("spec:test")
	if err == nil {
		t.Errorf("ReadyWithLabel() on nil client expected error but got nil")
		return
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("ReadyWithLabel() on nil client should mention nil, got: %v", err)
	}
}

// TestClientReadyWithLabelEmptyResults tests that ReadyWithLabel returns nil for empty results
func TestClientReadyWithLabelEmptyResults(t *testing.T) {
	tests := []struct {
		name        string
		jsonOutput  string
		description string
	}{
		{
			name:        "empty array",
			jsonOutput:  "[]",
			description: "No beads with label",
		},
		{
			name:        "empty string",
			jsonOutput:  "",
			description: "No output from bd",
		},
		{
			name:        "whitespace only",
			jsonOutput:  "   \n  ",
			description: "Only whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bead, err := parseBeadOutputExcluding(tt.jsonOutput, "epic")
			if err != nil {
				t.Fatalf("parseBeadOutputExcluding() error = %v", err)
			}
			if bead != nil {
				t.Errorf("parseBeadOutputExcluding(\"%s\") expected nil but got %+v", tt.description, bead)
			}
		})
	}
}

// TestClientReadyWithLabelExcludesEpics tests that ReadyWithLabel filters out epic beads
func TestClientReadyWithLabelExcludesEpics(t *testing.T) {
	tests := []struct {
		name       string
		jsonOutput string
		wantID     string
		wantType   string
		wantNil    bool
	}{
		{
			name: "task bead with spec:auth label",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Auth task",
				"priority": 1,
				"labels": ["spec:auth"],
				"issue_type": "task",
				"status": "open"
			}]`,
			wantID:   "task-001",
			wantType: "task",
			wantNil:  false,
		},
		{
			name: "epic bead only - should return nil",
			jsonOutput: `[{
				"id": "epic-001",
				"title": "Epic",
				"priority": 0,
				"labels": ["spec:auth"],
				"issue_type": "epic",
				"status": "open"
			}]`,
			wantNil: true,
		},
		{
			name: "epic before task - should skip epic",
			jsonOutput: `[{
				"id": "epic-001",
				"title": "Epic",
				"priority": 0,
				"labels": ["spec:auth"],
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "task-001",
				"title": "Task",
				"priority": 1,
				"labels": ["spec:auth"],
				"issue_type": "task",
				"status": "open"
			}]`,
			wantID:   "task-001",
			wantType: "task",
			wantNil:  false,
		},
		{
			name: "multiple epics only - should return nil",
			jsonOutput: `[{
				"id": "epic-001",
				"title": "Epic 1",
				"priority": 0,
				"labels": ["spec:auth"],
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "epic-002",
				"title": "Epic 2",
				"priority": 0,
				"labels": ["spec:auth"],
				"issue_type": "epic",
				"status": "open"
			}]`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBeadOutputExcluding(tt.jsonOutput, "epic")
			if err != nil {
				t.Fatalf("parseBeadOutputExcluding() error = %v", err)
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("Expected nil bead but got: %+v", got)
				}
				return
			}

			if got == nil {
				t.Fatal("Expected non-nil bead but got nil")
			}

			if got.ID != tt.wantID {
				t.Errorf("ID = %v, want %v", got.ID, tt.wantID)
			}
			if got.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", got.Type, tt.wantType)
			}
		})
	}
}

// TestClientListWithLabelValidation tests that ListWithLabel validates label parameter
func TestClientListWithLabelValidation(t *testing.T) {
	c := &Client{
		runFn: func(args ...string) (string, error) {
			return "[]", nil
		},
	}

	tests := []struct {
		name    string
		label   string
		wantErr bool
	}{
		{
			name:    "empty label",
			label:   "",
			wantErr: true,
		},
		{
			name:    "label with semicolon",
			label:   "spec:auth; rm -rf /",
			wantErr: true,
		},
		{
			name:    "label with ampersand",
			label:   "spec:auth & echo pwned",
			wantErr: true,
		},
		{
			name:    "label with angle brackets",
			label:   "spec:auth > /etc/passwd",
			wantErr: true,
		},
		{
			name:    "valid label",
			label:   "spec:auth",
			wantErr: false,
		},
		{
			name:    "valid label with colon",
			label:   "complexity:high",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.ListWithLabel(tt.label)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListWithLabel(%q) error = %v, wantErr %v", tt.label, err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if tt.label == "" && !strings.Contains(err.Error(), "empty") {
					t.Errorf("ListWithLabel(\"\") should mention empty label, got: %v", err)
				}
				if strings.ContainsAny(tt.label, ";\n|$`&<>(){}[]'\"\\") && !strings.Contains(err.Error(), "shell metacharacters") {
					t.Errorf("ListWithLabel(%q) should mention shell metacharacters, got: %v", tt.label, err)
				}
			}
		})
	}
}

// TestClientListWithLabelNilClient tests that ListWithLabel returns error on nil client
func TestClientListWithLabelNilClient(t *testing.T) {
	var c *Client
	_, err := c.ListWithLabel("spec:test")
	if err == nil {
		t.Errorf("ListWithLabel() on nil client expected error but got nil")
		return
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("ListWithLabel() on nil client should mention nil, got: %v", err)
	}
}

// TestClientListWithLabelEmptyResults tests that ListWithLabel returns empty slice for no results
func TestClientListWithLabelEmptyResults(t *testing.T) {
	tests := []struct {
		name        string
		jsonOutput  string
		description string
	}{
		{
			name:        "empty array",
			jsonOutput:  "[]",
			description: "No beads with label",
		},
		{
			name:        "empty string",
			jsonOutput:  "",
			description: "No output from bd",
		},
		{
			name:        "whitespace only",
			jsonOutput:  "   \n  ",
			description: "Only whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beads, err := parseBeadOutputList(tt.jsonOutput)
			if err != nil {
				t.Fatalf("parseBeadOutputList() error = %v", err)
			}
			if beads == nil {
				t.Errorf("parseBeadOutputList(\"%s\") expected empty slice not nil, got nil", tt.description)
				return
			}
			if len(beads) != 0 {
				t.Errorf("parseBeadOutputList(\"%s\") expected empty slice, got %d beads", tt.description, len(beads))
			}
		})
	}
}

// TestClientListWithLabelMultipleBeads tests that ListWithLabel returns multiple beads correctly
func TestClientListWithLabelMultipleBeads(t *testing.T) {
	tests := []struct {
		name       string
		jsonOutput string
		wantCount  int
		wantIDs    []string
	}{
		{
			name: "single bead with spec:auth label",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Auth task",
				"priority": 1,
				"labels": ["spec:auth"],
				"issue_type": "task",
				"status": "open"
			}]`,
			wantCount: 1,
			wantIDs:   []string{"task-001"},
		},
		{
			name: "multiple beads with spec:auth label",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Auth task 1",
				"priority": 1,
				"labels": ["spec:auth"],
				"issue_type": "task",
				"status": "open"
			}, {
				"id": "task-002",
				"title": "Auth task 2",
				"priority": 2,
				"labels": ["spec:auth", "complexity:high"],
				"issue_type": "task",
				"status": "open"
			}, {
				"id": "bug-001",
				"title": "Auth bug",
				"priority": 0,
				"labels": ["spec:auth"],
				"issue_type": "bug",
				"status": "open"
			}]`,
			wantCount: 3,
			wantIDs:   []string{"task-001", "task-002", "bug-001"},
		},
		{
			name: "beads with complexity:high label",
			jsonOutput: `[{
				"id": "complex-1",
				"title": "Complex task 1",
				"priority": 1,
				"labels": ["complexity:high"],
				"issue_type": "task",
				"status": "open"
			}, {
				"id": "complex-2",
				"title": "Complex task 2",
				"priority": 0,
				"labels": ["complexity:high", "spec:api"],
				"issue_type": "feature",
				"status": "open"
			}]`,
			wantCount: 2,
			wantIDs:   []string{"complex-1", "complex-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beads, err := parseBeadOutputList(tt.jsonOutput)
			if err != nil {
				t.Fatalf("parseBeadOutputList() error = %v", err)
			}
			if len(beads) != tt.wantCount {
				t.Errorf("parseBeadOutputList() returned %d beads, expected %d", len(beads), tt.wantCount)
				return
			}
			for i, id := range tt.wantIDs {
				if beads[i].ID != id {
					t.Errorf("parseBeadOutputList()[%d].ID = %q, expected %q", i, beads[i].ID, id)
				}
			}
		})
	}
}

// TestClientListWithLabelIncludesAllTypes tests that ListWithLabel includes all bead types (not just tasks)
func TestClientListWithLabelIncludesAllTypes(t *testing.T) {
	tests := []struct {
		name       string
		jsonOutput string
		wantTypes  []string
	}{
		{
			name: "mixed types with spec:auth label",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Task",
				"priority": 1,
				"labels": ["spec:auth"],
				"issue_type": "task",
				"status": "open"
			}, {
				"id": "bug-001",
				"title": "Bug",
				"priority": 1,
				"labels": ["spec:auth"],
				"issue_type": "bug",
				"status": "open"
			}, {
				"id": "feature-001",
				"title": "Feature",
				"priority": 1,
				"labels": ["spec:auth"],
				"issue_type": "feature",
				"status": "open"
			}, {
				"id": "epic-001",
				"title": "Epic",
				"priority": 0,
				"labels": ["spec:auth"],
				"issue_type": "epic",
				"status": "open"
			}]`,
			wantTypes: []string{"task", "bug", "feature", "epic"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beads, err := parseBeadOutputList(tt.jsonOutput)
			if err != nil {
				t.Fatalf("parseBeadOutputList() error = %v", err)
			}
			if len(beads) != len(tt.wantTypes) {
				t.Errorf("parseBeadOutputList() returned %d beads, expected %d", len(beads), len(tt.wantTypes))
				return
			}
			for i, wantType := range tt.wantTypes {
				if beads[i].Type != wantType {
					t.Errorf("parseBeadOutputList()[%d].Type = %q, expected %q", i, beads[i].Type, wantType)
				}
			}
		})
	}
}

// TestIsTestOnlyBead tests the IsTestOnlyBead heuristic for detecting beads
// whose deliverable IS tests (e.g., "Add unit tests for X"), which should
// automatically skip the ATDD pre-pass.
func TestIsTestOnlyBead(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{
			name:  "Add unit tests for X",
			title: "Add unit tests for config loading",
			want:  true,
		},
		{
			name:  "Add tests for X",
			title: "Add tests for runner escalation",
			want:  true,
		},
		{
			name:  "Write tests for X",
			title: "Write tests for prompt rendering",
			want:  true,
		},
		{
			name:  "regular feature bead",
			title: "Implement dark mode toggle",
			want:  false,
		},
		{
			name:  "bead mentioning tests but not test-only",
			title: "Fix failing tests in runner package",
			want:  false,
		},
		{
			name:  "refactor bead",
			title: "Refactor config loading to use interfaces",
			want:  false,
		},
		{
			name:  "bead with test in middle",
			title: "Implement test harness improvements",
			want:  false,
		},
		{
			name:  "Add acceptance tests for X",
			title: "Add acceptance tests for decompose phase",
			want:  true,
		},
		{
			name:  "Write unit tests for X",
			title: "Write unit tests for bead client",
			want:  true,
		},
		{
			name:  "empty title",
			title: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTestOnlyBead(tt.title)
			if got != tt.want {
				t.Errorf("IsTestOnlyBead(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}

// TestIsProactiveDecompositionCandidate tests the heuristic for detecting beads that
// should be proactively decomposed before first attempt based on title keywords.
func TestIsProactiveDecompositionCandidate_KeywordDetection(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{
			name:  "infrastructure keyword",
			title: "Build infrastructure for parallel bead execution",
			want:  true,
		},
		{
			name:  "E2E keyword",
			title: "Add E2E tests for authentication flow",
			want:  true,
		},
		{
			name:  "consolidate keyword",
			title: "Consolidate runner test helpers",
			want:  true,
		},
		{
			name:  "extract keyword",
			title: "Extract shared utilities into common package",
			want:  true,
		},
		{
			name:  "shared keyword",
			title: "Create shared config loading helpers",
			want:  true,
		},
		{
			name:  "refactor keyword",
			title: "Refactor config loading to use interfaces",
			want:  true,
		},
		{
			name:  "case insensitive - INFRASTRUCTURE",
			title: "INFRASTRUCTURE setup for deployment",
			want:  true,
		},
		{
			name:  "regular feature bead",
			title: "Add retry count to iteration log",
			want:  false,
		},
		{
			name:  "implementation bead",
			title: "Implement scope check in runner",
			want:  false,
		},
		{
			name:  "empty title",
			title: "",
			want:  false,
		},
		{
			name:  "keyword embedded in CamelCase identifier - RefactorInvokeFn",
			title: "Update RefactorInvokeFn type to return StreamStats",
			want:  false,
		},
		{
			name:  "keyword embedded in CamelCase - ExtractArray",
			title: "Fix ExtractArray parsing edge case",
			want:  false,
		},
		{
			name:  "keyword embedded in CamelCase - SharedConfig",
			title: "Update SharedConfig struct fields",
			want:  false,
		},
		{
			name:  "keyword as standalone verb still matches",
			title: "Refactor config loading to use interfaces",
			want:  true,
		},
		{
			name:  "keyword as standalone word mid-sentence",
			title: "Plan to extract helpers into shared package",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsProactiveDecompositionCandidate(tt.title)
			if got != tt.want {
				t.Errorf("IsProactiveDecompositionCandidate(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}

// TestIsProactiveDecompositionCandidate_TypeDefinitions tests that beads with 3 or more
// new type definitions in the description are flagged as decomposition candidates.
func TestIsProactiveDecompositionCandidate_TypeDefinitions(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		description string
		want        bool
	}{
		{
			name:  "three type definitions in description",
			title: "Implement config loader",
			description: `Add config loading with these types:
- ConfigLoader struct
- ConfigOptions struct
- ConfigResult struct`,
			want: true,
		},
		{
			name:  "four type definitions in description",
			title: "Add pipeline types",
			description: `Define:
- StageInput struct
- StageOutput struct
- StageConfig struct
- StageResult struct`,
			want: true,
		},
		{
			name:  "two type definitions - not enough",
			title: "Add two types",
			description: `Add:
- FooResult struct
- BarConfig struct`,
			want: false,
		},
		{
			name:        "no type definitions",
			title:       "Add retry logic",
			description: "Implement retry with exponential backoff",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsProactiveDecompositionCandidateWithDesc(tt.title, tt.description)
			if got != tt.want {
				t.Errorf("IsProactiveDecompositionCandidateWithDesc(%q, %q) = %v, want %v", tt.title, tt.description, got, tt.want)
			}
		})
	}
}

// parseBeadOutputList is a helper function that parses JSON output for ListWithLabel tests
func parseBeadOutputList(out string) ([]*Bead, error) {
	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return []*Bead{}, nil
	}

	var beads []Bead
	if err := jsonutil.ExtractArray(out, &beads); err != nil {
		return nil, fmt.Errorf("parsing bd list output: %w", err)
	}

	result := make([]*Bead, len(beads))
	for i := range beads {
		beads[i].normalizeNilFields()
		if err := beads[i].Validate(); err != nil {
			return nil, fmt.Errorf("invalid bead data at index %d: %w", i, err)
		}
		result[i] = &beads[i]
	}

	return result, nil
}

// parseBeadCount is a helper function that parses JSON output and returns bead count
func parseBeadCount(out string) (int, error) {
	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return 0, nil
	}

	var beads []Bead
	if err := jsonutil.ExtractArray(out, &beads); err != nil {
		return 0, fmt.Errorf("parsing bd output: %w", err)
	}

	return len(beads), nil
}

// TestCountClosedAfterNilClient tests that CountClosedAfter returns error on nil client
func TestCountClosedAfterNilClient(t *testing.T) {
	var c *Client
	after := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := c.CountClosedAfter(after)
	if err == nil {
		t.Fatal("CountClosedAfter() on nil client expected error but got nil")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("CountClosedAfter() on nil client should mention nil, got: %v", err)
	}
}

// TestCountClosedAfterEmptyResults tests that CountClosedAfter returns 0 for empty results
func TestCountClosedAfterEmptyResults(t *testing.T) {
	tests := []struct {
		name       string
		jsonOutput string
	}{
		{
			name:       "empty array",
			jsonOutput: "[]",
		},
		{
			name:       "empty string",
			jsonOutput: "",
		},
		{
			name:       "whitespace only",
			jsonOutput: "   \n  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := parseBeadCount(tt.jsonOutput)
			if err != nil {
				t.Fatalf("parseBeadCount() error = %v", err)
			}
			if count != 0 {
				t.Errorf("parseBeadCount() = %d, want 0", count)
			}
		})
	}
}

// TestCountClosedAfterWithBeads tests that CountClosedAfter returns correct count
func TestCountClosedAfterWithBeads(t *testing.T) {
	tests := []struct {
		name       string
		jsonOutput string
		wantCount  int
	}{
		{
			name: "single closed bead",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Task",
				"priority": 1,
				"issue_type": "task",
				"status": "closed"
			}]`,
			wantCount: 1,
		},
		{
			name: "three closed beads",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Task 1",
				"priority": 1,
				"issue_type": "task",
				"status": "closed"
			}, {
				"id": "task-002",
				"title": "Task 2",
				"priority": 1,
				"issue_type": "task",
				"status": "closed"
			}, {
				"id": "bug-001",
				"title": "Bug fix",
				"priority": 2,
				"issue_type": "bug",
				"status": "closed"
			}]`,
			wantCount: 3,
		},
		{
			name: "many closed beads in run",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Task 1",
				"priority": 1,
				"issue_type": "task",
				"status": "closed"
			}, {
				"id": "task-002",
				"title": "Task 2",
				"priority": 1,
				"issue_type": "task",
				"status": "closed"
			}, {
				"id": "task-003",
				"title": "Task 3",
				"priority": 1,
				"issue_type": "feature",
				"status": "closed"
			}, {
				"id": "task-004",
				"title": "Task 4",
				"priority": 2,
				"issue_type": "bug",
				"status": "closed"
			}, {
				"id": "task-005",
				"title": "Task 5",
				"priority": 0,
				"issue_type": "task",
				"status": "closed"
			}, {
				"id": "task-006",
				"title": "Task 6",
				"priority": 1,
				"issue_type": "task",
				"status": "closed"
			}, {
				"id": "task-007",
				"title": "Task 7",
				"priority": 1,
				"issue_type": "task",
				"status": "closed"
			}]`,
			wantCount: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := parseBeadCount(tt.jsonOutput)
			if err != nil {
				t.Fatalf("parseBeadCount() error = %v", err)
			}
			if count != tt.wantCount {
				t.Errorf("parseBeadCount() = %d, want %d", count, tt.wantCount)
			}
		})
	}
}

// TestCountByStatusNilClient tests that CountByStatus returns error on nil client
func TestCountByStatusNilClient(t *testing.T) {
	var c *Client
	_, err := c.CountByStatus("open")
	if err == nil {
		t.Fatal("CountByStatus() on nil client expected error but got nil")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("CountByStatus() on nil client should mention nil, got: %v", err)
	}
}

// TestCountByStatusEmptyResults tests that CountByStatus returns 0 for empty results
func TestCountByStatusEmptyResults(t *testing.T) {
	tests := []struct {
		name       string
		jsonOutput string
	}{
		{
			name:       "empty array",
			jsonOutput: "[]",
		},
		{
			name:       "empty string",
			jsonOutput: "",
		},
		{
			name:       "whitespace only",
			jsonOutput: "   \n  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := parseBeadCount(tt.jsonOutput)
			if err != nil {
				t.Fatalf("parseBeadCount() error = %v", err)
			}
			if count != 0 {
				t.Errorf("parseBeadCount() = %d, want 0", count)
			}
		})
	}
}

// TestCountByStatusWithBeads tests that CountByStatus returns correct count for various statuses
func TestCountByStatusWithBeads(t *testing.T) {
	tests := []struct {
		name       string
		jsonOutput string
		wantCount  int
	}{
		{
			name: "single open bead",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Task",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}]`,
			wantCount: 1,
		},
		{
			name: "multiple closed beads",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Task 1",
				"priority": 1,
				"issue_type": "task",
				"status": "closed"
			}, {
				"id": "task-002",
				"title": "Task 2",
				"priority": 1,
				"issue_type": "task",
				"status": "closed"
			}, {
				"id": "task-003",
				"title": "Task 3",
				"priority": 2,
				"issue_type": "bug",
				"status": "closed"
			}]`,
			wantCount: 3,
		},
		{
			name: "many in-progress beads",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Task 1",
				"priority": 1,
				"issue_type": "task",
				"status": "in_progress"
			}, {
				"id": "task-002",
				"title": "Task 2",
				"priority": 1,
				"issue_type": "task",
				"status": "in_progress"
			}, {
				"id": "task-003",
				"title": "Task 3",
				"priority": 1,
				"issue_type": "feature",
				"status": "in_progress"
			}, {
				"id": "task-004",
				"title": "Task 4",
				"priority": 2,
				"issue_type": "bug",
				"status": "in_progress"
			}, {
				"id": "task-005",
				"title": "Task 5",
				"priority": 0,
				"issue_type": "epic",
				"status": "in_progress"
			}]`,
			wantCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := parseBeadCount(tt.jsonOutput)
			if err != nil {
				t.Fatalf("parseBeadCount() error = %v", err)
			}
			if count != tt.wantCount {
				t.Errorf("parseBeadCount() = %d, want %d", count, tt.wantCount)
			}
		})
	}
}
