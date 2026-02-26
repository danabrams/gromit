package runner

import (
	"fmt"
	"io"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/experiment"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// Runner holds shared infrastructure used by pipeline stage adapters.
//
//nolint:govet // field alignment is intentionally grouped by responsibility.
type Runner struct {
	cfg             *config.Config
	tddOrchestrator *tddOrchestrator
	experimentMgr   *experiment.Manager
	beads           coverageCommentClient
	output          io.Writer
}

type coverageCommentClient interface {
	AddComment(id, comment string) error
}

func (r *Runner) reportCoverage(bc *runtypes.BeadContext, tracker *coverage.CoverageTracker) {
	if bc == nil || bc.Result == nil {
		return
	}
	populateCoverageResult(bc, tracker)
	addCoverageCommentWithClient(bc, tracker, r.beads)
	logCoverageSummary(r.output, tracker)
}

// Deps holds dependencies for constructing a Runner with Router-only support.
type Deps struct {
	Router *provider.Router
}

// NewRunner creates a new Orchestrator that sequences the 5-stage pipeline.
func NewRunner(cfg *config.Config, output io.Writer, labels ...string) (*Orchestrator, error) {
	return newRunnerImpl(cfg, output, labels)
}

// NewRunnerWithDeps creates an Orchestrator using only Router dependencies,
// without Claude fallback behavior.
func NewRunnerWithDeps(deps *Deps) (*Orchestrator, error) {
	if deps == nil {
		return nil, fmt.Errorf("deps cannot be nil")
	}
	if deps.Router == nil {
		return nil, fmt.Errorf("Router dependency is required")
	}

	// Return a minimal Orchestrator with Router-only dependencies
	return NewOrchestrator(OrchestratorConfig{}), nil
}
