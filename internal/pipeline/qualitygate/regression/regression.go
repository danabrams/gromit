package regression

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner/validation"
)

// CommandRunner executes shell commands for regression testing.
type CommandRunner interface {
	Run(ctx context.Context, command, workDir string) (string, string, int, error)
}

// Stage enforces the regression gate stage by running configured commands.
type Stage struct {
	runner CommandRunner
}

var _ pipeline.Stage = (*Stage)(nil)

// New constructs a regression stage that uses the supplied command runner.
func New(runner CommandRunner) pipeline.Stage {
	return &Stage{runner: runner}
}

func (s *Stage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	if !isEnabled(in.Config) {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}
	if s.runner == nil {
		return pipeline.Output{}, fmt.Errorf("regression gate: command runner is not configured")
	}

	cfg := in.Config.QualityGates.Regression
	command := strings.TrimSpace(cfg.Command)
	if command == "" {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	modulePath, err := s.modulePath(ctx)
	if err != nil {
		return pipeline.Output{}, err
	}

	packages, err := s.listPackages(ctx)
	if err != nil {
		return pipeline.Output{}, err
	}

	targets := filterPackages(packages, modulePath, in.TouchedPackages)
	if len(targets) == 0 {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	testCommand := buildTestCommand(command, targets)
	stdout, stderr, exitCode, err := s.runner.Run(ctx, testCommand, ".")
	if err != nil {
		return pipeline.Output{}, fmt.Errorf("regression gate: %w", err)
	}
	if exitCode != 0 {
		failureOutput := formatCommandFailure(testCommand, exitCode, stdout, stderr)
		summary := validation.ExtractValidationSummary(failureOutput)
		if summary == "" {
			summary = failureOutput
		}
		return pipeline.Output{
			Decision:           pipeline.Block,
			ValidationFailures: []string{summary},
		}, nil
	}

	return pipeline.Output{Decision: pipeline.Proceed}, nil
}

func (s *Stage) modulePath(ctx context.Context) (string, error) {
	stdout, stderr, exitCode, err := s.runner.Run(ctx, "go list -m", ".")
	if err != nil {
		return "", fmt.Errorf("regression gate: module path: %w", err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("regression gate: module path discovery failed (%d): %s", exitCode, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

func (s *Stage) listPackages(ctx context.Context) ([]string, error) {
	stdout, stderr, exitCode, err := s.runner.Run(ctx, "go list ./...", ".")
	if err != nil {
		return nil, fmt.Errorf("regression gate: listing packages: %w", err)
	}
	if exitCode != 0 {
		return nil, fmt.Errorf("regression gate: listing packages failed (%d): %s", exitCode, strings.TrimSpace(stderr))
	}
	return parsePackages(stdout), nil
}

func parsePackages(output string) []string {
	lines := strings.Split(output, "\n")
	packages := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		packages = append(packages, trimmed)
	}
	return packages
}

func filterPackages(all []string, modulePath string, touched []string) []string {
	touchedSet := make(map[string]struct{})
	for _, pkg := range normalizeTouchedPackages(touched) {
		touchedSet[pkg] = struct{}{}
	}

	remaining := make([]string, 0, len(all))
	for _, pkg := range all {
		rel := relativePackage(modulePath, pkg)
		if rel == "" {
			continue
		}
		if _, ok := touchedSet[rel]; ok {
			continue
		}
		remaining = append(remaining, rel)
	}
	return remaining
}

func normalizeTouchedPackages(packages []string) []string {
	seen := make(map[string]struct{}, len(packages))
	normalized := make([]string, 0, len(packages))
	for _, raw := range packages {
		trimmed := strings.TrimSpace(raw)
		trimmed = strings.TrimSuffix(trimmed, "/")
		trimmed = strings.TrimPrefix(trimmed, "./")
		trimmed = strings.TrimPrefix(trimmed, "/")
		if trimmed == "" {
			trimmed = "."
		}
		if trimmed == "." {
			trimmed = "."
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func relativePackage(modulePath, pkg string) string {
	if modulePath != "" {
		if pkg == modulePath {
			return "."
		}
		prefix := modulePath + "/"
		if strings.HasPrefix(pkg, prefix) {
			return strings.TrimPrefix(pkg, prefix)
		}
	}
	return pkg
}

func buildTestCommand(base string, targets []string) string {
	trimmed := strings.TrimSpace(base)
	formatted := formatTargets(targets)
	if len(formatted) == 0 {
		return trimmed
	}
	args := strings.Join(formatted, " ")
	if trimmed == "" {
		return args
	}
	if strings.Contains(trimmed, "./...") {
		return strings.ReplaceAll(trimmed, "./...", args)
	}
	return trimmed + " " + args
}

func formatTargets(targets []string) []string {
	formatted := make([]string, 0, len(targets))
	for _, pkg := range targets {
		arg := formatPackageArg(pkg)
		if arg == "" {
			continue
		}
		formatted = append(formatted, arg)
	}
	return formatted
}

func formatPackageArg(pkg string) string {
	trimmed := strings.TrimSpace(pkg)
	trimmed = strings.TrimSuffix(trimmed, "/")
	trimmed = strings.TrimPrefix(trimmed, "./")
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" || trimmed == "." {
		return "."
	}
	return "./" + trimmed
}

func formatCommandFailure(command string, exitCode int, stdout, stderr string) string {
	parts := []string{fmt.Sprintf("%s: exit code %d", command, exitCode)}
	if trimmed := strings.TrimSpace(stdout); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		parts = append(parts, trimmed)
	}
	return strings.Join(parts, "\n")
}

func isEnabled(cfg *config.Config) bool {
	if cfg == nil || cfg.QualityGates == nil || cfg.QualityGates.Regression == nil {
		return false
	}
	return cfg.QualityGates.Regression.Enabled
}
