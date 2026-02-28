package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/spf13/cobra"
)

func TestResolveInteractiveModel(t *testing.T) {
	t.Parallel()

	t.Run("returns model value when flag is set", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("model", "opus", "model override")
		if err := cmd.Flags().Set("model", "sonnet"); err != nil {
			t.Fatalf("setting model flag: %v", err)
		}

		got := resolveInteractiveModel(cmd, "model")
		if got != "sonnet" {
			t.Fatalf("resolveInteractiveModel() = %q, want %q", got, "sonnet")
		}
	})

	t.Run("returns default value when flag not set", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("model", "opus", "model override")

		got := resolveInteractiveModel(cmd, "model")
		if got != "opus" {
			t.Fatalf("resolveInteractiveModel() = %q, want %q", got, "opus")
		}
	})

	t.Run("returns empty string when flag does not exist", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{Use: "test"}

		got := resolveInteractiveModel(cmd, "model")
		if got != "" {
			t.Fatalf("resolveInteractiveModel() = %q, want empty string", got)
		}
	})

	t.Run("returns empty string when command is nil", func(t *testing.T) {
		t.Parallel()
		got := resolveInteractiveModel(nil, "model")
		if got != "" {
			t.Fatalf("resolveInteractiveModel() = %q, want empty string", got)
		}
	})
}

func TestTryOverrideModel(t *testing.T) {
	t.Parallel()

	t.Run("returns overridden agent when model flag set for Claude", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("model", "opus", "model override")
		if err := cmd.Flags().Set("model", "sonnet"); err != nil {
			t.Fatalf("setting model flag: %v", err)
		}

		claudeAgent := agent.New("claude", "claude", nil, agent.FileRef, "", nil)
		overridden := TryOverrideModel(cmd, claudeAgent, "sonnet", nil, "model")

		if overridden == claudeAgent {
			t.Fatal("expected overridden agent, got same agent")
		}
		if overridden == nil {
			t.Fatal("expected non-nil agent")
		}
	})

	t.Run("returns original agent when model flag not set", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("model", "opus", "model override")

		claudeAgent := agent.New("claude", "claude", nil, agent.FileRef, "", nil)
		overridden := TryOverrideModel(cmd, claudeAgent, "opus", nil, "model")

		if overridden != claudeAgent {
			t.Fatal("expected original agent when flag not changed")
		}
	})

	t.Run("returns original agent for non-Claude when flag changed", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("model", "opus", "model override")
		if err := cmd.Flags().Set("model", "sonnet"); err != nil {
			t.Fatalf("setting model flag: %v", err)
		}

		codexAgent := agent.New("codex", "codex", nil, agent.FileRef, "", nil)
		overridden := TryOverrideModel(cmd, codexAgent, "sonnet", nil, "model")

		if overridden != codexAgent {
			t.Fatal("expected original agent for non-Claude when flag changed")
		}
	})
}

type unsupportedAgent struct{}

func (unsupportedAgent) Name() string                      { return "custom" }
func (unsupportedAgent) Launch(string) error               { return nil }
func (unsupportedAgent) LaunchInDir(string, string) error  { return nil }
func (unsupportedAgent) Command(string) (*exec.Cmd, error) { return nil, nil }

func TestExploreInteractiveModelForwarderOverridesClaudeModel(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "explore"}
	cmd.Flags().String("model", "opus", "model override")
	if err := cmd.Flags().Set("model", "sonnet"); err != nil {
		t.Fatalf("setting model flag: %v", err)
	}

	forwarder := exploreInteractiveModelForwarder(cmd, nil, "model")
	claudeAgent := agent.New("claude", "claude", nil, agent.FileRef, "", nil)
	modified, warning := forwarder(claudeAgent, "sonnet")
	if warning != "" {
		t.Fatalf("expected no warning, got %q", warning)
	}
	if modified == claudeAgent {
		t.Fatal("expected modified agent, got original")
	}

	promptFile, err := os.CreateTemp(t.TempDir(), "prompt-*.md")
	if err != nil {
		t.Fatalf("creating temp prompt file: %v", err)
	}
	promptPath := promptFile.Name()
	promptFile.Close()
	defer os.Remove(promptPath)

	resolvableAgent, ok := modified.(agent.Agent)
	if !ok {
		t.Fatalf("modified agent is not agent.Agent, got %T", modified)
	}
	cmdStruct, err := resolvableAgent.Command(promptPath)
	if err != nil {
		t.Fatalf("getting command for modified agent: %v", err)
	}

	args := cmdStruct.Args
	modelFlagIndex := -1
	for i, arg := range args {
		if arg == "--model" {
			modelFlagIndex = i
			break
		}
	}
	if modelFlagIndex == -1 || modelFlagIndex+1 >= len(args) {
		t.Fatalf("expected --model flag in args, got %v", args)
	}
	if args[modelFlagIndex+1] != "sonnet" {
		t.Fatalf("expected model value sonnet, got %q", args[modelFlagIndex+1])
	}
}

func TestExploreInteractiveModelForwarderWarnsForUnsupportedAgent(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "explore"}
	cmd.Flags().String("model", "opus", "model override")
	if err := cmd.Flags().Set("model", "sonnet"); err != nil {
		t.Fatalf("setting model flag: %v", err)
	}

	forwarder := exploreInteractiveModelForwarder(cmd, nil, "model")
	custom := unsupportedAgent{}
	returned, warning := forwarder(custom, "sonnet")
	if returned != custom {
		t.Fatalf("expected unsupported agent to be returned unchanged")
	}
	expected := "--model flag ignored for non-Claude agent \"custom\""
	if warning != expected {
		t.Fatalf("warning = %q, want %q", warning, expected)
	}
}
