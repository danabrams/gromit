package main

import (
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
