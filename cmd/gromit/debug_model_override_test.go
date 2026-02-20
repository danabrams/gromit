package main

import (
	"testing"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/spf13/cobra"
)

func TestShouldOverrideDebugModel(t *testing.T) {
	newCmd := func(withModelFlag bool, setModel bool) *cobra.Command {
		cmd := &cobra.Command{Use: "debug"}
		if withModelFlag {
			cmd.Flags().String("model", "opus", "model override")
			if setModel {
				if err := cmd.Flags().Set("model", "sonnet"); err != nil {
					t.Fatalf("setting model flag: %v", err)
				}
			}
		}
		return cmd
	}

	claudeAgent := agent.New("claude", "claude", nil, agent.FileRef, "", nil)
	codexAgent := agent.New("codex", "codex", nil, agent.FileRef, "", nil)
	var nilAgent agent.Agent

	tests := []struct {
		name          string
		cmd           *cobra.Command
		selectedAgent agent.Agent
		want          bool
	}{
		{
			name:          "returns false when model flag not changed",
			cmd:           newCmd(true, false),
			selectedAgent: claudeAgent,
			want:          false,
		},
		{
			name:          "returns true when model flag changed for claude",
			cmd:           newCmd(true, true),
			selectedAgent: claudeAgent,
			want:          true,
		},
		{
			name:          "returns false when model flag changed for non-claude agent",
			cmd:           newCmd(true, true),
			selectedAgent: codexAgent,
			want:          false,
		},
		{
			name:          "returns false when model flag is missing",
			cmd:           newCmd(false, false),
			selectedAgent: claudeAgent,
			want:          false,
		},
		{
			name:          "returns false when command is nil",
			cmd:           nil,
			selectedAgent: claudeAgent,
			want:          false,
		},
		{
			name:          "returns false when selected agent is nil",
			cmd:           newCmd(true, true),
			selectedAgent: nilAgent,
			want:          false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldOverrideDebugModel(tc.cmd, tc.selectedAgent)
			if got != tc.want {
				t.Fatalf("shouldOverrideDebugModel() = %v, want %v", got, tc.want)
			}
		})
	}
}
