package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/agents"
)

func TestCliAgentResolver_UsesFlagOverrideParam(t *testing.T) {
	resolver := agents.NewResolver(nil)

	agent, err := resolver.Resolve("review", "codex", false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if agent.Name() != "codex" {
		t.Fatalf("Resolve agent name = %q, want %q", agent.Name(), "codex")
	}
}

func TestCmdAgentResolver_PropagatesResolveError(t *testing.T) {
	resolver := agents.NewResolver(nil)

	_, err := resolver.Resolve("review", "does-not-exist", false)
	if err == nil {
		t.Fatal("Resolve error = nil, want unknown agent error")
	}
}
