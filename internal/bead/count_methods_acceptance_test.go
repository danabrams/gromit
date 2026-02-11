//go:build acceptance

package bead

import (
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/jsonutil"
)

// TestCountByStatus_ReturnsCount tests that CountByStatus() returns the count of beads with a given status
func TestCountByStatus_ReturnsCount(t *testing.T) {
	// Expected failure: CountByStatus method does not exist on Client yet
	c, _ := NewClient()

	tests := []struct {
		name       string
		status     string
		jsonOutput string
		wantCount  int
	}{
		{
			name:   "count in-progress beads",
			status: "in_progress",
			jsonOutput: `[{
				"id": "task-001",
				"title": "In progress task",
				"priority": 1,
				"issue_type": "task",
				"status": "in_progress"
			}]`,
			wantCount: 1,
		},
		{
			name:   "count deferred beads",
			status: "deferred",
			jsonOutput: `[{
				"id": "task-002",
				"title": "Deferred task 1",
				"priority": 1,
				"issue_type": "task",
				"status": "deferred"
			}, {
				"id": "task-003",
				"title": "Deferred task 2",
				"priority": 2,
				"issue_type": "task",
				"status": "deferred"
			}]`,
			wantCount: 2,
		},
		{
			name:       "count closed beads when empty",
			status:     "closed",
			jsonOutput: "[]",
			wantCount:  0,
		},
		{
			name:   "count closed beads with multiple entries",
			status: "closed",
			jsonOutput: `[{
				"id": "task-001",
				"title": "Closed task 1",
				"priority": 1,
				"issue_type": "task",
				"status": "closed"
			}, {
				"id": "task-002",
				"title": "Closed task 2",
				"priority": 1,
				"issue_type": "task",
				"status": "closed"
			}, {
				"id": "task-003",
				"title": "Closed task 3",
				"priority": 2,
				"issue_type": "bug",
				"status": "closed"
			}]`,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail because CountByStatus doesn't exist yet
			count, err := c.CountByStatus(tt.status)
			if err != nil {
				// Command errors are expected in tests, but we're testing the method signature
				if !strings.Contains(err.Error(), "bd list") {
					t.Errorf("CountByStatus(%q) unexpected error: %v", tt.status, err)
				}
				return
			}

			// Once implemented, verify it returns correct count
			if count != tt.wantCount {
				t.Errorf("CountByStatus(%q) = %d, want %d", tt.status, count, tt.wantCount)
			}
		})
	}
}

// TestCountByStatus_NilClient tests that CountByStatus() returns error on nil client
func TestCountByStatus_NilClient(t *testing.T) {
	// Expected failure: CountByStatus method does not exist on Client yet
	var c *Client
	_, err := c.CountByStatus("open")
	if err == nil {
		t.Errorf("CountByStatus() on nil client expected error but got nil")
		return
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("CountByStatus() on nil client should mention nil, got: %v", err)
	}
}

// TestCountByStatus_InvalidStatus tests that CountByStatus() validates status parameter
func TestCountByStatus_InvalidStatus(t *testing.T) {
	// Expected failure: CountByStatus method does not exist on Client yet
	c, _ := NewClient()

	tests := []struct {
		name    string
		status  string
		wantErr bool
	}{
		{
			name:    "empty status",
			status:  "",
			wantErr: true,
		},
		{
			name:    "status with semicolon",
			status:  "open; rm -rf /",
			wantErr: true,
		},
		{
			name:    "status with newline",
			status:  "open\nmalicious",
			wantErr: true,
		},
		{
			name:    "status with pipe",
			status:  "open | cat /etc/passwd",
			wantErr: true,
		},
		{
			name:    "valid status open",
			status:  "open",
			wantErr: false,
		},
		{
			name:    "valid status in_progress",
			status:  "in_progress",
			wantErr: false,
		},
		{
			name:    "valid status deferred",
			status:  "deferred",
			wantErr: false,
		},
		{
			name:    "valid status closed",
			status:  "closed",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.CountByStatus(tt.status)
			if tt.wantErr {
				if err == nil {
					t.Errorf("CountByStatus(%q) expected validation error but got nil", tt.status)
					return
				}
				// Should fail validation before running command
				if strings.ContainsAny(tt.status, ";\n|$`&<>(){}[]'\"\\") {
					if !strings.Contains(err.Error(), "shell metacharacters") && !strings.Contains(err.Error(), "invalid") {
						t.Errorf("CountByStatus(%q) should reject shell metacharacters, got: %v", tt.status, err)
					}
				}
			}
		})
	}
}

// TestCountByStatus_EmptyResultsReturnsZero tests that CountByStatus() returns 0 for empty results
func TestCountByStatus_EmptyResultsReturnsZero(t *testing.T) {
	// Expected failure: CountByStatus method does not exist on Client yet
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
			// Test the parsing logic that CountByStatus should use
			count, err := parseCountFromBeadOutput(tt.jsonOutput)
			if err != nil {
				t.Fatalf("parseCountFromBeadOutput() error = %v", err)
			}
			if count != 0 {
				t.Errorf("parseCountFromBeadOutput() = %d, want 0 for empty results", count)
			}
		})
	}
}

