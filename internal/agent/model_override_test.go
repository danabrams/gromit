package agent

import (
	"os/exec"
	"testing"
)

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

func TestTryOverrideModelWarnsForUnsupportedAgent(t *testing.T) {
	t.Parallel()

	custom := unsupportedAgent{}
	got := TryOverrideModel(custom, "sonnet")
	if got.Agent != nil {
		t.Fatalf("expected nil agent for unsupported override, got %T", got.Agent)
	}
	if got.Warning != "model override not supported for agent \"custom\"" {
		t.Fatalf("unexpected warning: %q", got.Warning)
	}
}

type unsupportedAgent struct{}

func (unsupportedAgent) Name() string                      { return "custom" }
func (unsupportedAgent) Launch(string) error               { return nil }
func (unsupportedAgent) LaunchInDir(string, string) error  { return nil }
func (unsupportedAgent) Command(string) (*exec.Cmd, error) { return nil, nil }

func TestTryOverrideModelAddsModelFlagForCodex(t *testing.T) {
	t.Parallel()

	codexAgent := New("codex", "codex", nil, FileRef, "", nil)
	result := TryOverrideModel(codexAgent, "gpt-5.3-codex")

	if result.Warning != "" {
		t.Fatalf("unexpected warning for codex: %q", result.Warning)
	}
	if result.Agent == nil {
		t.Fatal("expected overridden agent for codex, got nil")
	}

	modified, ok := result.Agent.(*cliAgent)
	if !ok {
		t.Fatalf("expected *cliAgent, got %T", result.Agent)
	}

	if len(modified.extraArgs) < 2 {
		t.Fatalf("expected model flag in extra args, got %v", modified.extraArgs)
	}

	if modified.extraArgs[len(modified.extraArgs)-2] != "--model" || modified.extraArgs[len(modified.extraArgs)-1] != "gpt-5.3-codex" {
		t.Fatalf("expected --model gpt-5.3-codex at end of extra args, got %v", modified.extraArgs)
	}
}
