package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/runner/specbranch"
)

// TestOrchestratorConfigHasBranchRouter verifies that OrchestratorConfig has BranchRouter field.
// This test will fail compilation if the field doesn't exist.
func TestOrchestratorConfigHasBranchRouter(t *testing.T) {
	t.Parallel()
	cfg := OrchestratorConfig{}
	// This should compile if BranchRouter field exists
	var _ BranchRouter = cfg.BranchRouter
	if cfg.BranchRouter == nil {
		t.Log("OrchestratorConfig.BranchRouter is nil by default")
	}
}

// TestOrchestratorConfigHasGitCheckout verifies that OrchestratorConfig has GitCheckout field.
// This test will fail compilation if the field doesn't exist.
func TestOrchestratorConfigHasGitCheckout(t *testing.T) {
	t.Parallel()
	cfg := OrchestratorConfig{}
	// This should compile if GitCheckout field exists
	var _ GitCheckout = cfg.GitCheckout
	if cfg.GitCheckout == nil {
		t.Log("OrchestratorConfig.GitCheckout is nil by default")
	}
}

// TestOrchestratorConfigCanBeWiredWithDependencies verifies that dependencies can be set.
func TestOrchestratorConfigCanBeWiredWithDependencies(t *testing.T) {
	t.Parallel()
	router := specbranch.NewRouter("main")
	gitOps := specbranch.NewGitOps("/tmp", "main")

	cfg := OrchestratorConfig{
		BranchRouter: router,
		GitCheckout:  gitOps,
	}

	if cfg.BranchRouter != router {
		t.Error("BranchRouter not properly set")
	}
	if cfg.GitCheckout != gitOps {
		t.Error("GitCheckout not properly set")
	}
}
