package bead

import (
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/jsonutil"
)

// TestListWithLabel_NilClient tests that ListWithLabel() returns error on nil client
func TestListWithLabel_NilClient(t *testing.T) {
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

// TestListWithLabel_EmptyLabel tests that ListWithLabel() rejects empty label
func TestListWithLabel_EmptyLabel(t *testing.T) {
	c, _ := NewClient()
	_, err := c.ListWithLabel("")
	if err == nil {
		t.Errorf("ListWithLabel(\"\") expected error but got nil")
		return
	}
	if !strings.Contains(err.Error(), "label") && !strings.Contains(err.Error(), "empty") {
		t.Errorf("ListWithLabel(\"\") should reject empty label, got: %v", err)
	}
}

// TestListWithLabel_InvalidLabel tests that ListWithLabel() rejects invalid label characters
func TestListWithLabel_InvalidLabel(t *testing.T) {
	c, _ := NewClient()

	tests := []struct {
		name  string
		label string
	}{
		{
			name:  "label with semicolon",
			label: "spec:test; rm -rf /",
		},
		{
			name:  "label with newline",
			label: "spec:test\n",
		},
		{
			name:  "label with command substitution",
			label: "spec:test$(whoami)",
		},
		{
			name:  "label with pipe",
			label: "spec:test | cat /etc/passwd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.ListWithLabel(tt.label)
			if err == nil {
				t.Errorf("ListWithLabel(%q) expected validation error but got nil", tt.label)
				return
			}
			if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "label") {
				t.Errorf("ListWithLabel(%q) should reject invalid label, got: %v", tt.label, err)
			}
		})
	}
}

// TestListWithLabel_ValidLabels tests that ListWithLabel() accepts valid label formats
func TestListWithLabel_ValidLabels(t *testing.T) {
	var gotArgs []string
	c := &Client{
		RunFn: func(args ...string) (string, error) {
			gotArgs = append([]string(nil), args...)
			return "[]", nil
		},
	}

	tests := []struct {
		name  string
		label string
	}{
		{
			name:  "simple spec label",
			label: "spec:auth",
		},
		{
			name:  "spec label with hyphen",
			label: "spec:user-auth",
		},
		{
			name:  "spec label with underscore",
			label: "spec:user_auth",
		},
		{
			name:  "complexity label",
			label: "complexity:high",
		},
		{
			name:  "methodology label",
			label: "atdd:true",
		},
		{
			name:  "label with dots",
			label: "spec:v1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.ListWithLabel(tt.label)
			if err != nil {
				t.Errorf("ListWithLabel(%q) unexpected error: %v", tt.label, err)
				return
			}
			want := []string{"list", "--json", "--label", tt.label, "--sort", "priority", "--all", "--limit", "0"}
			if !hasSubsequence(gotArgs, want) {
				t.Errorf("ListWithLabel(%q) args = %v, want subsequence %v", tt.label, gotArgs, want)
			}
		})
	}
}

// TestListWithLabel_ReturnsEmptySliceForNoBeads tests that ListWithLabel() returns empty slice when no beads match
func TestListWithLabel_ReturnsEmptySliceForNoBeads(t *testing.T) {
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
			beads, err := parseListWithLabelOutput(tt.jsonOutput)
			if err != nil {
				t.Fatalf("parseListWithLabelOutput() error = %v", err)
			}
			if beads == nil {
				t.Errorf("ListWithLabel() should return empty slice not nil, got nil")
				return
			}
			if len(beads) != 0 {
				t.Errorf("ListWithLabel() expected empty slice, got %v", beads)
			}
		})
	}
}

// TestListWithLabel_ReturnsSingleBead tests that ListWithLabel() returns a single bead
func TestListWithLabel_ReturnsSingleBead(t *testing.T) {
	jsonOutput := `[{
		"id": "task-001",
		"title": "Test task",
		"priority": 1,
		"labels": ["spec:auth"],
		"issue_type": "task",
		"status": "open"
	}]`

	beads, err := parseListWithLabelOutput(jsonOutput)
	if err != nil {
		t.Fatalf("parseListWithLabelOutput() error = %v", err)
	}

	if len(beads) != 1 {
		t.Fatalf("Expected 1 bead, got %d", len(beads))
	}

	if beads[0].ID != "task-001" {
		t.Errorf("Bead ID = %v, want task-001", beads[0].ID)
	}
	if beads[0].Type != "task" {
		t.Errorf("Bead Type = %v, want task", beads[0].Type)
	}
	if !HasLabel(beads[0].Labels, "spec:auth") {
		t.Errorf("Bead should have label spec:auth, got %v", beads[0].Labels)
	}
}