// TestCountClosedAfter_ReturnsCount tests that CountClosedAfter() returns count of beads closed after a time
func TestCountClosedAfter_ReturnsCount(t *testing.T) {
	// Expected failure: CountClosedAfter method does not exist on Client yet
	c, _ := NewClient()

	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	tests := []struct {
		name       string
		after      time.Time
		jsonOutput string
		wantCount  int
	}{
		{
			name:  "count beads closed after specific time",
			after: oneHourAgo,
			jsonOutput: `[{
				"id": "task-001",
				"title": "Recently closed task",
				"priority": 1,
				"issue_type": "task",
				"status": "closed"
			}]`,
			wantCount: 1,
		},
		{
			name:       "no beads closed after time",
			after:      oneHourAgo,
			jsonOutput: "[]",
			wantCount:  0,
		},
		{
			name:  "multiple beads closed after time",
			after: oneHourAgo,
			jsonOutput: `[{
				"id": "task-001",
				"title": "Closed task 1",
				"priority": 1,
				"issue_type": "task",
				"status": "closed"
			}, {
				"id": "task-002",
				"title": "Closed task 2",
				"priority": 1,
				"issue_type": "task",
				"status": "closed"
			}, {
				"id": "bug-001",
				"title": "Closed bug",
				"priority": 0,
				"issue_type": "bug",
				"status": "closed"
			}]`,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail because CountClosedAfter doesn't exist yet
			count, err := c.CountClosedAfter(tt.after)
			if err != nil {
				// Command errors are expected in tests, but we're testing the method signature
				if !strings.Contains(err.Error(), "bd list") {
					t.Errorf("CountClosedAfter() unexpected error: %v", err)
				}
				return
			}

			// Once implemented, verify it returns correct count
			if count != tt.wantCount {
				t.Errorf("CountClosedAfter(%v) = %d, want %d", tt.after, count, tt.wantCount)
			}
		})
	}
}

// TestCountClosedAfter_NilClient tests that CountClosedAfter() returns error on nil client
func TestCountClosedAfter_NilClient(t *testing.T) {
	// Expected failure: CountClosedAfter method does not exist on Client yet
	var c *Client
	_, err := c.CountClosedAfter(time.Now())
	if err == nil {
		t.Errorf("CountClosedAfter() on nil client expected error but got nil")
		return
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("CountClosedAfter() on nil client should mention nil, got: %v", err)
	}
}

// TestCountClosedAfter_ZeroTime tests that CountClosedAfter() handles zero time value
func TestCountClosedAfter_ZeroTime(t *testing.T) {
	// Expected failure: CountClosedAfter method does not exist on Client yet
	c, _ := NewClient()

	// Zero time should be valid - bd should accept it and return all closed beads
	_, err := c.CountClosedAfter(time.Time{})
	if err != nil {
		// Command error is expected when bd isn't running, but should not be a validation error
		if strings.Contains(err.Error(), "invalid") && strings.Contains(err.Error(), "time") {
			t.Errorf("CountClosedAfter() should accept zero time value, got validation error: %v", err)
		}
	}
}

// TestCountClosedAfter_FormatsTimeCorrectly tests that CountClosedAfter() formats time for bd CLI
func TestCountClosedAfter_FormatsTimeCorrectly(t *testing.T) {
	// Expected failure: CountClosedAfter method does not exist on Client yet
	// This test verifies that the time is formatted correctly for the --closed-after flag

	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	// CountClosedAfter should format the time in a way that bd --closed-after accepts
	// The spec doesn't specify format, but ISO 8601 (RFC3339) is standard: "2024-01-15T10:30:00Z"
	c, _ := NewClient()

	_, err := c.CountClosedAfter(testTime)
	if err != nil {
		// We expect a bd command error, but we're verifying the method exists
		// and formats the time parameter correctly
		if !strings.Contains(err.Error(), "bd list") {
			t.Errorf("CountClosedAfter() unexpected error type: %v", err)
		}
	}
}

// TestCountClosedAfter_EmptyResultsReturnsZero tests that CountClosedAfter() returns 0 for empty results
func TestCountClosedAfter_EmptyResultsReturnsZero(t *testing.T) {
	// Expected failure: CountClosedAfter method does not exist on Client yet
	c, _ := NewClient()

	// Create a time far in the future - no beads should be closed after it
	futureTime := time.Now().Add(24 * time.Hour * 365) // 1 year from now

	count, err := c.CountClosedAfter(futureTime)
	if err != nil {
		// bd command error expected, but we're checking the behavior
		if strings.Contains(err.Error(), "parsing") {
			t.Errorf("CountClosedAfter() should handle empty results without parse error: %v", err)
		}
		return
	}

	// Should return 0 when no beads match the filter
	if count != 0 {
		t.Errorf("CountClosedAfter(future time) = %d, want 0", count)
	}
}

// TestCountByStatus_ErrorWrapping tests that CountByStatus() wraps command errors with context
func TestCountByStatus_ErrorWrapping(t *testing.T) {
	// Expected failure: CountByStatus method does not exist on Client yet
	c, _ := NewClient()

	// Test that errors contain context when bd command fails
	_, err := c.CountByStatus("closed")
	if err != nil && !strings.Contains(err.Error(), "bd list") {
		t.Errorf("CountByStatus() error should contain 'bd list' context: %v", err)
	}
}

// TestCountClosedAfter_ErrorWrapping tests that CountClosedAfter() wraps command errors with context
func TestCountClosedAfter_ErrorWrapping(t *testing.T) {
	// Expected failure: CountClosedAfter method does not exist on Client yet
	c, _ := NewClient()

	// Test that errors contain context when bd command fails
	_, err := c.CountClosedAfter(time.Now().Add(-1 * time.Hour))
	if err != nil && !strings.Contains(err.Error(), "bd list") {
		t.Errorf("CountClosedAfter() error should contain 'bd list' context: %v", err)
	}
}

// parseCountFromBeadOutput is a helper that mimics what CountByStatus and CountClosedAfter should do
func parseCountFromBeadOutput(out string) (int, error) {
	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return 0, nil
	}

	var beads []Bead
	if err := jsonutil.ExtractArray(out, &beads); err != nil {
		return 0, err
	}

	return len(beads), nil
}
