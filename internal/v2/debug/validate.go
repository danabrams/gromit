package debug

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ValidateContext describes the context for validating a fix.
type ValidateContext struct {
	WorktreeRoot string
	FailedStage  string
	ValidateCmd  string
}

// ValidateResult holds the outcome of validation.
type ValidateResult struct {
	Passed bool
	Output string
	Error  string
}

// ValidateFix runs the validation command to verify a fix passes.
func ValidateFix(ctx context.Context, validCtx *ValidateContext) (*ValidateResult, error) {
	if validCtx == nil {
		return nil, ErrNilValidateContext
	}
	if ctx == nil {
		ctx = context.Background()
	}

	worktreeRoot := strings.TrimSpace(validCtx.WorktreeRoot)
	if worktreeRoot == "" {
		return nil, ErrEmptyPath
	}

	result := &ValidateResult{}
	validateCmd := strings.TrimSpace(validCtx.ValidateCmd)
	if validateCmd == "" {
		derivedCmd, err := validationCommandForStage(validCtx.FailedStage)
		if err != nil {
			return nil, err
		}
		validateCmd = derivedCmd
	}

	// Run the validation command in the worktree
	cmd := exec.CommandContext(ctx, "sh", "-c", validateCmd)
	cmd.Dir = worktreeRoot

	output, err := cmd.CombinedOutput()
	result.Output = string(output)

	if err != nil {
		result.Passed = false
		result.Error = err.Error()
		return result, nil // Not a fatal error - just validation failed
	}

	result.Passed = true
	return result, nil
}

func validationCommandForStage(stage string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "build":
		return "go build ./...", nil
	case "validate", "test":
		return "go test ./...", nil
	case "lint":
		return "go vet ./...", nil
	default:
		return "", fmt.Errorf("no validation command configured for failed stage %q", stage)
	}
}
