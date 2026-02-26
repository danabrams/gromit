package agents

import (
	"os"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

type resolver struct {
	cfg *config.Config
}

var _ pipeline.AgentResolver = (*resolver)(nil)

// Resolve delegates to agent.Resolve with stdin/stdout so CLI commands can prompt the user.
func (r *resolver) Resolve(phase string, flagOverride string, choosePicker bool) (pipeline.Agent, error) {
	return agent.Resolve(r.cfg, phase, flagOverride, choosePicker, os.Stdin, os.Stdout)
}

// NewResolver creates a pipeline.AgentResolver backed by agent.Resolve using the provided config.
func NewResolver(cfg *config.Config) pipeline.AgentResolver {
	return &resolver{cfg: cfg}
}
