package bead

import (
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/jsonutil"
)

// parseBeadOutput parses JSON output from a bd command that returns a bead array
// and returns the first bead after validation, or nil if no beads are present.
func parseBeadOutput(out string) (*Bead, error) {
	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return nil, nil // No work available
	}

	var beads []Bead
	if err := jsonutil.ExtractArray(out, &beads); err != nil {
		return nil, fmt.Errorf("parsing bd output: %w", err)
	}

	if len(beads) == 0 {
		return nil, nil
	}

	beads[0].normalizeNilFields()

	if err := beads[0].Validate(); err != nil {
		return nil, fmt.Errorf("invalid bead data: %w", err)
	}

	return &beads[0], nil
}

// parseBeadOutputExcluding parses JSON output and returns the first bead whose
// Type does not match excludeType, or nil if no matching beads are present.
func parseBeadOutputExcluding(out string, excludeType string) (*Bead, error) {
	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return nil, nil
	}

	var beads []Bead
	if err := jsonutil.ExtractArray(out, &beads); err != nil {
		return nil, fmt.Errorf("parsing bd output: %w", err)
	}

	for i := range beads {
		if beads[i].Type == excludeType {
			continue
		}
		beads[i].normalizeNilFields()
		if err := beads[i].Validate(); err != nil {
			return nil, fmt.Errorf("invalid bead data: %w", err)
		}
		return &beads[i], nil
	}

	return nil, nil
}

// Ready returns the next unblocked bead ready for work (excludes epics)
func (c *Client) Ready() (*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	// Fetch a batch and filter out epics client-side.
	// bd doesn't handle multiple -t flags correctly, so we can't exclude epics server-side.
	out, err := c.run("ready", "--json", "--limit", "3")
	if err != nil {
		return nil, fmt.Errorf("bd ready: %w", err)
	}

	return parseBeadOutputExcluding(out, "epic")
}

// ReadyExcluding returns the next unblocked non-epic bead whose ID is not in excludeIDs.
// Fetches a larger batch to increase the chance of finding a non-excluded bead.
func (c *Client) ReadyExcluding(excludeIDs map[string]bool) (*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	if len(excludeIDs) == 0 {
		return c.Ready()
	}

	out, err := c.run("ready", "--json", "--limit", "10")
	if err != nil {
		return nil, fmt.Errorf("bd ready: %w", err)
	}

	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return nil, nil
	}

	var beads []Bead
	if err := jsonutil.ExtractArray(out, &beads); err != nil {
		return nil, fmt.Errorf("parsing bd output: %w", err)
	}

	for i := range beads {
		if beads[i].Type == "epic" || excludeIDs[beads[i].ID] {
			continue
		}
		beads[i].normalizeNilFields()
		if err := beads[i].Validate(); err != nil {
			return nil, fmt.Errorf("invalid bead data: %w", err)
		}
		return &beads[i], nil
	}

	return nil, nil
}

// ReadyAny returns the next unblocked bead of any type (including epics)
func (c *Client) ReadyAny() (*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	out, err := c.run("ready", "--json", "--limit", "1")
	if err != nil {
		return nil, fmt.Errorf("bd ready: %w", err)
	}

	return parseBeadOutput(out)
}

// countBeads is a helper that runs a bd command and returns the count of beads in the result
func (c *Client) countBeads(cmdName string, args ...string) (int, error) {
	if c == nil {
		return 0, fmt.Errorf("bead client is nil")
	}

	out, err := c.run(args...)
	if err != nil {
		return 0, fmt.Errorf("bd %s: %w", cmdName, err)
	}

	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return 0, nil
	}

	var beads []Bead
	if err := jsonutil.ExtractArray(out, &beads); err != nil {
		return 0, fmt.Errorf("parsing bd %s output: %w", cmdName, err)
	}

	return len(beads), nil
}

// CountReady returns the count of ready (unblocked) beads
func (c *Client) CountReady() (int, error) {
	return c.countBeads("ready", "ready", "--json", "--limit", "0")
}

// CountByStatus returns the count of beads with the specified status
func (c *Client) CountByStatus(status string) (int, error) {
	return c.countBeads("list", "list", "--json", "--status", status, "--limit", "0")
}

// CountClosedAfter returns the count of beads closed after the specified time
func (c *Client) CountClosedAfter(after time.Time) (int, error) {
	afterStr := after.Format(time.RFC3339)
	return c.countBeads("list", "list", "--json", "--status", "closed", "--closed-after", afterStr, "--limit", "0")
}

