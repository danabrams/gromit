package main

import (
	"bytes"
	"testing"

	"github.com/danabrams/gromit/internal/agent"
	"github.com/spf13/cobra"
)

func TestShouldOverrideDebugModel(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			got := shouldOverrideDebugModel(tc.cmd, tc.selectedAgent)
			if got != tc.want {
				t.Fatalf("shouldOverrideDebugModel() = %v, want %v", got, tc.want)
			}
		})
	}
}

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

func TestMaybeWarnModelFlagOnNonClaudeAgent(t *testing.T) {
	t.Parallel()
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

	t.Run("warns when model flag changed for non-claude agent", func(t *testing.T) {
		t.Parallel()
		codexAgent := agent.New("codex", "codex", nil, agent.FileRef, "", nil)
		cmd := newCmd(true, true)

		stderr := &bytes.Buffer{}
		maybeWarnModelFlagOnNonClaudeAgent(cmd, codexAgent, stderr)

		warnOutput := stderr.String()
		if warnOutput == "" {
			t.Fatal("expected warning to be emitted to stderr")
		}
		if !bytes.Contains([]byte(warnOutput), []byte("codex")) {
			t.Errorf("expected warning to contain agent name 'codex', got: %s", warnOutput)
		}
		if !bytes.Contains([]byte(warnOutput), []byte("model")) {
			t.Errorf("expected warning to mention model flag, got: %s", warnOutput)
		}
	})

	t.Run("does not warn when model flag not changed", func(t *testing.T) {
		t.Parallel()
		codexAgent := agent.New("codex", "codex", nil, agent.FileRef, "", nil)
		cmd := newCmd(true, false) // model flag not changed

		stderr := &bytes.Buffer{}
		maybeWarnModelFlagOnNonClaudeAgent(cmd, codexAgent, stderr)

		if stderr.String() != "" {
			t.Errorf("expected no warning when model flag not changed, got: %s", stderr.String())
		}
	})

	t.Run("does not warn for Claude agent", func(t *testing.T) {
		t.Parallel()
		claudeAgent := agent.New("claude", "claude", nil, agent.FileRef, "", nil)
		cmd := newCmd(true, true) // model flag changed

		stderr := &bytes.Buffer{}
		maybeWarnModelFlagOnNonClaudeAgent(cmd, claudeAgent, stderr)

		if stderr.String() != "" {
			t.Errorf("expected no warning for Claude agent, got: %s", stderr.String())
		}
	})
}

func TestMaybeWarnModelFlagOnNonClaudeAgentTableDriven(t *testing.T) {
	t.Parallel()

	codexAgent := agent.New("codex", "codex", nil, agent.FileRef, "", nil)
	claudeAgent := agent.New("claude", "claude", nil, agent.FileRef, "", nil)

	cases := []struct {
		name           string
		agent          agent.Agent
		withModelFlag  bool
		setModel       bool
		expectWarning  bool
		expectedSubstr string
	}{
		{
			name:           "non-Claude warns when --model changed",
			agent:          codexAgent,
			withModelFlag:  true,
			setModel:       true,
			expectWarning:  true,
			expectedSubstr: "codex",
		},
		{
			name:          "non-Claude silent when --model not changed",
			agent:         codexAgent,
			withModelFlag: true,
			setModel:      false,
		},
		{
			name:          "non-Claude silent when --model flag missing",
			agent:         codexAgent,
			withModelFlag: false,
			setModel:      false,
		},
		{
			name:          "Claude silent when --model changed",
			agent:         claudeAgent,
			withModelFlag: true,
			setModel:      true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stderr := &bytes.Buffer{}
			cmd := prepareDebugModelCmd(t, tc.withModelFlag, tc.setModel)

			maybeWarnModelFlagOnNonClaudeAgent(cmd, tc.agent, stderr)
			output := stderr.String()

			if tc.expectWarning {
				if output == "" {
					t.Fatalf("expected warning for %s, got none", tc.name)
				}
				if tc.expectedSubstr != "" && !bytes.Contains(stderr.Bytes(), []byte(tc.expectedSubstr)) {
					t.Fatalf("expected warning to contain %q, got: %s", tc.expectedSubstr, output)
				}
				return
			}

			if output != "" {
				t.Fatalf("expected no warning for %s, got: %s", tc.name, output)
			}
		})
	}
}
