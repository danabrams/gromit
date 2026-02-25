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

func runReviewTagOutput(args ...string) ([]byte, error) {
	cmd := reviewTagCommandFn("git", args...)
	return reviewTagOutputFn(cmd)
}

// CreateReviewTag creates a git tag gromit/interactive-review/<timestamp> pointing
// at the given commit. The timestamp uses the format YYYY-MM-DDTHH-MM-SS.
func CreateReviewTag(commit string) error {
	tag := reviewTagPrefix + time.Now().Format("2006-01-02T15-04-05")
	_, err := runReviewTagOutput("tag", tag, commit)
	if err != nil {
		return fmt.Errorf("creating review tag %s: %w", tag, err)
	}
	return nil
}

// LatestReviewTagCommit finds the most recent gromit/interactive-review/* tag and
// returns the commit hash it points to. Returns ("", nil) when no review tags exist.
func LatestReviewTagCommit() (string, error) {
	out, err := runReviewTagOutput("tag", "-l", "--sort=-creatordate", reviewTagPrefix+"*")
	if err != nil {
		return "", fmt.Errorf("listing review tags: %w", err)
	}

	tags := strings.TrimSpace(string(out))
	if tags == "" {
		return "", nil
	}

	// First line is the most recent tag.
	latestTag := strings.SplitN(tags, "\n", 2)[0]

	commitOut, err := runReviewTagOutput("rev-list", "-1", latestTag)
	if err != nil {
		return "", fmt.Errorf("resolving tag %s: %w", latestTag, err)
	}

	return strings.TrimSpace(string(commitOut)), nil
}