// ListReadyIDs returns a slice of ready bead IDs (from a batch of 10)
func (c *Client) ListReadyIDs() ([]string, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	// Fetch a batch of ready beads
	out, err := c.run("ready", "--json", "--limit", "10")
	if err != nil {
		return nil, fmt.Errorf("bd ready: %w", err)
	}

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

// ReadyWithLabel returns the next unblocked bead with the specified label (excludes epics)
func (c *Client) ReadyWithLabel(label string) (*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	if err := validateLabel(label); err != nil {
		return nil, err
	}

	// Fetch a batch of beads with the specified label and filter out epics client-side
	out, err := c.run("ready", "--json", "--limit", "3", "--label", label)
	if err != nil {
		return nil, fmt.Errorf("bd ready: %w", err)
	}

	bead, err := parseBeadOutputExcluding(out, "epic")
	if err != nil || bead == nil {
		return bead, err
	}

	// Some bd versions omit labels from ready output; fetch full details when needed.
	if !HasLabel(bead.Labels, label) {
		full, err := c.Show(bead.ID)
		if err != nil {
			return nil, err
		}
		if full != nil && HasLabel(full.Labels, label) {
			return full, nil
		}
		return nil, nil
	}

	return bead, nil
}

func (c *Client) listByStatus(status string) ([]*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	out, err := c.run("list", "--json", "--status", status, "--sort", "priority", "--limit", "0")
	if err != nil {
		return nil, fmt.Errorf("bd list (%s): %w", status, err)
	}

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

// List returns all open beads, sorted by priority (P0 first)
func (c *Client) List() ([]*Bead, error) {
	return c.listByStatus("open")
}

// ListByStatus returns all beads with the given status, sorted by priority (P0 first).
func (c *Client) ListByStatus(status string) ([]*Bead, error) {
	if strings.TrimSpace(status) == "" {
		return nil, fmt.Errorf("status cannot be empty")
	}
	return c.listByStatus(status)
}

// ListReady returns all ready (unblocked) beads, sorted by priority (P0 first).
func (c *Client) ListReady() ([]*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	out, err := c.run("list", "--json", "--status", "ready", "--sort", "priority", "--limit", "0")
	if err != nil {
		return nil, fmt.Errorf("bd list ready: %w", err)
	}

	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return []*Bead{}, nil
	}

	var beads []Bead
	if err := jsonutil.ExtractArray(out, &beads); err != nil {
		return nil, fmt.Errorf("parsing bd list ready output: %w", err)
	}

	result := make([]*Bead, len(beads))
	for i := range beads {
		beads[i].normalizeNilFields()
		if err := beads[i].Validate(); err != nil {
			return nil, fmt.Errorf("invalid ready bead data at index %d: %w", i, err)
		}
		result[i] = &beads[i]
	}

	return result, nil
}

// ListReadyWork returns all ready work based on bd ready semantics.
func (c *Client) ListReadyWork() ([]*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	out, err := c.run("ready", "--json", "--sort", "priority", "--limit", "0")
	if err != nil {
		return nil, fmt.Errorf("bd ready: %w", err)
	}

	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return []*Bead{}, nil
	}

	var beads []Bead
	if err := jsonutil.ExtractArray(out, &beads); err != nil {
		return nil, fmt.Errorf("parsing bd ready output: %w", err)
	}

	result := make([]*Bead, len(beads))
	for i := range beads {
		beads[i].normalizeNilFields()
		if err := beads[i].Validate(); err != nil {
			return nil, fmt.Errorf("invalid ready bead data at index %d: %w", i, err)
		}
		result[i] = &beads[i]
	}

	return result, nil
}

// ListAll returns all beads (both open and closed), grouped by status
func (c *Client) ListAll() (open []*Bead, closed []*Bead, err error) {
	if c == nil {
		return nil, nil, fmt.Errorf("bead client is nil")
	}

	// Initialize to empty slices to avoid nil returns
	open = []*Bead{}
	closed = []*Bead{}

	// Get open beads
	out, err := c.run("list", "--json", "--status", "open")
	if err != nil {
		return nil, nil, fmt.Errorf("bd list open: %w", err)
	}

	if strings.TrimSpace(out) != "" && strings.TrimSpace(out) != "[]" {
		var beads []Bead
		if err := jsonutil.ExtractArray(out, &beads); err != nil {
			return nil, nil, fmt.Errorf("parsing bd list open output: %w", err)
		}

		open = make([]*Bead, len(beads))
		for i := range beads {
			beads[i].normalizeNilFields()
			if err := beads[i].Validate(); err != nil {
				return nil, nil, fmt.Errorf("invalid open bead data at index %d: %w", i, err)
			}
			open[i] = &beads[i]
		}
	}

	// Get closed beads
	out, err = c.run("list", "--json", "--status", "closed")
	if err != nil {
		return nil, nil, fmt.Errorf("bd list closed: %w", err)
	}

	if strings.TrimSpace(out) != "" && strings.TrimSpace(out) != "[]" {
		var beads []Bead
		if err := jsonutil.ExtractArray(out, &beads); err != nil {
			return nil, nil, fmt.Errorf("parsing bd list closed output: %w", err)
		}

		closed = make([]*Bead, len(beads))
		for i := range beads {
			beads[i].normalizeNilFields()
			if err := beads[i].Validate(); err != nil {
				return nil, nil, fmt.Errorf("invalid closed bead data at index %d: %w", i, err)
			}
			closed[i] = &beads[i]
		}
	}

	return open, closed, nil
}

// ListWithLabel returns all beads with the specified label
func (c *Client) ListWithLabel(label string) ([]*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	if err := validateLabel(label); err != nil {
		return nil, err
	}

	out, err := c.run("list", "--json", "--label", label, "--sort", "priority", "--all", "--limit", "0")
	if err != nil {
		return nil, fmt.Errorf("bd list: %w", err)
	}

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
