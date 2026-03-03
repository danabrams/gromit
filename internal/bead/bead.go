package bead

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/procutil"
)

// Bead represents an issue from the bd issue tracker
type Bead struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Priority        int      `json:"priority"`
	CreatedAt       string   `json:"created_at,omitempty"`
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
	minPriority          = 0
	maxPriority          = 4
	defaultBDBinary      = "bd"
	labelMetaChars       = ";\n|$`&<>(){}[]'\"\\"
)

// procutil helpers are declared as vars so tests can replace them.
// Tests must call restoreBeadProcutilFns(t) (or equivalent cleanup) when doing so
// to avoid polluting other tests. The indirection also centralizes cleanup
// helpers that need context-aware behavior during tests that exercise
// subprocess management.
var (
	waitForProcessCapacityFn  = procutil.WaitForProcessCapacity
	subprocessEnvFn           = procutil.SubprocessEnv
	killDescendantsOnCancelFn = procutil.KillDescendantsOnCancel
	reapProcessTreeFn         = procutil.ReapProcessTree
	resolveBeadsDirFn         = resolveDefaultBeadsDir
	errContextRequired        = errors.New("bead: context required")
	runWithRetryCascadeFn     = runWithRetryCascadeDefault
)

// DefaultCommandTimeout is the per-command timeout applied to bd subprocess
// invocations when no explicit CommandTimeout is configured on the Client.
const DefaultCommandTimeout = 30 * time.Second

// normalizeNilFields ensures nil slices are replaced with empty slices.
// This prevents issues with downstream code that may range over nil slices
// (which is safe) vs code that checks len() or marshals to JSON (nil -> "null"
// vs [] -> "[]").
// See CLAUDE.md nil-field normalization visibility convention:
// Bead lives in bead/, so the helper stays unexported.
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

// resolveExpectedOutputsFromAcceptanceCriteria maps legacy acceptance_criteria
// values into expected_outputs when expected_outputs is empty.
func (b *Bead) resolveExpectedOutputsFromAcceptanceCriteria() {
	if b == nil || len(b.ExpectedOutputs) > 0 || strings.TrimSpace(b.AcceptanceCriteria) == "" {
		return
	}

	for _, line := range strings.Split(b.AcceptanceCriteria, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		b.ExpectedOutputs = append(b.ExpectedOutputs, trimmed)
	}
}

func prepareBeadForUse(b *Bead) error {
	if b == nil {
		return fmt.Errorf("bead is nil")
	}

	b.normalizeNilFields()
	b.resolveExpectedOutputsFromAcceptanceCriteria()

	return b.Validate()
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
	// CommandTimeout is the maximum duration for a single bd subprocess
	// invocation. Zero means use DefaultCommandTimeout.
	CommandTimeout time.Duration
}

// NewClient creates a new bd client
func NewClient() (*Client, error) {
	return &Client{binary: defaultBDBinary}, nil
}

// NewClientWithBinary creates a bd client using the provided binary path.
// The binary path must not be empty.
func NewClientWithBinary(binary string) (*Client, error) {
	if strings.TrimSpace(binary) == "" {
		return nil, errors.New("bd binary path cannot be empty")
	}
	return &Client{binary: binary}, nil
}

// commandTimeout returns the effective per-command timeout.
func (c *Client) commandTimeout() time.Duration {
	if c.CommandTimeout > 0 {
		return c.CommandTimeout
	}
	return DefaultCommandTimeout
}

// Show returns full details for a bead
func (c *Client) Show(ctx context.Context, id string) (*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	if !validBeadID.MatchString(id) || len(id) > maxIDLength {
		return nil, fmt.Errorf("invalid bead ID %q", id)
	}

	out, err := c.run(ctx, "show", id, "--json")
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

	if err := prepareBeadForUse(&b); err != nil {
		return nil, fmt.Errorf("invalid bead data: %w", err)
	}

	return &b, nil
}

// Close marks a bead as complete
func (c *Client) Close(ctx context.Context, id string) error {
	if c == nil {
		return fmt.Errorf("bead client is nil")
	}
	if !validBeadID.MatchString(id) || len(id) > maxIDLength {
		return fmt.Errorf("invalid bead ID %q", id)
	}

	out, err := c.runClose(ctx, id)
	if err != nil {
		return fmt.Errorf("bd close %s: %w", id, err)
	}
	// bd close exits 0 even when it cannot close (e.g. blocked by open
	// dependencies). Detect this from the output so callers don't assume
	// the bead was actually closed.
	if strings.Contains(strings.ToLower(out), "cannot close") {
		return fmt.Errorf("bd close %s: %s", id, strings.TrimSpace(out))
	}
	return nil
}

