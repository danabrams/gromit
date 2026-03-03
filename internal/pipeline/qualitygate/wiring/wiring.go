package wiring

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// GitDiffFn returns the current git diff for wiring analysis.
type GitDiffFn func(ctx context.Context) (string, error)

// Gate enforces wiring checks for extracted symbols.
type Gate struct {
	gitDiff GitDiffFn
}

// Ensure Gate implements pipeline.Stage.
var _ pipeline.Stage = (*Gate)(nil)

// New creates a wiring gate stage. gitDiff should be provided when the gate is enabled.
func New(gitDiff GitDiffFn) *Gate {
	if gitDiff == nil {
		gitDiff = func(context.Context) (string, error) { return "", nil }
	}
	return &Gate{gitDiff: gitDiff}
}

// Run executes the wiring gate. Symbols are extracted from the git diff and each
// must have at least one reference outside its defining file.
func (g *Gate) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	if !isEnabled(in.Config) {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}
	diff, err := g.gitDiff(ctx)
	if err != nil {
		return pipeline.Output{}, fmt.Errorf("wiring: git diff: %w", err)
	}

	symbols := ExtractSymbolsFromDiff(diff)
	if len(symbols) == 0 {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	var failures []string
	for _, sym := range symbols {
		wired, err := hasExternalReference(ctx, sym)
		if err != nil {
			return pipeline.Output{}, err
		}
		if !wired {
			failures = append(failures, fmt.Sprintf("%s exported but not referenced", sym.Name))
		}
	}

	if len(failures) > 0 {
		return pipeline.Output{
			Decision:       pipeline.Block,
			WiringFailures: failures,
		}, nil
	}
	return pipeline.Output{Decision: pipeline.Proceed}, nil
}

func isEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return cfg.WiringGate.Enabled
}

func hasExternalReference(ctx context.Context, sym Symbol) (bool, error) {
	cmd := exec.CommandContext(
		ctx,
		"grep",
		"-R",
		"-l",
		"-F",
		"-w",
		"--exclude-dir=.git",
		"--include=*.go",
		sym.Name,
		".",
	)

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 1 {
				output = exitErr.Stderr
			} else {
				return false, fmt.Errorf("wiring: grep error: %w", err)
			}
		} else {
			return false, fmt.Errorf("wiring: grep error: %w", err)
		}
	}

	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		match := filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(line), "./"))
		if match == "" {
			continue
		}
		if match != sym.File {
			return true, nil
		}
	}

	return false, nil
}
