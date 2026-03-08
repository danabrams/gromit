package llm

import (
	"strings"
	"testing"
	"time"
)

// Contract: codex adapter args must never contain Claude-specific flags.

func TestCodexAdapter_NeverIncludesDashP(t *testing.T) {
	a := &codexAdapter{
		binary:          "codex",
		flags:           nil,
		timeout:         5 * time.Second,
		tierToReasoning: map[string]string{},
	}
	args := a.buildExecCommandArgs("o3", "high")
	for _, arg := range args {
		if arg == "-p" {
			t.Fatalf("codex args must not contain -p (claude --print flag): %v", args)
		}
	}
}

func TestCodexAdapter_NeverIncludesOutputFormat(t *testing.T) {
	a := &codexAdapter{
		binary:          "codex",
		flags:           nil,
		timeout:         5 * time.Second,
		tierToReasoning: map[string]string{},
	}
	args := a.buildExecCommandArgs("o3", "")
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--output-format") {
		t.Fatalf("codex args must not contain --output-format (claude flag): %v", args)
	}
}

// Contract: codex adapter args must always start with "exec" subcommand.

func TestCodexAdapter_AlwaysStartsWithExec(t *testing.T) {
	a := &codexAdapter{
		binary:          "codex",
		flags:           []string{"--some-flag"},
		timeout:         5 * time.Second,
		tierToReasoning: map[string]string{},
	}
	args := a.buildExecCommandArgs("gpt-5.3-codex", "medium")
	if len(args) == 0 || args[0] != "exec" {
		t.Fatalf("codex args must start with 'exec', got: %v", args)
	}
}
