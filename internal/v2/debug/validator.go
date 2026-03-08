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
	StageTrace   *StageTrace
}

// StageCommitter commits structured stage artifacts.
type StageCommitter interface {
	CommitStage(ctx context.Context, worktree, beadID, stageName string, iteration int, decision string) error
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

	commands, err := validationCommands(validCtx)
	if err != nil {
		return nil, err
	}

	result := &ValidateResult{}
	outputs := make([]string, 0, len(commands))
	for _, validateCmd := range commands {
		trimmed := strings.TrimSpace(validateCmd)
		if trimmed == "" {
			continue
		}
		cmd := exec.CommandContext(ctx, "sh", "-c", trimmed)
		cmd.Dir = worktreeRoot
		output, cmdErr := cmd.CombinedOutput()
		outputs = append(outputs, strings.TrimSpace(string(output)))
		if cmdErr != nil {
			result.Output = strings.TrimSpace(strings.Join(outputs, "\n"))
			result.Passed = false
			result.Error = cmdErr.Error()
			return result, nil
		}
	}

	result.Output = strings.TrimSpace(strings.Join(outputs, "\n"))
	result.Passed = true
	return result, nil
}

// ValidateAndCommitFix reruns the failed stage validation and commits the fix when it succeeds.
func ValidateAndCommitFix(ctx context.Context, validCtx *ValidateContext, committer StageCommitter) (*ValidateResult, error) {
	result, err := ValidateFix(ctx, validCtx)
	if err != nil {
		return nil, err
	}
	if !result.Passed || committer == nil {
		return result, nil
	}
	if validCtx.StageTrace == nil {
		return result, fmt.Errorf("stage trace required for validated commit")
	}
	stageName := strings.TrimSpace(validCtx.StageTrace.StageName)
	if stageName == "" {
		return result, fmt.Errorf("stage name missing for validated commit")
	}
	iteration := validCtx.StageTrace.Iteration
	if iteration <= 0 {
		iteration = 1
	} else {
		iteration++
	}
	beadID := strings.TrimSpace(validCtx.StageTrace.BeadID)
	err = committer.CommitStage(ctx, validCtx.WorktreeRoot, beadID, stageName, iteration, "Proceed")
	if err != nil {
		return result, err
	}
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

func validationCommands(validCtx *ValidateContext) ([]string, error) {
	if validCtx == nil {
		return nil, ErrNilValidateContext
	}
	if cmd := strings.TrimSpace(validCtx.ValidateCmd); cmd != "" {
		return []string{cmd}, nil
	}
	commands := make([]string, 0)
	if vt := validCtx.StageTrace; vt != nil && vt.Validation != nil {
		for _, entry := range vt.Validation.Commands {
			if trimmed := strings.TrimSpace(entry); trimmed != "" {
				commands = append(commands, trimmed)
			}
		}
	}
	if len(commands) > 0 {
		return commands, nil
	}
	stageName := validationStageName(validCtx)
	if stageName == "" {
		return nil, fmt.Errorf("stage name required to derive validation command")
	}
	cmd, err := validationCommandForStage(stageName)
	if err != nil {
		return nil, err
	}
	return []string{cmd}, nil
}

func validationStageName(validCtx *ValidateContext) string {
	if validCtx == nil {
		return ""
	}
	if vt := validCtx.StageTrace; vt != nil {
		if name := strings.TrimSpace(vt.StageName); name != "" {
			return name
		}
	}
	return strings.TrimSpace(validCtx.FailedStage)
}
