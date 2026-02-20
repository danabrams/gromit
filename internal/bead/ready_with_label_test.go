package bead

import (
	"strings"
	"testing"
)

// TestReadyWithLabel_NilClient tests that ReadyWithLabel() returns error on nil client
func TestReadyWithLabel_NilClient(t *testing.T) {
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

// TestReadyWithLabel_EmptyLabel tests that ReadyWithLabel() rejects empty label
func TestReadyWithLabel_EmptyLabel(t *testing.T) {
	c, _ := NewClient()
	_, err := c.ReadyWithLabel("")
	if err == nil {
		t.Errorf("ReadyWithLabel(\"\") expected error but got nil")
		return
	}
	if !strings.Contains(err.Error(), "label") && !strings.Contains(err.Error(), "empty") {
		t.Errorf("ReadyWithLabel(\"\") should reject empty label, got: %v", err)
	}
}

// TestReadyWithLabel_InvalidLabel tests that ReadyWithLabel() rejects invalid label characters
func TestReadyWithLabel_InvalidLabel(t *testing.T) {
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
			_, err := c.ReadyWithLabel(tt.label)
			if err == nil {
				t.Errorf("ReadyWithLabel(%q) expected validation error but got nil", tt.label)
				return
			}
			if !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "label") {
				t.Errorf("ReadyWithLabel(%q) should reject invalid label, got: %v", tt.label, err)
			}
		})
	}
}

// TestReadyWithLabel_ValidLabels tests that ReadyWithLabel() accepts valid label formats
func TestReadyWithLabel_ValidLabels(t *testing.T) {
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
			gotArgs = nil

			_, err := c.ReadyWithLabel(tt.label)
			if err != nil {
				t.Errorf("ReadyWithLabel(%q) unexpected error: %v", tt.label, err)
				return
			}
			if len(gotArgs) == 0 {
				t.Fatalf("ReadyWithLabel(%q) did not invoke RunFn", tt.label)
			}
			if gotArgs[0] != "ready" {
				t.Errorf("ReadyWithLabel(%q) command = %q, want %q", tt.label, gotArgs[0], "ready")
			}
			want := []string{"ready", "--json", "--limit", "3", "--label", tt.label}
			if !hasSubsequence(gotArgs, want) {
				t.Errorf("ReadyWithLabel(%q) args = %v, want subsequence %v", tt.label, gotArgs, want)
			}
		})
	}
}

// TestReadyWithLabel_ParsesOutput tests that ReadyWithLabel() parses bd output correctly
func TestReadyWithLabel_ParsesOutput(t *testing.T) {
	tests := []struct {
		name       string
		jsonOutput string
		wantID     string
		wantType   string
		wantNil    bool
	}{
		{
			name:       "empty array",
			jsonOutput: "[]",
			wantNil:    true,
		},
		{
			name:       "empty string",
			jsonOutput: "",
			wantNil:    true,
		},
		{
			name:       "whitespace only",
			jsonOutput: "   \n  ",
			wantNil:    true,
		},
		{
			name: "single task bead",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Test task",
				"description": "Description",
				"priority": 1,
				"labels": ["spec:auth"],
				"parent": "",
				"issue_type": "task",
				"status": "open",
				"owner": ""
			}]`,
			wantID:   "task-001",
			wantType: "task",
			wantNil:  false,
		},
		{
			name: "epic bead - should be excluded",
			jsonOutput: `[{
				"id": "epic-001",
				"title": "Epic",
				"description": "Epic description",
				"priority": 0,
				"labels": ["spec:test"],
				"parent": "",
				"issue_type": "epic",
				"status": "open",
				"owner": ""
			}]`,
			wantNil: true,
		},
		{
			name: "epic before task - should skip epic",
			jsonOutput: `[{
				"id": "epic-001",
				"title": "Epic",
				"priority": 0,
				"labels": ["spec:test"],
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "task-001",
				"title": "Task",
				"priority": 1,
				"labels": ["spec:test"],
				"issue_type": "task",
				"status": "open"
			}]`,
			wantID:   "task-001",
			wantType: "task",
			wantNil:  false,
		},
		{
			name: "multiple tasks - returns first",
			jsonOutput: `[{
				"id": "task-001",
				"title": "First task",
				"priority": 1,
				"labels": ["spec:auth"],
				"issue_type": "task",
				"status": "open"
			}, {
				"id": "task-002",
				"title": "Second task",
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
			name: "bug bead",
			jsonOutput: `[{
				"id": "bug-001",
				"title": "Bug",
				"priority": 1,
				"labels": ["spec:auth"],
				"issue_type": "bug",
				"status": "open"
			}]`,
			wantID:   "bug-001",
			wantType: "bug",
			wantNil:  false,
		},
		{
			name: "feature bead",
			jsonOutput: `[{
				"id": "feat-001",
				"title": "Feature",
				"priority": 1,
				"labels": ["spec:auth"],
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
			// Use the existing parseBeadOutputExcluding helper which should be
			// what ReadyWithLabel uses internally
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

// TestReadyWithLabel_ErrorWrapping tests that ReadyWithLabel() wraps command errors with context
func TestReadyWithLabel_ErrorWrapping(t *testing.T) {
	c, _ := NewClient()

	// Test that errors contain context when bd command fails
	_, err := c.ReadyWithLabel("spec:test")
	if err != nil && !strings.Contains(err.Error(), "bd ready") {
		t.Errorf("ReadyWithLabel() error should contain 'bd ready' context: %v", err)
	}
}

// TestReadyWithLabel_JSONParseError tests that ReadyWithLabel() handles JSON parse errors
func TestReadyWithLabel_JSONParseError(t *testing.T) {
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
			_, err := parseBeadOutputExcluding(tt.jsonOutput, "epic")
			if err == nil {
				t.Errorf("parseBeadOutputExcluding() expected error for invalid JSON, got nil")
				return
			}
			if !strings.Contains(err.Error(), "parsing") {
				t.Errorf("parseBeadOutputExcluding() error should mention parsing, got: %v", err)
			}
		})
	}
}
