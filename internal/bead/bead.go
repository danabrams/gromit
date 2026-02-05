package bead

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Bead represents an issue from the bd issue tracker
type Bead struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    int      `json:"priority"`
	Labels      []string `json:"labels"`
	Parent      string   `json:"parent"`
	Type        string   `json:"issue_type"` // bd uses issue_type
	Status      string   `json:"status"`
	Owner       string   `json:"owner"`
}

// Client wraps the bd CLI
type Client struct {
	binary string
}

// NewClient creates a new bd client
func NewClient() *Client {
	return &Client{binary: "bd"}
}

// Ready returns the next unblocked bead ready for work (excludes epics)
func (c *Client) Ready() (*Bead, error) {
	// Use --type task to exclude epics - we want atomic work items
	out, err := c.run("ready", "--json", "--limit", "1", "--type", "task")
	if err != nil {
		return nil, fmt.Errorf("bd ready: %w", err)
	}

	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return nil, nil // No work available
	}

	var beads []Bead
	if err := json.Unmarshal([]byte(out), &beads); err != nil {
		return nil, fmt.Errorf("parsing bd ready output: %w", err)
	}

	if len(beads) == 0 {
		return nil, nil
	}

	return &beads[0], nil
}

// ReadyAny returns the next unblocked bead of any type (including epics)
func (c *Client) ReadyAny() (*Bead, error) {
	out, err := c.run("ready", "--json", "--limit", "1")
	if err != nil {
		return nil, fmt.Errorf("bd ready: %w", err)
	}

	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return nil, nil
	}

	var beads []Bead
	if err := json.Unmarshal([]byte(out), &beads); err != nil {
		return nil, fmt.Errorf("parsing bd ready output: %w", err)
	}

	if len(beads) == 0 {
		return nil, nil
	}

	return &beads[0], nil
}

// Show returns full details for a bead
func (c *Client) Show(id string) (*Bead, error) {
	out, err := c.run("show", id, "--json")
	if err != nil {
		return nil, fmt.Errorf("bd show %s: %w", id, err)
	}

	var bead Bead
	if err := json.Unmarshal([]byte(out), &bead); err != nil {
		return nil, fmt.Errorf("parsing bd show output: %w", err)
	}

	return &bead, nil
}

// Close marks a bead as complete
func (c *Client) Close(id string) error {
	_, err := c.run("close", id)
	if err != nil {
		return fmt.Errorf("bd close %s: %w", id, err)
	}
	return nil
}

// Sync syncs the bd database with git
func (c *Client) Sync() error {
	_, err := c.run("sync")
	if err != nil {
		return fmt.Errorf("bd sync: %w", err)
	}
	return nil
}

// AddComment adds a comment to a bead
func (c *Client) AddComment(id, comment string) error {
	_, err := c.run("comments", "add", id, comment)
	if err != nil {
		return fmt.Errorf("bd comments add: %w", err)
	}
	return nil
}

// GetParent returns the parent bead if one exists
func (c *Client) GetParent(b *Bead) (*Bead, error) {
	if b.Parent == "" {
		return nil, nil
	}
	return c.Show(b.Parent)
}

// FindSpecLabel returns the spec name from labels (spec:<name>) or empty string
func FindSpecLabel(labels []string) string {
	for _, label := range labels {
		if strings.HasPrefix(label, "spec:") {
			return strings.TrimPrefix(label, "spec:")
		}
	}
	return ""
}

// HasLabel checks if a bead has a specific label
func HasLabel(labels []string, target string) bool {
	for _, label := range labels {
		if label == target {
			return true
		}
	}
	return false
}

func (c *Client) run(args ...string) (string, error) {
	cmd := exec.Command(c.binary, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%w: %s", err, string(exitErr.Stderr))
		}
		return "", err
	}
	return string(out), nil
}
