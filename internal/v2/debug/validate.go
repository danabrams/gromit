package debug

import (
	"context"
	"os/exec"
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

	result := &ValidateResult{}

	// Run the validation command in the worktree
	cmd := exec.CommandContext(ctx, "sh", "-c", validCtx.ValidateCmd)
	cmd.Dir = validCtx.WorktreeRoot

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