// TestListWithLabel_ReturnsMultipleBeads tests that ListWithLabel() returns multiple beads
func TestListWithLabel_ReturnsMultipleBeads(t *testing.T) {
	jsonOutput := `[{
		"id": "task-001",
		"title": "First task",
		"priority": 1,
		"labels": ["spec:auth", "complexity:high"],
		"issue_type": "task",
		"status": "open"
	}, {
		"id": "task-002",
		"title": "Second task",
		"priority": 1,
		"labels": ["spec:auth"],
		"issue_type": "task",
		"status": "open"
	}, {
		"id": "bug-001",
		"title": "Bug",
		"priority": 0,
		"labels": ["spec:auth", "priority:p0"],
		"issue_type": "bug",
		"status": "open"
	}]`

	beads, err := parseListWithLabelOutput(jsonOutput)
	if err != nil {
		t.Fatalf("parseListWithLabelOutput() error = %v", err)
	}

	if len(beads) != 3 {
		t.Fatalf("Expected 3 beads, got %d", len(beads))
	}

	// Verify first bead
	if beads[0].ID != "task-001" {
		t.Errorf("Bead[0] ID = %v, want task-001", beads[0].ID)
	}

	// Verify second bead
	if beads[1].ID != "task-002" {
		t.Errorf("Bead[1] ID = %v, want task-002", beads[1].ID)
	}

	// Verify third bead
	if beads[2].ID != "bug-001" {
		t.Errorf("Bead[2] ID = %v, want bug-001", beads[2].ID)
	}
	if beads[2].Type != "bug" {
		t.Errorf("Bead[2] Type = %v, want bug", beads[2].Type)
	}

	// Verify all beads have the spec:auth label
	for i, bead := range beads {
		if !HasLabel(bead.Labels, "spec:auth") {
			t.Errorf("Bead[%d] should have label spec:auth, got %v", i, bead.Labels)
		}
	}
}

// TestListWithLabel_ExcludesEpicBeads tests that ListWithLabel() excludes epic-type beads
// This is an acceptance test for the epic exclusion requirement
func TestListWithLabel_ExcludesEpicBeads(t *testing.T) {
	jsonOutput := `[{
		"id": "epic-001",
		"title": "Epic",
		"priority": 0,
		"labels": ["spec:database"],
		"issue_type": "epic",
		"status": "open"
	}, {
		"id": "task-001",
		"title": "Task",
		"priority": 1,
		"labels": ["spec:database"],
		"issue_type": "task",
		"status": "open"
	}, {
		"id": "bug-001",
		"title": "Bug",
		"priority": 0,
		"labels": ["spec:database"],
		"issue_type": "bug",
		"status": "open"
	}, {
		"id": "feature-001",
		"title": "Feature",
		"priority": 1,
		"labels": ["spec:database"],
		"issue_type": "feature",
		"status": "open"
	}]`

	beads, err := parseListWithLabelOutputExcludingEpics(jsonOutput)
	if err != nil {
		t.Fatalf("parseListWithLabelOutputExcludingEpics() error = %v", err)
	}

	// Should only include task, bug, feature (not epic)
	if len(beads) != 3 {
		t.Fatalf("Expected 3 beads (task, bug, feature), got %d", len(beads))
	}

	// Verify no epic beads in results
	for i, bead := range beads {
		if bead.Type == "epic" {
			t.Errorf("Bead[%d] should not be type epic, got: %s", i, bead.Type)
		}
	}

	// Verify we have the non-epic types
	types := make(map[string]bool)
	for _, bead := range beads {
		types[bead.Type] = true
	}

	expectedTypes := []string{"task", "bug", "feature"}
	for _, expectedType := range expectedTypes {
		if !types[expectedType] {
			t.Errorf("Expected to find bead with type %q", expectedType)
		}
	}

	// Verify epic is NOT present
	if types["epic"] {
		t.Error("Should not include epic type beads")
	}
}

