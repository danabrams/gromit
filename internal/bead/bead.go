package bead

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"unicode"
)

// Bead represents an issue from the bd issue tracker
type Bead struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Priority        int      `json:"priority"`
	Labels          []string `json:"labels"`
	Parent          string   `json:"parent"`
	Type            string   `json:"issue_type"` // bd uses issue_type
	Status          string   `json:"status"`
	Owner           string   `json:"owner"`
	ExpectedOutputs []string `json:"expected_outputs,omitempty"`
}

// validBeadID matches alphanumeric characters, hyphens, and underscores.
var validBeadID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Maximum lengths for bead fields
const (
	maxIDLength          = 128
	maxTitleLength       = 512
	maxDescriptionLength = 16384
	maxLabelLength       = 128
	maxLabelCount        = 64
)

// normalizeNilFields ensures nil slices are replaced with empty slices.
// This prevents issues with downstream code that may range over nil slices
// (which is safe) vs code that checks len() or marshals to JSON (nil → "null"
// vs [] → "[]").
func (b *Bead) normalizeNilFields() {
	if b == nil {
		return
	}
	if b.Labels == nil {
		b.Labels = []string{}
	}
	if b.ExpectedOutputs == nil {
		b.ExpectedOutputs = []string{}
	}
}

// Validate checks that bead fields are safe for use in prompts, commands, and logging.
func (b *Bead) Validate() error {
	if b == nil {
		return fmt.Errorf("bead is nil")
	}
	// ID is used in shell commands (bd close, bd show) - must be strictly validated
	if b.ID == "" {
		return fmt.Errorf("bead has empty ID")
	}
	if len(b.ID) > maxIDLength {
		return fmt.Errorf("bead ID exceeds max length (%d > %d)", len(b.ID), maxIDLength)
	}
	if !validBeadID.MatchString(b.ID) {
		return fmt.Errorf("bead ID %q contains invalid characters (allowed: alphanumeric, hyphens, underscores)", b.ID)
	}

	// Title - enforce length limit and no control characters
	if len(b.Title) > maxTitleLength {
		return fmt.Errorf("bead title exceeds max length (%d > %d)", len(b.Title), maxTitleLength)
	}
	if err := rejectControlChars(b.Title, "title"); err != nil {
		return err
	}

	// Description - enforce length limit and no control characters
	if len(b.Description) > maxDescriptionLength {
		return fmt.Errorf("bead description exceeds max length (%d > %d)", len(b.Description), maxDescriptionLength)
	}
	if err := rejectControlChars(b.Description, "description"); err != nil {
		return err
	}

	// Labels
	if len(b.Labels) > maxLabelCount {
		return fmt.Errorf("bead has too many labels (%d > %d)", len(b.Labels), maxLabelCount)
	}
	for _, label := range b.Labels {
		if len(label) > maxLabelLength {
			return fmt.Errorf("bead label exceeds max length (%d > %d)", len(label), maxLabelLength)
		}
		if err := rejectControlChars(label, "label"); err != nil {
			return err
		}
	}

	// Parent ID - if set, must follow same rules as ID
	if b.Parent != "" {
		if len(b.Parent) > maxIDLength {
			return fmt.Errorf("bead parent ID exceeds max length (%d > %d)", len(b.Parent), maxIDLength)
		}
		if !validBeadID.MatchString(b.Parent) {
			return fmt.Errorf("bead parent ID %q contains invalid characters", b.Parent)
		}
	}

	return nil
}

// rejectControlChars returns an error if the string contains control characters
// (except newlines and tabs, which are valid in descriptions).
func rejectControlChars(s, fieldName string) error {
	for i, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return fmt.Errorf("bead %s contains control character at position %d", fieldName, i)
		}
	}
	return nil
}

// Client wraps the bd CLI
type Client struct {
	binary string
}

// NewClient creates a new bd client
func NewClient() *Client {
	return &Client{binary: "bd"}
}

