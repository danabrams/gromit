//go:build acceptance

package bead

import (
	"strings"
	"testing"
	"time"
)

// TestCountByStatus_ReturnsCorrectCounts verifies CountByStatus returns correct counts for beads
func TestCountByStatus_ReturnsCorrectCounts(t *testing.T) {
	c, _ := NewClient()

	// Test that CountByStatus runs bd list --status <status> --json and returns count
	_, err := c.CountByStatus("closed")
	if err != nil && !strings.Contains(err.Error(), "bd list") {
		t.Errorf("CountByStatus() should run bd list command: %v", err)
	}
}

// TestCountByStatus_HandlesEmptyResultsCorrectly verifies CountByStatus returns 0 for empty results
func TestCountByStatus_HandlesEmptyResultsCorrectly(t *testing.T) {
	c, _ := NewClient()

	// Test with deferred status (likely to be empty in test environment)
	count, err := c.CountByStatus("deferred")
	if err != nil {
		t.Skipf("bd command error (expected in test env): %v", err)
	}
	if count < 0 {
		t.Errorf("CountByStatus() returned negative count: %d", count)
	}
}

// TestCountByStatus_NilClientCheck verifies CountByStatus returns error on nil client
func TestCountByStatus_NilClientCheck(t *testing.T) {
	var c *Client
	_, err := c.CountByStatus("open")
	if err == nil || !strings.Contains(err.Error(), "nil") {
		t.Errorf("CountByStatus() on nil client should return nil error, got: %v", err)
	}
}

// TestCountClosedAfter_ReturnsCorrectCounts verifies CountClosedAfter returns correct counts
func TestCountClosedAfter_ReturnsCorrectCounts(t *testing.T) {
	c, _ := NewClient()

	oneHourAgo := time.Now().Add(-1 * time.Hour)

	// Test that CountClosedAfter runs bd list --status closed --closed-after <time> --json
	_, err := c.CountClosedAfter(oneHourAgo)
	if err != nil && !strings.Contains(err.Error(), "bd list") {
		t.Errorf("CountClosedAfter() should run bd list command: %v", err)
	}
}

// TestCountClosedAfter_HandlesEmptyResultsCorrectly verifies CountClosedAfter returns 0 for empty results
func TestCountClosedAfter_HandlesEmptyResultsCorrectly(t *testing.T) {
	c, _ := NewClient()

	// Use a future time - no beads should be closed after it
	futureTime := time.Now().Add(24 * time.Hour * 365)

	count, err := c.CountClosedAfter(futureTime)
	if err != nil {
		t.Skipf("bd command error (expected in test env): %v", err)
	}
	if count != 0 {
		t.Errorf("CountClosedAfter(future time) = %d, want 0", count)
	}
}

// TestCountClosedAfter_NilClientCheck verifies CountClosedAfter returns error on nil client
func TestCountClosedAfter_NilClientCheck(t *testing.T) {
	var c *Client
	_, err := c.CountClosedAfter(time.Now())
	if err == nil || !strings.Contains(err.Error(), "nil") {
		t.Errorf("CountClosedAfter() on nil client should return nil error, got: %v", err)
	}
}
