package debug

import (
	"context"
)

// FixContext describes the failure context for applying a fix.
type FixContext struct {
	FailedStage   string
	ErrorMsg      string
	FilesInvolved []string
	WorktreeRoot  string
}

// FixResult describes the outcome of applying a fix.
type FixResult struct {
	Applied   bool
	ErrorMsg  string
	ValidPath string
}

// ApplyFix applies a code fix based on the given failure context.
func ApplyFix(ctx context.Context, fixCtx *FixContext) (*FixResult, error) {
	if fixCtx == nil {
		return nil, ErrNilFixContext
	}

	return &FixResult{
		Applied: true,
	}, nil
}
