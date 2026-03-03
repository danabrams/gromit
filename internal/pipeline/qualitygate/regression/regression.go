package regression

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// Stage runs regression tests via the configured command.
type Stage struct {
	cmdRunner runtypes.CmdRunnerFn
}

// New creates a regression gate stage that executes the configured command using
// the provided command runner. The stage uses the command defined in the
// regression gate config (defaults to "go test ./...").
func New(cmdRunner runtypes.CmdRunnerFn) pipeline.Stage {
	if cmdRunner == nil {
		cmdRunner = func(ctx context.Context, command, workDir string) (string, string, int, error) {
			return "", "", 0, fmt.Errorf("regression gate: command runner is not configured")
		}
	}
	return &Stage{cmdRunner: cmdRunner}
}

// Run executes the regression command when configured and enabled.
func (s *Stage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	if !isEnabled(in.Config) {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	command := strings.TrimSpace(in.Config.QualityGates.Regression.Command)
	if command == "" {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	stdout, stderr, exitCode, err := s.cmdRunner(ctx, command, ".")
	if err != nil {
		return pipeline.Output{}, fmt.Errorf("regression gate: %w", err)
	}

	if exitCode == 0 {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	failureOutput := summarizeFailure(command, stdout, stderr, exitCode)
	return pipeline.Output{
		Decision:           pipeline.Block,
		ValidationFailures: []string{failureOutput},
	}, nil
}

func summarizeFailure(command, stdout, stderr string, exitCode int) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("%s: exit code %d", command, exitCode))
	if trimmed := strings.TrimSpace(stdout); trimmed != "" {
		lines = append(lines, trimmed)
	}
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		lines = append(lines, trimmed)
	}
	return strings.Join(lines, "\n")
}

func isEnabled(cfg *config.Config) bool {
	if cfg == nil || cfg.QualityGates == nil || cfg.QualityGates.Regression == nil {
		return false
	}
	return cfg.QualityGates.Regression.Enabled
}
