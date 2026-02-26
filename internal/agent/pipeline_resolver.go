package agent

import (
	"os"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

type pipelineResolver struct {
	cfg *config.Config
}

var _ pipeline.AgentResolver = (*pipelineResolver)(nil)

// Resolve delegates to agent.Resolve with stdin/stdout so CLI commands can prompt the user.
func (r *pipelineResolver) Resolve(phase string, flagOverride string, choosePicker bool) (pipeline.Agent, error) {
	return Resolve(r.cfg, phase, flagOverride, choosePicker, os.Stdin, os.Stdout)
}

// NewResolver creates a pipeline.AgentResolver backed by agent.Resolve using the provided config.
func NewResolver(cfg *config.Config) pipeline.AgentResolver {
	return &pipelineResolver{cfg: cfg}
}
