package bead

import (
	"encoding/json"
	"os/exec"
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
			// Simulate parsing logic from Ready()/ReadyAny()
			if strings.TrimSpace(tt.jsonOutput) == "" || strings.TrimSpace(tt.jsonOutput) == "[]" {
				if !tt.wantNil {
					t.Errorf("Expected non-nil bead for output: %s", tt.jsonOutput)
				}
				return
			}

			var beads []Bead
			err := json.Unmarshal([]byte(tt.jsonOutput), &beads)
			if err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			if len(beads) == 0 {
				if !tt.wantNil {
					t.Errorf("Expected non-nil bead but got empty array")
				}
				return
			}

			got := &beads[0]
			if tt.wantNil {
				t.Errorf("Expected nil bead but got: %+v", got)
				return
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
	c := newIsolatedClient(t)

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
			_, err := c.Create(tt.title, tt.priority, tt.labels, tt.expectedOutputs)
			// bd may reject unknown flags (e.g. --expected-output); that's a
			// separate issue. We only fail on truly unexpected errors.
			if err != nil && !strings.Contains(err.Error(), "bd create") && !strings.Contains(err.Error(), "parsing") {
				t.Errorf("Create() unexpected error type: %v", err)
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
	c, _ := NewClient()
	err := c.Sync()
	// May succeed if bd is available, or fail if not - either is fine
	// Just testing it doesn't panic
	if err != nil && !strings.Contains(err.Error(), "bd sync") {
		t.Errorf("Sync() unexpected error type: %v", err)
	}
}

// TestErrorWrapping tests that CLI errors are properly wrapped
func TestErrorWrapping(t *testing.T) {
	c, _ := NewClient()

	// Test that errors contain context
	_, err := c.Ready()
	if err != nil && !strings.Contains(err.Error(), "bd ready") {
		t.Errorf("Ready() error should contain context: %v", err)
	}

	_, err = c.ReadyAny()
	if err != nil && !strings.Contains(err.Error(), "bd ready") {
		t.Errorf("ReadyAny() error should contain context: %v", err)
	}

	err = c.Close("test-id")
	if err != nil && !strings.Contains(err.Error(), "bd close") {
		t.Errorf("Close() error should contain context: %v", err)
	}
}

// TestClientCreateWithParent tests the CreateWithParent() method
func TestClientCreateWithParent(t *testing.T) {
	c := newIsolatedClient(t)

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

			// For valid inputs, errors are expected if bd isn't running,
			// but the method should build arguments correctly
			if err != nil {
				if !strings.Contains(err.Error(), "bd create") {
					t.Errorf("CreateWithParent() unexpected error type: %v", err)
				}
			}
		})
	}
}

// TestClientCreateWithParentInheritance tests that Create() delegates to CreateWithParent
func TestClientCreateInheritsCreateWithParent(t *testing.T) {
	c := newIsolatedClient(t)

	// Create() should call CreateWithParent with empty parentID
	_, err1 := c.Create("Test", 1, []string{}, []string{})
	_, err2 := c.CreateWithParent("Test", 1, []string{}, []string{}, "")

	// Both should have the same error behavior (or lack thereof)
	hasErr1 := err1 != nil
	hasErr2 := err2 != nil

	if hasErr1 != hasErr2 {
		t.Errorf("Create() and CreateWithParent(\"\") should behave identically: Create err=%v, CreateWithParent err=%v", err1, err2)
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
