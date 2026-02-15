package main

import "testing"

func TestDebugCommandHasAgentFlag(t *testing.T) {
	flag := debugCmd.Flags().Lookup("agent")
	if flag == nil {
		t.Fatal("debug command missing --agent flag")
	}
	if flag.Value.Type() != "string" {
		t.Fatalf("--agent flag type = %q, want %q", flag.Value.Type(), "string")
	}
}
