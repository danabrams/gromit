package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/integrationqueue"
	"github.com/danabrams/gromit/internal/pipeline/execute"
)

// TestInvokerAdapterImplementsExecuteInvoker verifies invokerAdapter implements execute.Invoker interface.
// This test will fail compilation if invokerAdapter doesn't implement the interface.
func TestInvokerAdapterImplementsExecuteInvoker(t *testing.T) {
	t.Parallel()
	var _ execute.Invoker = (*invokerAdapter)(nil)
	t.Log("invokerAdapter implements execute.Invoker")
}

func TestIntegrationQueueGitOpsAdapterImplementsGitOps(t *testing.T) {
	t.Parallel()
	var _ integrationqueue.GitOps = (*integrationQueueGitOpsAdapter)(nil)
	t.Log("integrationQueueGitOpsAdapter implements integrationqueue.GitOps")
}