// Sync syncs the bd database with git
func (c *Client) Sync(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("bead client is nil")
	}
	_, err := c.run(ctx, "sync")
	if err != nil {
		return fmt.Errorf("bd sync: %w", err)
	}
	return nil
}

// AddComment adds a comment to a bead
func (c *Client) AddComment(ctx context.Context, id, comment string) error {
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

	_, err = c.run(ctx, "comments", "add", id, "--file", commentPath)
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
func (c *Client) GetComments(ctx context.Context, id string) ([]Comment, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	if !validBeadID.MatchString(id) || len(id) > maxIDLength {
		return nil, fmt.Errorf("invalid bead ID %q", id)
	}

	out, err := c.run(ctx, "comments", id, "--json")
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

// UpdateExpectedOutputs updates the documented expected outputs for a bead
func (c *Client) UpdateExpectedOutputs(ctx context.Context, id string, criteria []string) error {
	if c == nil {
		return fmt.Errorf("bead client is nil")
	}
	if !validBeadID.MatchString(id) || len(id) > maxIDLength {
		return fmt.Errorf("invalid bead ID %q", id)
	}
	acceptance := strings.Join(criteria, "\n")
	_, err := c.run(ctx, "update", id, "--acceptance", acceptance)
	if err != nil {
		return fmt.Errorf("bd update acceptance: %w", err)
	}
	return nil
}

// UpdatePriority changes the priority of a bead
func (c *Client) UpdatePriority(ctx context.Context, id string, priority int) error {
	if c == nil {
		return fmt.Errorf("bead client is nil")
	}
	if !validBeadID.MatchString(id) || len(id) > maxIDLength {
		return fmt.Errorf("invalid bead ID %q", id)
	}
	if priority < minPriority || priority > maxPriority {
		return fmt.Errorf("invalid priority %d (must be %d-%d)", priority, minPriority, maxPriority)
	}

	_, err := c.run(ctx, "update", id, "--priority", fmt.Sprintf("%d", priority))
	if err != nil {
		return fmt.Errorf("bd update priority: %w", err)
	}
	return nil
}

// GetParent returns the parent bead if one exists
func (c *Client) GetParent(ctx context.Context, b *Bead) (*Bead, error) {
	if c == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	if b == nil || b.Parent == "" {
		return nil, nil
	}
	return c.Show(ctx, b.Parent)
}

// HasOpenChildren checks if an epic has any remaining open child tasks
func (c *Client) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("bead client is nil")
	}
	if !validBeadID.MatchString(parentID) || len(parentID) > maxIDLength {
		return false, fmt.Errorf("invalid parent ID %q", parentID)
	}

	// Use targeted query with --parent flag to filter server-side
	out, err := c.run(ctx, "list", "--json", "--status", "open", "--parent", parentID, "--limit", "1")
	if err != nil {
		return false, err
	}

	// Parse the output - if non-empty array, parent has open children
	out = strings.TrimSpace(out)
	if out == "" || out == "[]" {
		return false, nil
	}

	// Verify it's a valid JSON array (with noise-tolerant extraction).
	var beads []Bead
	if err := jsonutil.ExtractArray(out, &beads); err != nil {
		return false, fmt.Errorf("failed to parse bd output: %w", err)
	}

	return len(beads) > 0, nil
}

func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	return c.runWithRunner(ctx, args, nil, c.runWithEnv)
}

func (c *Client) runWithEnv(ctx context.Context, args []string, extraEnv []string) (string, error) {
	if err := waitForProcessCapacityFn(ctx, procutil.DefaultProcessCapacityMaxWait); err != nil {
		return "", err
	}

	cmdCtx, cancel := context.WithTimeout(ctx, c.commandTimeout())
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, c.binary, args...)
	procutil.SetProcessGroupKill(cmd)
	if c.Dir != "" {
		cmd.Dir = c.Dir
	}

	env := subprocessEnvFn()
	if len(extraEnv) > 0 {
		env = append(env, extraEnv...)
	}
	if !envHasKey(env, "BEADS_DIR") {
		if beadsDir := resolveBeadsDirFn(cmdCtx, c.Dir); beadsDir != "" {
			env = append(env, "BEADS_DIR="+beadsDir)
		}
	}
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", err
	}
	killDescendantsOnCancelFn(cmdCtx, cmd)
	defer reapProcessTreeFn(cmd)

	if err := cmd.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%w: %s", err, stderr.String())
		}
		return "", err
	}
	return stdout.String(), nil
}