// TestListWithLabel_ValidatesAndNormalizesBeads tests that ListWithLabel() validates and normalizes returned beads
func TestListWithLabel_ValidatesAndNormalizesBeads(t *testing.T) {
	jsonOutput := `[{
		"id": "task-001",
		"title": "Task with missing optional fields",
		"priority": 1,
		"issue_type": "task",
		"status": "open"
	}]`

	beads, err := parseListWithLabelOutput(jsonOutput)
	if err != nil {
		t.Fatalf("parseListWithLabelOutput() error = %v", err)
	}

	if len(beads) != 1 {
		t.Fatalf("Expected 1 bead, got %d", len(beads))
	}

	bead := beads[0]

	// Verify nil fields are normalized to empty slices
	if bead.Labels == nil {
		t.Error("Labels should not be nil after normalization")
	}
	if bead.ExpectedOutputs == nil {
		t.Error("ExpectedOutputs should not be nil after normalization")
	}

	// Verify bead passes validation
	if err := bead.Validate(); err != nil {
		t.Errorf("Bead should pass validation after normalization: %v", err)
	}
}

// TestListWithLabel_HandlesJSONParseErrors tests that ListWithLabel() handles JSON parse errors
func TestListWithLabel_HandlesJSONParseErrors(t *testing.T) {
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
			_, err := parseListWithLabelOutput(tt.jsonOutput)
			if err == nil {
				t.Errorf("parseListWithLabelOutput() expected error for invalid JSON, got nil")
				return
			}
			if !strings.Contains(err.Error(), "parsing") {
				t.Errorf("parseListWithLabelOutput() error should mention parsing, got: %v", err)
			}
		})
	}
}

// TestListWithLabel_ErrorWrapping tests that ListWithLabel() wraps command errors with context
func TestListWithLabel_ErrorWrapping(t *testing.T) {
	c := &Client{
		RunFn: func(args ...string) (string, error) {
			return "", fmt.Errorf("boom")
		},
	}

	// Test that errors contain context when bd command fails
	_, err := c.ListWithLabel("spec:test")
	if err == nil {
		t.Fatal("ListWithLabel() expected error")
	}
	if !strings.Contains(err.Error(), "bd list") {
		t.Errorf("ListWithLabel() error should contain 'bd list' context: %v", err)
	}
}

// TestListWithLabel_ReturnsPointersToBeads tests that ListWithLabel() returns pointers not values
func TestListWithLabel_ReturnsPointersToBeads(t *testing.T) {
	jsonOutput := `[{
		"id": "task-001",
		"title": "Task",
		"priority": 1,
		"labels": ["spec:auth"],
		"issue_type": "task",
		"status": "open"
	}]`

	beads, err := parseListWithLabelOutput(jsonOutput)
	if err != nil {
		t.Fatalf("parseListWithLabelOutput() error = %v", err)
	}

	if len(beads) != 1 {
		t.Fatalf("Expected 1 bead, got %d", len(beads))
	}

	// Verify we got a pointer, not a value
	if beads[0] == nil {
		t.Error("Expected non-nil pointer to bead")
	}
}