// parseBeadOutput parses JSON output from a bd command that returns a bead array
// and returns the first bead after validation, or nil if no beads are present.
func parseBeadOutput(out string) (*Bead, error) {
	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return nil, nil // No work available
	}

	var beads []Bead
	if err := json.Unmarshal([]byte(out), &beads); err != nil {
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

// Ready returns the next unblocked bead ready for work (excludes epics)
func (c *Client) Ready() (*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	// Use --type task to exclude epics - we want atomic work items
	out, err := c.run("ready", "--json", "--limit", "1", "--type", "task")
	if err != nil {
		return nil, fmt.Errorf("bd ready: %w", err)
	}

	return parseBeadOutput(out)
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

// Show returns full details for a bead
func (c *Client) Show(id string) (*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	if !validBeadID.MatchString(id) || len(id) > maxIDLength {
		return nil, fmt.Errorf("invalid bead ID %q", id)
	}

	out, err := c.run("show", id, "--json")
	if err != nil {
		return nil, fmt.Errorf("bd show %s: %w", id, err)
	}

	// bd show returns an array-wrapped JSON object; try array first, fall back to single object
	trimmed := strings.TrimSpace(out)
	var b Bead
	if strings.HasPrefix(trimmed, "[") {
		var beads []Bead
		if err := json.Unmarshal([]byte(trimmed), &beads); err != nil {
			return nil, fmt.Errorf("parsing bd show output: %w", err)
		}
		if len(beads) == 0 {
			return nil, fmt.Errorf("bd show %s: empty result", id)
		}
		b = beads[0]
	} else {
		if err := json.Unmarshal([]byte(trimmed), &b); err != nil {
			return nil, fmt.Errorf("parsing bd show output: %w", err)
		}
	}

	b.normalizeNilFields()

	if err := b.Validate(); err != nil {
		return nil, fmt.Errorf("invalid bead data: %w", err)
	}

	return &b, nil
}

// Create creates a new bead via the bd CLI and returns the created bead
func (c *Client) Create(title string, priority int, labels []string, expectedOutputs []string) (*Bead, error) {
	return c.CreateWithParent(title, priority, labels, expectedOutputs, "")
}

// CreateWithParent creates a new bead with an optional parent via the bd CLI
func (c *Client) CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	args := []string{"create", title, "--priority", fmt.Sprintf("%d", priority), "--json"}

	for _, label := range labels {
		args = append(args, "--label", label)
	}

	for _, output := range expectedOutputs {
		args = append(args, "--expected-output", output)
	}

	// Add parent if specified
	if parentID != "" {
		if !validBeadID.MatchString(parentID) || len(parentID) > maxIDLength {
			return nil, fmt.Errorf("invalid parent ID %q", parentID)
		}
		args = append(args, "--parent", parentID)
	}

	out, err := c.run(args...)
	if err != nil {
		return nil, fmt.Errorf("bd create: %w", err)
	}

	var b Bead
	if err := json.Unmarshal([]byte(out), &b); err != nil {
		return nil, fmt.Errorf("parsing bd create output: %w", err)
	}

	b.normalizeNilFields()

	if err := b.Validate(); err != nil {
		return nil, fmt.Errorf("invalid bead data: %w", err)
	}

	return &b, nil
}

// Close marks a bead as complete
func (c *Client) Close(id string) error {
	if c == nil {
		return fmt.Errorf("bead client is nil")
	}
	if !validBeadID.MatchString(id) || len(id) > maxIDLength {
		return fmt.Errorf("invalid bead ID %q", id)
	}

	_, err := c.run("close", id)
	if err != nil {
		return fmt.Errorf("bd close %s: %w", id, err)
	}
	return nil
}

// Sync syncs the bd database with git
func (c *Client) Sync() error {
	if c == nil {
		return fmt.Errorf("bead client is nil")
	}
	_, err := c.run("sync")
	if err != nil {
		return fmt.Errorf("bd sync: %w", err)
	}
	return nil
}

// AddComment adds a comment to a bead
func (c *Client) AddComment(id, comment string) error {
	if c == nil {
		return fmt.Errorf("bead client is nil")
	}
	if !validBeadID.MatchString(id) || len(id) > maxIDLength {
		return fmt.Errorf("invalid bead ID %q", id)
	}

	_, err := c.run("comments", "add", id, comment)
	if err != nil {
		return fmt.Errorf("bd comments add: %w", err)
	}
	return nil
}

// GetParent returns the parent bead if one exists
func (c *Client) GetParent(b *Bead) (*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	if b == nil || b.Parent == "" {
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