func (c *Client) runClose(ctx context.Context, id string) (string, error) {
	args := []string{"close", id}
	return c.runWithRunner(ctx, args, nil, c.runWithEnvCombinedOutput)
}

func (c *Client) runWithRunner(ctx context.Context, args []string, extraEnv []string, runner func(context.Context, []string, []string) (string, error)) (string, error) {
	if c.RunFn != nil {
		return c.RunFn(args...)
	}
	return c.runWithRetryCascade(ctx, args, extraEnv, runner)
}

// runWithRetryCascade centralizes the retry cascade shared by run variants.
func (c *Client) runWithRetryCascade(ctx context.Context, args []string, extraEnv []string, runner func(context.Context, []string, []string) (string, error)) (string, error) {
	return runWithRetryCascadeFn(c, ctx, args, extraEnv, runner)
}

func runWithRetryCascadeDefault(c *Client, ctx context.Context, args []string, extraEnv []string, runner func(context.Context, []string, []string) (string, error)) (string, error) {
	out, err := runner(ctx, args, extraEnv)
	if err == nil {
		return out, nil
	}
	if shouldRetryWithNoDB(err) && !beadsNoDBAlreadyEnabled() {
		retryEnv := append([]string(nil), extraEnv...)
		retryEnv = append(retryEnv, "BEADS_NO_DB=true")
		retryOut, retryErr := runner(ctx, args, retryEnv)
		if retryErr == nil {
			return retryOut, nil
		}
		return "", fmt.Errorf("%w (retry with BEADS_NO_DB=true failed: %v)", err, retryErr)
	}
	if shouldRetryWithJSONLSync(err) {
		if _, syncErr := c.runWithEnv(ctx, []string{"init", "--from-jsonl"}, nil); syncErr != nil {
			return "", fmt.Errorf("%w (auto re-sync via 'bd init --from-jsonl' failed: %v)", err, syncErr)
		}
		retryOut, retryErr := runner(ctx, args, extraEnv)
		if retryErr == nil {
			return retryOut, nil
		}
		return "", fmt.Errorf("%w (auto re-sync via 'bd init --from-jsonl' succeeded, retry failed: %v)", err, retryErr)
	}
	if shouldRetryWithIssuePrefixBootstrap(err) {
		prefix, prefixErr := c.deriveIssuePrefix(ctx)
		if prefixErr != nil {
			deriveErr := fmt.Errorf("derive issue_prefix: %w", prefixErr)
			return "", errors.Join(err, deriveErr)
		}
		if _, setErr := c.runWithEnv(ctx, []string{"config", "set", "issue_prefix", prefix}, nil); setErr != nil {
			return "", fmt.Errorf("%w (auto-set issue_prefix=%q failed: %v)", err, prefix, setErr)
		}
		retryOut, retryErr := runner(ctx, args, extraEnv)
		if retryErr == nil {
			return retryOut, nil
		}
		return "", fmt.Errorf("%w (auto-set issue_prefix=%q succeeded, retry failed: %v)", err, prefix, retryErr)
	}
	return "", err
}

func (c *Client) runWithEnvCombinedOutput(ctx context.Context, args []string, extraEnv []string) (string, error) {
	if err := waitForProcessCapacityFn(ctx, procutil.DefaultProcessCapacityMaxWait); err != nil {
		return "", err
	}

	cmdCtx, cancel := context.WithTimeout(ctx, c.commandTimeout())
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, c.binary, args...)
	procutil.SetProcessGroupKill(cmd)
	if c.Dir != "" {
		cmd.Dir = c.Dir
	}

	env := subprocessEnvFn()
	if len(extraEnv) > 0 {
		env = append(env, extraEnv...)
	}
	if !envHasKey(env, "BEADS_DIR") {
		if beadsDir := resolveBeadsDirFn(cmdCtx, c.Dir); beadsDir != "" {
			env = append(env, "BEADS_DIR="+beadsDir)
		}
	}
	cmd.Env = env

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	if err := cmd.Start(); err != nil {
		return "", err
	}
	killDescendantsOnCancelFn(cmdCtx, cmd)
	defer reapProcessTreeFn(cmd)

	if err := cmd.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%w: %s", err, combined.String())
		}
		return "", err
	}
	return combined.String(), nil
}

