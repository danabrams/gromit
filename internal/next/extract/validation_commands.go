package extract

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danabrams/gromit/internal/next/fact"
)

// ValidationCommandsExtractor discovers validation commands from Makefiles
// and CI workflow files.
type ValidationCommandsExtractor struct{}

// NewValidationCommandsExtractor returns a ready-to-use ValidationCommandsExtractor.
func NewValidationCommandsExtractor() *ValidationCommandsExtractor {
	return &ValidationCommandsExtractor{}
}

// Name returns the extractor identifier.
func (e *ValidationCommandsExtractor) Name() string { return "validation-commands" }

// targetPattern matches Makefile target lines like "test:" or "build:".
var targetPattern = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_-]*)\s*:`)

// ciRunPattern matches CI workflow run steps like "- run: go test ./...".
var ciRunPattern = regexp.MustCompile(`^\s*-\s*run:\s*(.+)$`)

// Extract inspects a repository for validation commands in Makefiles and
// CI workflow files, returning one Observed fact per command found.
func (e *ValidationCommandsExtractor) Extract(repoPath string) ([]fact.Fact, error) {
	var facts []fact.Fact

	mkFacts, err := e.extractMakefile(repoPath)
	if err != nil {
		return nil, err
	}
	facts = append(facts, mkFacts...)

	ciFacts, err := e.extractCIWorkflows(repoPath)
	if err != nil {
		return nil, err
	}
	facts = append(facts, ciFacts...)

	return facts, nil
}

// extractMakefile parses a Makefile for target/command pairs.
func (e *ValidationCommandsExtractor) extractMakefile(repoPath string) ([]fact.Fact, error) {
	path := filepath.Join(repoPath, "Makefile")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var facts []fact.Fact
	scanner := bufio.NewScanner(f)

	var currentTarget string
	cmdIndex := 0
	for scanner.Scan() {
		line := scanner.Text()

		// Check for a target line.
		if m := targetPattern.FindStringSubmatch(line); m != nil {
			currentTarget = m[1]
			cmdIndex = 0
			continue
		}

		// Check for a command line (tab-indented) under the current target.
		if currentTarget != "" && strings.HasPrefix(line, "\t") {
			cmd := strings.TrimSpace(line)
			if cmd != "" {
				id := fmt.Sprintf("valcmd-makefile-%s-%d", currentTarget, cmdIndex)
				content := fmt.Sprintf("Makefile target '%s': %s", currentTarget, cmd)
				facts = append(facts, fact.New(id, fact.Observed, content, "validation-commands"))
				cmdIndex++
			}
			continue
		}

		// A non-indented, non-target line resets the current target.
		if !strings.HasPrefix(line, "\t") {
			currentTarget = ""
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return facts, nil
}

// extractCIWorkflows scans .github/workflows/*.yml for run steps.
func (e *ValidationCommandsExtractor) extractCIWorkflows(repoPath string) ([]fact.Fact, error) {
	workflowDir := filepath.Join(repoPath, ".github", "workflows")
	entries, err := os.ReadDir(workflowDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var facts []fact.Fact
	idx := 0

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}

		fileFacts, err := e.scanWorkflowFile(filepath.Join(workflowDir, name), name, idx)
		if err != nil {
			return nil, err
		}
		facts = append(facts, fileFacts...)
		idx += len(fileFacts)
	}

	return facts, nil
}

// scanWorkflowFile scans a single CI workflow file for run steps, returning
// facts with IDs starting at startIdx. Using a separate function ensures the
// file handle is closed via defer even if a panic occurs.
func (e *ValidationCommandsExtractor) scanWorkflowFile(path, name string, startIdx int) ([]fact.Fact, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var facts []fact.Fact
	idx := startIdx
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if m := ciRunPattern.FindStringSubmatch(line); m != nil {
			cmd := strings.TrimSpace(m[1])
			id := fmt.Sprintf("valcmd-ci-%d", idx)
			content := fmt.Sprintf("CI workflow '%s' run step: %s", name, cmd)
			facts = append(facts, fact.New(id, fact.Observed, content, "validation-commands"))
			idx++
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return facts, nil
}
