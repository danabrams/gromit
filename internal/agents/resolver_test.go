package agents_test

import (
	"testing"

	"github.com/danabrams/gromit/internal/agents"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

func TestNewResolver_ReturnsAgentResolver(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	resolver := agents.NewResolver(cfg)

	if resolver == nil {
		t.Fatal("agents.NewResolver returned nil")
	}

	var _ pipeline.AgentResolver = resolver
}
