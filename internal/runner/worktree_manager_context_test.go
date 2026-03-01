package runner

import (
	"context"
	"testing"
)

func TestWorktreeManagerContextualMethods(_ *testing.T) {
	type ctxOps interface {
		MergeBack(context.Context, string) error
		PendingBranches(context.Context) ([]string, error)
		RemoveByPath(context.Context, string) error
	}

	var _ ctxOps = WorktreeManager(nil)
}
