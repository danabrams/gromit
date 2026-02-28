package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/spf13/cobra"
)

func prepareDebugModelCmd(t *testing.T, withModelFlag, setModel bool) *cobra.Command {
	t.Helper()

	cmd := &cobra.Command{Use: "debug"}
	if !withModelFlag {
		return cmd
	}

	cmd.Flags().String(debugModelFlag, "opus", "model override")
	if setModel {
		if err := cmd.Flags().Set(debugModelFlag, "sonnet"); err != nil {
			t.Fatalf("setting model flag: %v", err)
		}
	}

	return cmd
}

func TestApplyDebugModelOverrideAddsModelFlag(t *testing.T) {
	t.Parallel()

	cmd := prepareDebugModelCmd(t, true, true)
	selectedAgent := agent.New("claude", "claude", nil, agent.FileRef, "", nil)
	stderr := &bytes.Buffer{}
	overridden := applyDebugModelOverride(cmd, selectedAgent, nil, stderr)
	if stderr.Len() != 0 {
		t.Fatalf("unexpected warning to stderr: %s", stderr.String())
	}
	if overridden == selectedAgent {
		t.Fatal("expected agent override to occur")
	}

	agentWithModel, ok := overridden.(agent.Agent)
	if !ok {
		t.Fatalf("overridden agent is not agent.Agent, got %T", overridden)
	}

	promptFile, err := os.CreateTemp(t.TempDir(), "debug-prompt-*.md")
	if err != nil {
		t.Fatalf("creating temp prompt file: %v", err)
	}
	promptPath := promptFile.Name()
	promptFile.Close()
	defer os.Remove(promptPath)

	cmdStruct, err := agentWithModel.Command(promptPath)
	if err != nil {
		t.Fatalf("command build failed: %v", err)
	}
	modelFlagIndex := -1
	for i, arg := range cmdStruct.Args {
		if arg == "--model" {
			modelFlagIndex = i
			break
		}
	}
	if modelFlagIndex == -1 || modelFlagIndex+1 >= len(cmdStruct.Args) {
		t.Fatalf("expected --model flag, got args %v", cmdStruct.Args)
	}
	if cmdStruct.Args[modelFlagIndex+1] != "sonnet" {
		t.Fatalf("expected model value sonnet, got %q", cmdStruct.Args[modelFlagIndex+1])
	}
}

func TestApplyDebugModelOverrideWarnsForUnsupportedAgent(t *testing.T) {
	t.Parallel()

	cmd := prepareDebugModelCmd(t, true, true)
	stderr := &bytes.Buffer{}
	customAgent := unsupportedAgent{}
	overridden := applyDebugModelOverride(cmd, customAgent, nil, stderr)

	if overridden != customAgent {
		t.Fatalf("expected unsupported agent to be returned unchanged, got %T", overridden)
	}

	output := stderr.String()
	if !strings.Contains(output, "model override not supported for agent \"custom\"") {
		t.Fatalf("unexpected warning output: %s", output)
	}
}
