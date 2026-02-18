package main

import "testing"

func TestCliAgentResolver_UsesFlagOverrideParam(t *testing.T) {
	resolver := &cliAgentResolver{cfg: nil}

	agent, err := resolver.Resolve("review", "codex", false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if agent.Name() != "codex" {
		t.Fatalf("Resolve agent name = %q, want %q", agent.Name(), "codex")
	}
}