func envHasKey(env []string, key string) bool {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}

func resolveDefaultBeadsDir(ctx context.Context, dir string) string {
	workingDir := strings.TrimSpace(dir)
	if workingDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return ""
		}
		workingDir = cwd
	}

	root, err := resolveCanonicalRepoRoot(ctx, workingDir)
	if err != nil {
		return ""
	}
	return filepath.Join(root, ".beads")
}

func resolveCanonicalRepoRoot(ctx context.Context, dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("working directory is empty")
	}

	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--git-common-dir")
	procutil.SetProcessGroupKill(cmd)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	defer reapProcessTreeFn(cmd)
	if err := cmd.Wait(); err != nil {
		if strings.TrimSpace(stderr.String()) != "" {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", err
	}

	commonDir := strings.TrimSpace(stdout.String())
	if commonDir == "" {
		return "", fmt.Errorf("git common dir is empty")
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Clean(filepath.Join(dir, commonDir))
	}
	if filepath.Base(commonDir) != ".git" {
		return "", fmt.Errorf("git common dir %q is not a .git directory", commonDir)
	}
	return filepath.Dir(commonDir), nil
}

func shouldRetryWithNoDB(err error) bool {
	if err == nil {
		return false
	}
	errText := err.Error()
	if strings.Contains(errText, "database not found:") {
		return true
	}
	if strings.Contains(errText, "table not found: issues") {
		return true
	}
	return strings.Contains(errText, "Error 1146") &&
		strings.Contains(errText, "table not found") &&
		strings.Contains(errText, "issues")
}

func shouldRetryWithJSONLSync(err error) bool {
	if err == nil {
		return false
	}
	errText := err.Error()
	return strings.Contains(errText, "database out of sync:") &&
		strings.Contains(errText, "issues.jsonl is newer than last import")
}

func shouldRetryWithIssuePrefixBootstrap(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "issue_prefix config is missing")
}

func (c *Client) deriveIssuePrefix(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errContextRequired
	}
	repoName, err := c.repoBaseName(ctx)
	if err != nil {
		return "", err
	}
	prefix := normalizeIssuePrefix(repoName)
	if prefix == "" {
		return "", fmt.Errorf("empty normalized prefix from %q", repoName)
	}
	return prefix, nil
}

func (c *Client) repoBaseName(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errContextRequired
	}
	timeout := DefaultCommandTimeout
	if c != nil {
		timeout = c.commandTimeout()
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := waitForProcessCapacityFn(cmdCtx, procutil.DefaultProcessCapacityMaxWait); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(cmdCtx, "git", "rev-parse", "--show-toplevel")
	procutil.SetProcessGroupKill(cmd)
	if c != nil && c.Dir != "" {
		cmd.Dir = c.Dir
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", err
	}
	killDescendantsOnCancelFn(cmdCtx, cmd)
	defer reapProcessTreeFn(cmd)

	gitErr := cmd.Wait()
	if gitErr != nil {
		if _, ok := gitErr.(*exec.ExitError); ok {
			gitErr = fmt.Errorf("%w: %s", gitErr, stderr.String())
		}
	}
	if gitErr == nil {
		root := strings.TrimSpace(stdout.String())
		if root != "" {
			return filepath.Base(root), nil
		}
	}

	if gitErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
	}

	if c != nil && c.Dir != "" {
		return filepath.Base(c.Dir), nil
	}
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		if gitErr != nil {
			return "", fmt.Errorf("git rev-parse failed: %v; getwd failed: %w", gitErr, cwdErr)
		}
		return "", fmt.Errorf("getwd failed: %w", cwdErr)
	}
	return filepath.Base(cwd), nil
}

func normalizeIssuePrefix(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func beadsNoDBAlreadyEnabled() bool {
	value, ok := os.LookupEnv("BEADS_NO_DB")
	if !ok {
		return false
	}
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "1" || value == "true" || value == "yes"
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
