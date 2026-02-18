package escalation

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestInvokeFn_UsesRuntypesInvocationResult(t *testing.T) {
	var _ InvokeFn = func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
		return nil, nil
	}
}
