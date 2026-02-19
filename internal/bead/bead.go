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

// Client wraps the bd CLI
type Client struct {
	binary string
	Dir    string // working directory for bd commands; if empty, uses current directory
	RunFn  func(args ...string) (string, error)
	runFn  func(args ...string) (string, error)
}

// NewClient creates a new bd client
func NewClient() (*Client, error) {
	return &Client{binary: "bd"}, nil
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
	if label == "" {
		return nil, fmt.Errorf("label cannot be empty")
	}
	// Validate label doesn't contain shell metacharacters
	if strings.ContainsAny(label, ";\n|$`&<>(){}[]'\"\\") {
		return nil, fmt.Errorf("invalid label: contains shell metacharacters")
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

// Create creates a new bead via the bd CLI and returns the created bead
func (c *Client) Create(title string, priority int, labels []string, expectedOutputs []string) (*Bead, error) {
	return c.CreateWithParentAndDescription(title, priority, labels, expectedOutputs, "", "")
}

// CreateWithParent creates a new bead with an optional parent and description via the bd CLI
func (c *Client) CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*Bead, error) {
	return c.CreateWithParentAndDescription(title, priority, labels, expectedOutputs, parentID, "")
}

// CreateWithParentAndDescription creates a new bead with an optional parent and description via the bd CLI
func (c *Client) CreateWithParentAndDescription(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}

	// Add parent if specified
	var extra []string
	if parentID != "" {
		if !validBeadID.MatchString(parentID) || len(parentID) > maxIDLength {
			return nil, fmt.Errorf("invalid parent ID %q", parentID)
		}
		extra = append(extra, "--parent", parentID)
	}

	return c.runCreate(title, priority, labels, expectedOutputs, description, extra)
}

// CreateWithDepsAndDescription creates a new bead with dependencies and description via the bd CLI
func (c *Client) CreateWithDepsAndDescription(title string, priority int, labels []string, expectedOutputs []string, dependencies []string, description string) (*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}

	// Add dependencies if specified
	var extra []string
	if len(dependencies) > 0 {
		// Validate all dependency IDs
		for _, depID := range dependencies {
			if !validBeadID.MatchString(depID) || len(depID) > maxIDLength {
				return nil, fmt.Errorf("invalid dependency ID %q", depID)
			}
		}
		// bd --deps accepts comma-separated list or multiple IDs
		extra = append(extra, "--deps", strings.Join(dependencies, ","))
	}

	return c.runCreate(title, priority, labels, expectedOutputs, description, extra)
}

// runCreate builds args, writes description content to a temp file, invokes bd create, and parses the result.
// extraArgs are inserted after acceptance (e.g. --parent or --deps flags).
func (c *Client) runCreate(title string, priority int, labels []string, expectedOutputs []string, description string, extraArgs []string) (*Bead, error) {
	args := []string{"create", title, "--priority", fmt.Sprintf("%d", priority), "--json"}

	for _, label := range labels {
		args = append(args, "--label", label)
	}

	if len(expectedOutputs) > 0 {
		acceptance := strings.Join(expectedOutputs, "\n")
		args = append(args, "--acceptance", acceptance)
	}

	args = append(args, extraArgs...)

	if description != "" {
		descriptionPath, cleanup, err := writeTempFile("bd-description-*.md", description)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		args = append(args, "--body-file", descriptionPath)
	}

	out, err := c.run(args...)
	if err != nil {
		return nil, fmt.Errorf("bd create: %w", err)
	}

	var b Bead
	if err := jsonutil.ExtractObject(out, &b); err != nil {
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

// List returns all open beads, sorted by priority (P0 first)
func (c *Client) List() ([]*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	out, err := c.run("list", "--json", "--status", "open", "--sort", "priority", "--limit", "0")
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
	if label == "" {
		return nil, fmt.Errorf("label cannot be empty")
	}
	// Validate label doesn't contain shell metacharacters
	if strings.ContainsAny(label, ";\n|$`&<>(){}[]'\"\\") {
		return nil, fmt.Errorf("invalid label: contains shell metacharacters")
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

// IsTestOnlyBead returns true if the bead's title indicates that tests ARE the deliverable
// (e.g., "Add unit tests for X", "Write tests for Y"). Such beads should skip the ATDD
// pre-pass since acceptance tests are the implementation itself.
func IsTestOnlyBead(title string) bool {
	t := strings.ToLower(strings.TrimSpace(title))
	if t == "" {
		return false
	}
	// Match titles where the primary verb is about writing/adding tests
	prefixes := []string{
		"add tests for",
		"add unit tests for",
		"add acceptance tests for",
		"add integration tests for",
		"write tests for",
		"write unit tests for",
		"write acceptance tests for",
		"write integration tests for",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(t, prefix) {
			return true
		}
	}
	return false
}

// proactiveDecomposeKeywords matches broad-scope keywords as whole words only.
// This prevents false positives on identifiers like "RefactorInvokeFn" or "ExtractArray"
// where the keyword is embedded in a CamelCase name rather than used as a verb/noun.
var proactiveDecomposeKeywords = regexp.MustCompile(
	`(?i)\b(infrastructure|e2e|consolidate|extract|shared|refactor)\b`,
)

// IsProactiveDecompositionCandidate returns true if the bead's title contains keywords
// that signal broad scope and should trigger proactive decomposition before first attempt.
// Keywords must appear as whole words — "Refactor the auth system" matches but
// "Update RefactorInvokeFn" does not.
func IsProactiveDecompositionCandidate(title string) bool {
	t := strings.TrimSpace(title)
	if t == "" {
		return false
	}
	return proactiveDecomposeKeywords.MatchString(t)
}

// IsProactiveDecompositionCandidateWithDesc returns true if the bead should be proactively
// decomposed before first attempt, based on title keywords OR a description that mentions
// "struct" 3+ times (used as a proxy for introducing 3+ new type definitions).
func IsProactiveDecompositionCandidateWithDesc(title, description string) bool {
	if IsProactiveDecompositionCandidate(title) {
		return true
	}
	// Count "struct" occurrences in description as a proxy for new type definitions
	count := strings.Count(strings.ToLower(description), "struct")
	return count >= 3
}

// IsMethodologyActive checks if a methodology (e.g., "atdd", "tdd") is active for a bead.
// It checks for a label like "atdd:true" or "atdd:false" and returns that value if present.
// If no matching label is found, it falls back to the globalDefault value.
func IsMethodologyActive(labels []string, methodologyName string, globalDefault bool) bool {
	trueLabel := methodologyName + ":true"
	falseLabel := methodologyName + ":false"

	for _, label := range labels {
		if label == trueLabel {
			return true
		}
		if label == falseLabel {
			return false
		}
	}

	return globalDefault
}

func (c *Client) run(args ...string) (string, error) {
	if c.RunFn != nil {
		return c.RunFn(args...)
	}
	if c.runFn != nil {
		return c.runFn(args...)
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
