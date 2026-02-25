package state

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const reviewTagPrefix = "gromit/interactive-review/"

// reviewTagCommandFn is the injectable function for constructing git subcommands.
// Tests may replace this to avoid real subprocess calls.
var reviewTagCommandFn = exec.Command

// reviewTagOutputFn is the injectable function for executing a command and
// capturing its stdout. Tests may replace this to return synthetic output.
var reviewTagOutputFn = func(cmd *exec.Cmd) ([]byte, error) {
	return cmd.Output()
}

func runReviewTagOutputInDir(repoDir string, args ...string) ([]byte, error) {
	cmd := reviewTagCommandFn("git", args...)
	if repoDir != "" {
		cmd.Dir = repoDir
	}
	return reviewTagOutputFn(cmd)
}

// CreateReviewTag creates a git tag gromit/interactive-review/<timestamp> pointing
// at the given commit. The timestamp uses the format YYYY-MM-DDTHH-MM-SS.
func CreateReviewTag(commit string) error {
	return CreateReviewTagInRepo("", commit)
}

// CreateReviewTagInRepo creates a review tag in the provided repository directory.
// If repoDir is empty, git runs in the current process working directory.
func CreateReviewTagInRepo(repoDir, commit string) error {
	if strings.TrimSpace(commit) == "" {
		return fmt.Errorf("creating review tag: commit cannot be empty")
	}
	// Use nanoseconds to avoid collisions when multiple reviews are recorded in the same second.
	tag := fmt.Sprintf("%s%d", reviewTagPrefix, time.Now().UTC().UnixNano())
	_, err := runReviewTagOutputInDir(repoDir, "tag", tag, commit)
	if err != nil {
		return fmt.Errorf("creating review tag %s: %w", tag, err)
	}
	return nil
}

// LatestReviewTagCommit finds the most recent gromit/interactive-review/* tag and
// returns the commit hash it points to. Returns ("", nil) when no review tags exist.
func LatestReviewTagCommit() (string, error) {
	return LatestReviewTagCommitInRepo("")
}

// LatestReviewTagCommitInRepo finds the most recent review tag in repoDir.
// If repoDir is empty, git runs in the current process working directory.
func LatestReviewTagCommitInRepo(repoDir string) (string, error) {
	out, err := runReviewTagOutputInDir(repoDir, "tag", "-l", "--sort=-creatordate", reviewTagPrefix+"*")
	if err != nil {
		return "", fmt.Errorf("listing review tags: %w", err)
	}

	tags := strings.TrimSpace(string(out))
	if tags == "" {
		return "", nil
	}

	// First line is the most recent tag.
	latestTag := strings.SplitN(tags, "\n", 2)[0]

	commitOut, err := runReviewTagOutputInDir(repoDir, "rev-list", "-1", latestTag)
	if err != nil {
		return "", fmt.Errorf("resolving tag %s: %w", latestTag, err)
	}

	return strings.TrimSpace(string(commitOut)), nil
}
