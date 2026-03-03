package wiring

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
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

	symbols := extractSymbolsFromDiff(diff)
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

type symbol struct {
	Name string
	File string
}

func extractSymbolsFromDiff(diff string) []symbol {
	scanner := bufio.NewScanner(strings.NewReader(diff))
	currentFile := ""
	seens := make(map[string]struct{})
	var symbols []symbol

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "+++ ") {
			raw := strings.TrimPrefix(line, "+++ ")
			if raw == "/dev/null" {
				currentFile = ""
				continue
			}
			if strings.HasPrefix(raw, "b/") {
				currentFile = filepath.ToSlash(strings.TrimPrefix(raw, "b/"))
			} else {
				currentFile = filepath.ToSlash(raw)
			}
			continue
		}
		if currentFile == "" {
			continue
		}
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		trimmed := strings.TrimSpace(line[1:])
		if trimmed == "" {
			continue
		}
		if name := parseSymbolName(trimmed); name != "" {
			key := currentFile + ":" + name
			if _, ok := seens[key]; ok {
				continue
			}
			seens[key] = struct{}{}
			symbols = append(symbols, symbol{
				Name: name,
				File: currentFile,
			})
		}
	}

	return symbols
}

var (
	funcDeclRE    = regexp.MustCompile(`^func\s+([A-Z]\w*)\s*\(`)
	methodDeclRE  = regexp.MustCompile(`^func\s+\([^)]*\)\s+([A-Z]\w*)\s*\(`)
	typeDeclRE    = regexp.MustCompile(`^type\s+([A-Z]\w*)\b`)
	constDeclRE   = regexp.MustCompile(`^const\s+([A-Z]\w*)\b`)
	varDeclRE     = regexp.MustCompile(`^var\s+([A-Z]\w*)\b`)
	symbolPatterns = []*regexp.Regexp{
		methodDeclRE,
		funcDeclRE,
		typeDeclRE,
		constDeclRE,
		varDeclRE,
	}
)

func parseSymbolName(line string) string {
	for _, pattern := range symbolPatterns {
		if matches := pattern.FindStringSubmatch(line); len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

func hasExternalReference(ctx context.Context, sym symbol) (bool, error) {
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
