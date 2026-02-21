package bead

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/danabrams/gromit/internal/jsonutil"
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
	CloseReason     string   `json:"close_reason,omitempty"`
	Owner           string   `json:"owner"`
	ExpectedOutputs []string `json:"expected_outputs,omitempty"`
	// acceptance_criteria is used by current bd JSON responses and should be
	// treated as equivalent to expected_outputs for runner methodology logic.
	AcceptanceCriteria string       `json:"acceptance_criteria,omitempty"`
	Dependencies       []Dependency `json:"dependencies,omitempty"`
	BlockedBy          []Dependency `json:"blocked_by,omitempty"`
	DependsOn          []Dependency `json:"depends_on,omitempty"`
	DependencyCount    *int         `json:"dependency_count,omitempty"`
	DependentCount     *int         `json:"dependent_count,omitempty"`
}

// Dependency represents a dependency relation returned by bd show/list.
type Dependency struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	DependencyType string `json:"dependency_type,omitempty"`
}

// validBeadID matches alphanumeric characters, hyphens, underscores, and dots.
var validBeadID = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// Maximum lengths for bead fields
const (
	maxIDLength          = 128
	maxTitleLength       = 512
	maxDescriptionLength = 16384
	maxLabelLength       = 128
	maxLabelCount        = 64
	defaultBDBinary      = "bd"
	labelMetaChars       = ";\n|$`&<>(){}[]'\"\\"
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
	if len(b.ExpectedOutputs) == 0 && strings.TrimSpace(b.AcceptanceCriteria) != "" {
		for _, line := range strings.Split(b.AcceptanceCriteria, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			b.ExpectedOutputs = append(b.ExpectedOutputs, trimmed)
		}
	}
	if b.ExpectedOutputs == nil {
		b.ExpectedOutputs = []string{}
	}
	if b.Dependencies == nil {
		b.Dependencies = []Dependency{}
	}
	if b.BlockedBy == nil {
		b.BlockedBy = []Dependency{}
	}
	if b.DependsOn == nil {
		b.DependsOn = []Dependency{}
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
		return fmt.Errorf("bead ID %q contains invalid characters (allowed: alphanumeric, hyphens, underscores, dots)", b.ID)
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

func validateLabel(label string) error {
	if label == "" {
		return fmt.Errorf("label cannot be empty")
	}
	if strings.ContainsAny(label, labelMetaChars) {
		return fmt.Errorf("invalid label: contains shell metacharacters")
	}
	return nil
}

// Client wraps the bd CLI
type Client struct {
	binary string
	Dir    string // working directory for bd commands; if empty, uses current directory
	// RunFn optionally overrides command execution (primarily for tests).
	RunFn func(args ...string) (string, error)
}

// NewClient creates a new bd client
func NewClient() (*Client, error) {
	return &Client{binary: defaultBDBinary}, nil
}

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
		if err := jsonutil.ExtractArray(trimmed, &beads); err != nil {
			return nil, fmt.Errorf("parsing bd show output: %w", err)
		}
		if len(beads) == 0 {
			return nil, fmt.Errorf("bd show %s: empty result", id)
		}
		b = beads[0]
	} else {
		if err := jsonutil.ExtractObject(trimmed, &b); err != nil {
			return nil, fmt.Errorf("parsing bd show output: %w", err)
		}
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

	commentPath, cleanup, err := writeTempFile("bd-comment-*.txt", comment)
	if err != nil {
		return err
	}
	defer cleanup()

	_, err = c.run("comments", "add", id, "--file", commentPath)
	if err != nil {
		return fmt.Errorf("bd comments add: %w", err)
	}
	return nil
}

// Comment represents a comment on a bead
type Comment struct {
	Text      string    `json:"text"`
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
}

// GetComments retrieves all comments for a bead
func (c *Client) GetComments(id string) ([]Comment, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	if !validBeadID.MatchString(id) || len(id) > maxIDLength {
		return nil, fmt.Errorf("invalid bead ID %q", id)
	}

	out, err := c.run("comments", id, "--json")
	if err != nil {
		return nil, fmt.Errorf("bd comments %s: %w", id, err)
	}

	if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
		return []Comment{}, nil
	}

	var comments []Comment
	if err := jsonutil.ExtractArray(out, &comments); err != nil {
		return nil, fmt.Errorf("parsing bd comments output: %w", err)
	}

	return comments, nil
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

// UpdatePriority changes the priority of a bead
func (c *Client) UpdatePriority(id string, priority int) error {
	if c == nil {
		return fmt.Errorf("bead client is nil")
	}
	if !validBeadID.MatchString(id) || len(id) > maxIDLength {
		return fmt.Errorf("invalid bead ID %q", id)
	}
	if priority < 0 || priority > 4 {
		return fmt.Errorf("invalid priority %d (must be 0-4)", priority)
	}

	_, err := c.run("update", id, "--priority", fmt.Sprintf("%d", priority))
	if err != nil {
		return fmt.Errorf("bd update priority: %w", err)
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

// HasOpenChildren checks if an epic has any remaining open child tasks
func (c *Client) HasOpenChildren(parentID string) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("bead client is nil")
	}
	if !validBeadID.MatchString(parentID) || len(parentID) > maxIDLength {
		return false, fmt.Errorf("invalid parent ID %q", parentID)
	}

	// Use targeted query with --parent flag to filter server-side
	out, err := c.run("list", "--json", "--status", "open", "--parent", parentID, "--limit", "1")
	if err != nil {
		return false, err
	}

	// Parse the output - if non-empty array, parent has open children
	out = strings.TrimSpace(out)
	if out == "" || out == "[]" {
		return false, nil
	}

	// Verify it's a valid JSON array (basic sanity check)
	var beads []Bead
	if err := json.Unmarshal([]byte(out), &beads); err != nil {
		return false, fmt.Errorf("failed to parse bd output: %w", err)
	}

	return len(beads) > 0, nil
}

func (c *Client) run(args ...string) (string, error) {
	if c.RunFn != nil {
		return c.RunFn(args...)
	}
	cmd := exec.Command(c.binary, args...)
	if c.Dir != "" {
		cmd.Dir = c.Dir
	}
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%w: %s", err, string(exitErr.Stderr))
		}
		return "", err
	}
	return string(out), nil
}

func writeTempFile(pattern, content string) (string, func(), error) {
	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, fmt.Errorf("creating temp file: %w", err)
	}
	path := tmpFile.Name()

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(path)
		return "", nil, fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(path)
		return "", nil, fmt.Errorf("closing temp file: %w", err)
	}

	cleanup := func() {
		_ = os.Remove(path)
	}

	return path, cleanup, nil
}
