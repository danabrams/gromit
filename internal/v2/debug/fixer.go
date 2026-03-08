package debug

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// FixContext describes the failure context for applying a fix.
type FixContext struct {
	FailedStage   string
	ErrorMsg      string
	FilesInvolved []string
	WorktreeRoot  string
	FailureCommit string
	CodePatch     string
}

// FixResult describes the outcome of applying a fix.
type FixResult struct {
	Applied   bool
	ErrorMsg  string
	ValidPath string
}

var fixCommitHashPattern = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)

// ApplyFix applies a code fix based on the given failure context.
func ApplyFix(ctx context.Context, fixCtx *FixContext) (*FixResult, error) {
	if fixCtx == nil {
		return nil, ErrNilFixContext
	}
	if ctx == nil {
		ctx = context.Background()
	}
	worktreeRoot := strings.TrimSpace(fixCtx.WorktreeRoot)
	if worktreeRoot == "" {
		return nil, ErrEmptyPath
	}

	result := &FixResult{
		ValidPath: worktreeRoot,
	}

	failureCommit := strings.TrimSpace(fixCtx.FailureCommit)
	if failureCommit != "" {
		if !fixCommitHashPattern.MatchString(failureCommit) {
			return nil, fmt.Errorf("invalid failure commit %q", fixCtx.FailureCommit)
		}

		checkout := exec.CommandContext(ctx, "git", "checkout", failureCommit)
		checkout.Dir = worktreeRoot
		out, err := checkout.CombinedOutput()
		if err != nil {
			result.ErrorMsg = strings.TrimSpace(string(out))
			return result, fmt.Errorf("checking out failure commit %s: %w", failureCommit, err)
		}
	}

	patch := fixCtx.CodePatch
	if strings.TrimSpace(patch) != "" {
		apply := exec.CommandContext(ctx, "git", "apply", "--whitespace=nowarn", "-")
		apply.Dir = worktreeRoot
		apply.Stdin = strings.NewReader(patch)
		out, err := apply.CombinedOutput()
		if err != nil {
			result.ErrorMsg = strings.TrimSpace(string(out))
			return result, fmt.Errorf("applying code patch: %w\n%s", err, strings.TrimSpace(string(out)))
		}
	}

	result.Applied = true
	return result, nil
}
