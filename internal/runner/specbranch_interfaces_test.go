package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/runner/specbranch"
)

// TestBranchRouterInterface verifies that Router implements BranchRouter interface.
// This test will fail compilation if BranchRouter doesn't exist or Router doesn't implement it.
func TestBranchRouterInterface(t *testing.T) {
	t.Parallel()
	var _ BranchRouter = (*specbranch.Router)(nil)
	t.Log("Router implements BranchRouter")
}

// TestGitCheckoutInterface verifies that GitOps implements GitCheckout interface.
// This test will fail compilation if GitCheckout doesn't exist or GitOps doesn't implement it.
func TestGitCheckoutInterface(t *testing.T) {
	t.Parallel()
	var _ GitCheckout = (*specbranch.GitOps)(nil)
	t.Log("GitOps implements GitCheckout")
}
