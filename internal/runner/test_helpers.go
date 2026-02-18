package runner

import (
	"io"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/execution"
)

// test_helpers.go provides shared test utilities for the runner package.
// Mock provider implementations are in mock_provider_test.go.

// newInvokerForTest creates an execution.Invoker wired to a *provider.Router.
// Used by tests that construct Runner structs directly with &Runner{}.
func newInvokerForTest(router *provider.Router, output io.Writer, sl *logger.StreamLogger) *execution.Invoker {
	if router == nil {
		return execution.NewInvoker(nil, output, sl)
	}
	return execution.NewInvoker(&routerAdapter{r: router}, output, sl)
}

func stubGitHeadFn() func() (string, error) {
	return func() (string, error) {
		return "abc123", nil
	}
}
