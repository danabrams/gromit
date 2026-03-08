package debug

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
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

	branchName := currentBranchName(ctx, worktreeRoot)

	failureCommit := strings.TrimSpace(fixCtx.FailureCommit)
	if failureCommit != "" {
		if !fixCommitHashPattern.MatchString(failureCommit) {
			return nil, fmt.Errorf("invalid failure commit %q", fixCtx.FailureCommit)
		}

		if err := checkoutFailureCommit(ctx, worktreeRoot, failureCommit, branchName, result); err != nil {
			return result, err
		}
	}

	patch := fixCtx.CodePatch
	if err := validateFilesInvolved(fixCtx.FilesInvolved, patch); err != nil {
		result.ErrorMsg = err.Error()
		return result, err
	}
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

func checkoutFailureCommit(ctx context.Context, worktreeRoot, failureCommit, branchName string, result *FixResult) error {
	if branchName != "" {
		out, err := gitCommand(ctx, worktreeRoot, "reset", "--hard", failureCommit)
		if err != nil {
			result.ErrorMsg = strings.TrimSpace(out)
			return fmt.Errorf("resetting branch %s to commit %s: %w", branchName, failureCommit, err)
		}
		return nil
	}
	out, err := gitCommand(ctx, worktreeRoot, "checkout", failureCommit)
	if err != nil {
		result.ErrorMsg = strings.TrimSpace(out)
		return fmt.Errorf("checking out failure commit %s: %w", failureCommit, err)
	}
	return nil
}

func currentBranchName(ctx context.Context, worktreeRoot string) string {
	out, err := gitCommand(ctx, worktreeRoot, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func gitCommand(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func validateFilesInvolved(files []string, patch string) error {
	if len(files) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(patch)
	if trimmed == "" {
		return nil
	}
	paths := extractPatchPaths(patch)
	if len(paths) == 0 {
		return fmt.Errorf("patch does not modify any files; expected %v", files)
	}
	for _, file := range files {
		if !patchTouchesFile(file, paths) {
			return fmt.Errorf("patch missing expected file %q", file)
		}
	}
	return nil
}

func patchTouchesFile(file string, patchPaths []string) bool {
	normalizedTarget := normalizePatchPath(file)
	if normalizedTarget == "" {
		return false
	}
	targetBase := path.Base(normalizedTarget)
	for _, raw := range patchPaths {
		normalized := normalizePatchPath(raw)
		if normalized == "" {
			continue
		}
		if normalized == normalizedTarget || normalized == targetBase || strings.HasSuffix(normalized, "/"+normalizedTarget) {
			return true
		}
		if targetBase != "" && (normalized == targetBase || strings.HasSuffix(normalized, "/"+targetBase)) {
			return true
		}
	}
	return false
}

func normalizePatchPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	normalized := filepath.ToSlash(trimmed)
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.Trim(normalized, "/")
	return strings.ToLower(normalized)
}