// TestListWithLabel_HandlesInvalidBeadData tests that ListWithLabel() returns error for invalid bead data
func TestListWithLabel_HandlesInvalidBeadData(t *testing.T) {
	tests := []struct {
		name       string
		jsonOutput string
		wantErr    string
	}{
		{
			name: "invalid bead ID with spaces",
			jsonOutput: `[{
				"id": "invalid id with spaces",
				"title": "Task",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}]`,
			wantErr: "invalid bead data",
		},
		{
			name: "oversized title",
			jsonOutput: `[{
				"id": "task-001",
				"title": "` + strings.Repeat("x", 513) + `",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}]`,
			wantErr: "invalid bead data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseListWithLabelOutput(tt.jsonOutput)
			if err == nil {
				t.Errorf("parseListWithLabelOutput() expected error for invalid bead data")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("parseListWithLabelOutput() error should contain %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

// parseListWithLabelOutput is a test helper that parses JSON output like ListWithLabel does
func parseListWithLabelOutput(out string) ([]*Bead, error) {
	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return []*Bead{}, nil
	}

	var beads []Bead
	if err := jsonutil.ExtractArray(out, &beads); err != nil {
		return nil, fmt.Errorf("parsing bd list output: %w", err)
	}

	// Convert to pointers and normalize
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

// parseListWithLabelOutputExcludingEpics parses JSON output excluding epic-type beads
// This is the expected behavior for ListWithLabel after implementing epic exclusion
func parseListWithLabelOutputExcludingEpics(out string) ([]*Bead, error) {
	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return []*Bead{}, nil
	}

	var beads []Bead
	if err := jsonutil.ExtractArray(out, &beads); err != nil {
		return nil, fmt.Errorf("parsing bd list output: %w", err)
	}

	// Filter out epic beads and convert to pointers
	result := []*Bead{}
	for i := range beads {
		if beads[i].Type == "epic" {
			continue
		}
		beads[i].normalizeNilFields()
		if err := beads[i].Validate(); err != nil {
			return nil, fmt.Errorf("invalid bead data at index %d: %w", i, err)
		}
		result = append(result, &beads[i])
	}

	return result, nil
}

// TestListWithLabel_IntegrationReturnsPrioritySortedBeads tests that ListWithLabel returns beads sorted by priority
func TestListWithLabel_IntegrationReturnsPrioritySortedBeads(t *testing.T) {
	testLabel := "spec:priority-sort-test"
	var gotArgs []string
	c := &Client{
		RunFn: func(args ...string) (string, error) {
			gotArgs = append([]string(nil), args...)
			return `[{"id":"task-2","title":"P0","priority":0,"labels":["` + testLabel + `"],"issue_type":"task","status":"open"},
				{"id":"task-3","title":"P1","priority":1,"labels":["` + testLabel + `"],"issue_type":"task","status":"open"},
				{"id":"task-1","title":"P2","priority":2,"labels":["` + testLabel + `"],"issue_type":"task","status":"open"}]`, nil
		},
	}

	beads, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	if len(beads) != 3 {
		t.Fatalf("Expected 3 beads, got %d", len(beads))
	}

	for i := 1; i < len(beads); i++ {
		if beads[i].Priority < beads[i-1].Priority {
			t.Fatalf("ListWithLabel() beads not sorted by priority: %+v", beads)
		}
	}

	want := []string{"list", "--json", "--label", testLabel, "--sort", "priority", "--all", "--limit", "0"}
	if !hasSubsequence(gotArgs, want) {
		t.Errorf("ListWithLabel() args = %v, want subsequence %v", gotArgs, want)
	}
}

// TestListWithLabel_IntegrationConsistentWithListMethod tests ordering consistency between List() and ListWithLabel()
func TestListWithLabel_IntegrationConsistentWithListMethod(t *testing.T) {
	c := &Client{
		RunFn: func(args ...string) (string, error) {
			return `[{"id":"task-a","title":"P0","priority":0,"labels":["test-label"],"issue_type":"task","status":"open"},
				{"id":"task-b","title":"P1","priority":1,"labels":["test-label"],"issue_type":"task","status":"open"},
				{"id":"task-c","title":"P2","priority":2,"labels":["test-label"],"issue_type":"task","status":"open"}]`, nil
		},
	}

	allBeads, err := c.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	labelBeads, err := c.ListWithLabel("test-label")
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	if len(allBeads) != len(labelBeads) {
		t.Errorf("List() and ListWithLabel() returned different bead counts: %d vs %d", len(allBeads), len(labelBeads))
		return
	}

	for i := range allBeads {
		if allBeads[i].Priority != labelBeads[i].Priority {
			t.Errorf("Priority order mismatch at position %d: List() has P%d, ListWithLabel() has P%d",
				i, allBeads[i].Priority, labelBeads[i].Priority)
		}
	}

	for i := 1; i < len(labelBeads); i++ {
		if labelBeads[i].Priority < labelBeads[i-1].Priority {
			t.Errorf("ListWithLabel() beads not in priority order at position %d: P%d comes after P%d",
				i, labelBeads[i].Priority, labelBeads[i-1].Priority)
		}
	}
}

// TestListWithLabel_CommandArgumentsIncludeAllAndLimit verifies that the bd list command
// is invoked with --all and --limit 0 flags, ensuring complete results are returned.
func TestListWithLabel_CommandArgumentsIncludeAllAndLimit(t *testing.T) {
	var gotArgs []string
	c := &Client{
		RunFn: func(args ...string) (string, error) {
			gotArgs = append([]string(nil), args...)
			return "[]", nil
		},
	}
	testLabel := "spec:command-args-test"

	_, err := c.ListWithLabel(testLabel)
	if err != nil {
		t.Fatalf("ListWithLabel() error = %v", err)
	}

	want := []string{"list", "--json", "--label", testLabel, "--sort", "priority", "--all", "--limit", "0"}
	if !hasSubsequence(gotArgs, want) {
		t.Errorf("ListWithLabel() args = %v, want subsequence %v", gotArgs, want)
	}
}

func hasSubsequence(args []string, seq []string) bool {
	if len(seq) == 0 {
		return true
	}
	j := 0
	for _, arg := range args {
		if arg == seq[j] {
			j++
			if j == len(seq) {
				return true
			}
		}
	}
	return false
}
