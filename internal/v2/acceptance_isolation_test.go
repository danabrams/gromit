//go:build acceptance
// +build acceptance

package v2

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/prompt"
	"github.com/danabrams/gromit/internal/v2/stage"
	stagevalidate "github.com/danabrams/gromit/internal/v2/stage/validate"
)

func TestValidationUsesProjectConfiguration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := &config.Config{}
	cfg.Validation.Commands = []string{"inspect", "lint"}

	runner := &acceptanceSpyValidationRunner{}
	vs, err := stagevalidate.New(cfg, runner)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	req := &stage.Request{Worktree: "/tmp/worktree"}
	result, err := vs.Run(ctx, req)
	if err != nil {
		t.Fatalf("validation failed: %v", err)
	}
	if result.Decision != stage.DecisionProceed {
		t.Fatalf("expected DecisionProceed, got %v", result.Decision)
	}

	if got, want := runner.commands, cfg.Validation.Commands; len(got) != len(want) {
		t.Fatalf("commands recorded = %v, want %v", got, want)
	}
}

type acceptanceSpyValidationRunner struct {
	commands []string
}

func (s *acceptanceSpyValidationRunner) Run(_ context.Context, command, worktree string) error {
	s.commands = append(s.commands, command)
	return nil
}

func TestPromptAssemblerCompilesAllLayers(t *testing.T) {
	t.Parallel()

	assembler := prompt.NewPromptAssembler("base", "project", "instance", "fragment")
	output := assembler.Assemble("", prompt.BeadInfo{})

	order := []string{"base", "project", "instance", "fragment"}
	for idx, fragment := range order {
		if !strings.Contains(output, fragment) {
			t.Fatalf("missing %q in prompt output", fragment)
		}
		if idx > 0 {
			prev := order[idx-1]
			if strings.Index(output, prev) > strings.Index(output, fragment) {
				t.Fatalf("layer %q should appear before %q", prev, fragment)
			}
		}
	}
}
