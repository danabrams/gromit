package pipeline

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/danabrams/gromit/internal/scope"
)

// gitCommandFn is the injectable function for constructing git subcommands.
// Tests may replace this to avoid real subprocess calls.
var reviewScopeGitCommandFn = exec.Command

// gitOutputFn is the injectable function for executing a command and capturing its stdout.
// Tests may replace this to return synthetic output.
var reviewScopeGitOutputFn = func(cmd *exec.Cmd) ([]byte, error) {
	return cmd.Output()
}

// resolveScopeGitOutput runs a git command and returns its output.
func resolveScopeGitOutput(args ...string) ([]byte, error) {
	cmd := reviewScopeGitCommandFn("git", args...)
	return reviewScopeGitOutputFn(cmd)
}

// findFirstCommitForBeadID finds the first (earliest) commit that mentions the bead ID.
// Returns empty string if no commits are found.
func findFirstCommitForBeadID(beadID string) (string, error) {
	if beadID == "" || strings.HasPrefix(beadID, "-") {
		return "", fmt.Errorf("invalid bead ID %q: must not be empty or start with '-'", beadID)
	}

	out, err := resolveScopeGitOutput("log", "--all", "--format=%H", "--grep", beadID, "--fixed-strings")
	if err != nil {
		return "", nil // No commits found - not an error
	}

	trimmedOutput := strings.TrimSpace(string(out))
	if trimmedOutput == "" {
		return "", nil
	}

	commitHashes := strings.Split(trimmedOutput, "\n")
	// git log returns newest first, so the last line is the earliest commit.
	return commitHashes[len(commitHashes)-1], nil
}

// getCommitTimestamp returns the timestamp of a commit.
func getCommitTimestamp(commit string) (int64, error) {
	if err := validateCommitRef(commit); err != nil {
		return 0, err
	}
	out, err := resolveScopeGitOutput("log", "-1", "--format=%at", commit, "--")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
}

// isCommitEarlier returns true if commit1 is earlier than commit2.
func isCommitEarlier(commit1, commit2 string) bool {
	ts1, err1 := getCommitTimestamp(commit1)
	ts2, err2 := getCommitTimestamp(commit2)
	if err1 != nil || err2 != nil {
		return false
	}
	return ts1 < ts2
}

// findEarliestCommitFromBeadIDs iterates through bead IDs and returns the earliest commit found.
// Returns empty string if no commits are found.
func findEarliestCommitFromBeadIDs(beadIDs []string) string {
	var earliestCommit string

	for _, id := range beadIDs {
		commit, err := findFirstCommitForBeadID(id)
		if err != nil || commit == "" {
			continue // Skip beads without commits
		}

		if earliestCommit == "" || isCommitEarlier(commit, earliestCommit) {
			earliestCommit = commit
		}
	}

	return earliestCommit
}

// validateCommitRef rejects refs that look like git flags (start with "-") or are empty.
func validateCommitRef(ref string) error {
	if ref == "" {
		return fmt.Errorf("commit ref must not be empty")
	}
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("invalid commit ref %q: must not start with '-'", ref)
	}
	return nil
}

// resolveSpecScope resolves the earliest commit from beads matching a spec label.
// It uses TrackerClient to query beads by the spec label.
func resolveSpecScope(ctx context.Context, specName string, trackerClient TrackerClient) (string, error) {
	if trackerClient == nil {
		return "", fmt.Errorf("tracker client is nil")
	}

	// Get the spec label
	labels := scope.ResolveSpec(specName)
	if len(labels) == 0 {
		return "", fmt.Errorf("no label found for spec %q", specName)
	}

	// Query for beads with this label
	beadIDs, err := trackerClient.ListWithLabel(ctx, labels[0])
	if err != nil {
		return "", fmt.Errorf("listing beads with label %q: %w", labels[0], err)
	}

	if len(beadIDs) == 0 {
		return "", fmt.Errorf("no beads found for spec %q - try using --since to specify a commit", specName)
	}

	// Find the earliest commit from these beads
	earliestCommit := findEarliestCommitFromBeadIDs(beadIDs)
	if earliestCommit == "" {
		return "", fmt.Errorf("no commits found for spec %q - try using --since to specify a commit", specName)
	}

	return earliestCommit, nil
}

// resolveEpicScope resolves the earliest commit from beads matching an epic's spec labels.
func resolveEpicScope(ctx context.Context, epicID string, specsDir string, trackerClient TrackerClient) (string, error) {
	if trackerClient == nil {
		return "", fmt.Errorf("tracker client is nil")
	}

	// Use scope.ResolveEpic to get spec labels for this epic
	specLabels, err := scope.ResolveEpic(epicID, specsDir)
	if err != nil {
		return "", fmt.Errorf("resolving epic %q: %w", epicID, err)
	}

	if len(specLabels) == 0 {
		return "", fmt.Errorf("no specs found for epic %q - try using --since to specify a commit", epicID)
	}

	// Collect all beads from all spec labels
	var allBeadIDs []string
	for _, label := range specLabels {
		beadIDs, err := trackerClient.ListWithLabel(ctx, label)
		if err != nil {
			return "", fmt.Errorf("listing beads with label %q: %w", label, err)
		}
		allBeadIDs = append(allBeadIDs, beadIDs...)
	}

	if len(allBeadIDs) == 0 {
		return "", fmt.Errorf("no beads found for epic %q - try using --since to specify a commit", epicID)
	}

	// Find the earliest commit from all beads
	earliestCommit := findEarliestCommitFromBeadIDs(allBeadIDs)
	if earliestCommit == "" {
		return "", fmt.Errorf("no commits found for epic %q - try using --since to specify a commit", epicID)
	}

	return earliestCommit, nil
}
