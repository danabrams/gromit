package agent_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

func TestNewResolver_ReturnsAgentResolver(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	resolver := agent.NewResolver(cfg)

	if resolver == nil {
		t.Fatal("agent.NewResolver returned nil")
	}

	var _ pipeline.AgentResolver = resolver
}
