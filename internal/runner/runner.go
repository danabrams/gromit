package runner

import (
	"io"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/experiment"
)

// Runner holds shared infrastructure used by pipeline stage adapters.
//
//nolint:govet // field alignment is intentionally grouped by responsibility.
type Runner struct {
	cfg             *config.Config
	tddOrchestrator *tddOrchestrator
	experimentMgr   *experiment.Manager
}

// NewRunner creates a new Orchestrator that sequences the 5-stage pipeline.
func NewRunner(cfg *config.Config, output io.Writer, labels ...string) (*Orchestrator, error) {
	return newRunnerImpl(cfg, output, labels)
}
