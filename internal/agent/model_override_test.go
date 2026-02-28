package agent

import "testing"

func TestTryOverrideModelAddsClaudeModelFlag(t *testing.T) {
    t.Parallel()

    claudeAgent := New("claude", "claude", nil, FileRef, "", nil)
    result := TryOverrideModel(claudeAgent, "sonnet")

    if result.Warning != "" {
        t.Fatalf("unexpected warning: %q", result.Warning)
    }
    if result.Agent == nil {
        t.Fatal("expected overridden agent, got nil")
    }

    modified, ok := result.Agent.(*cliAgent)
    if !ok {
        t.Fatalf("expected *cliAgent, got %T", result.Agent)
    }

    if len(modified.flags) < 2 {
        t.Fatalf("expected model flag appended, got flags %v", modified.flags)
    }

    if modified.flags[len(modified.flags)-2] != "--model" || modified.flags[len(modified.flags)-1] != "sonnet" {
        t.Fatalf("expected --model sonnet at end of flags, got %v", modified.flags)
    }
}
