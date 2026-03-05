//go:build acceptance
// +build acceptance

package v2

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestSpecExecutionCreatesIsolatedWorktree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	git := &spyGitAdapter{}
	executor := NewSpecExecutor(git)

	_, err := executor.Execute(ctx, "spec-123", nil)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if git.calledCreateWorktree == 0 {
		t.Fatalf("expected GitAdapter to be asked for a worktree")
	}

	if got, want := git.lastSpecID, "spec-123"; got != want {
		t.Fatalf("last spec ID = %q, want %q", got, want)
	}
}

type spyGitAdapter struct {
	calledCreateWorktree int
	lastSpecID           string
}

func (s *spyGitAdapter) CreateIsolatedWorktree(ctx context.Context, specID string) (string, error) {
	s.calledCreateWorktree++
	s.lastSpecID = specID
	return "/tmp/spec-worktree", nil
}

func TestValidationUsesProjectConfiguration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := &config.Config{}
	cfg.Validation.Commands = []string{"inspect", "lint"}

	runner := &spyValidationRunner{}
	stage := NewValidationStage(runner, cfg)

	if err := stage.Run(ctx); err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	if got, want := runner.commands, cfg.Validation.Commands; len(got) != len(want) {
		t.Fatalf("commands recorded = %v, want %v", got, want)
	}
}

type spyValidationRunner struct {
	commands []string
}

func (s *spyValidationRunner) Run(ctx context.Context, command string) error {
	s.commands = append(s.commands, command)
	return nil
}

func TestPromptAssemblerCompilesAllLayers(t *testing.T) {
	t.Parallel()

	assembler := NewPromptAssembler("base", "project", "instance", "fragment")
	output := assembler.Assemble()

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
