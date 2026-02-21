package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/frontmatter"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/validation"
	"github.com/danabrams/gromit/internal/specgate"
)

// SpecGateValidationRunner abstracts direct validation execution used by spec gate.
type SpecGateValidationRunner interface {
	RunDirect(ctx context.Context, commands []string, workDir string) (*claude.Result, error)
}

var _ SpecGateValidationRunner = (*validation.Runner)(nil)

func (r *Runner) buildSpecGate() (*specgate.Gate, error) {
	if r == nil || r.cfg == nil {
		return nil, fmt.Errorf("runner config is nil")
	}
	if r.renderer == nil {
		return nil, fmt.Errorf("prompt renderer is nil")
	}
	if r.validationRunner == nil {
		return nil, fmt.Errorf("validation runner is nil")
	}

	gate := &specgate.Gate{
		Model:     r.cfg.SpecGate.Model,
		MaxCycles: r.cfg.SpecGate.MaxCycles,
		RunTests: func(ctx context.Context) (string, error) {
			validationResult, err := r.validationRunner.RunDirect(ctx, r.cfg.Validation.FullCommandsOrDefault(), r.gromitDir)
			if err != nil {
				return "", fmt.Errorf("running spec gate validation: %w", err)
			}
			if validationResult == nil {
				return "", fmt.Errorf("spec gate validation returned nil result")
			}
			return validationResult.Output, nil
		},
		GetDiff: func(ctx context.Context) (string, error) {
			head, err := r.getHead()
			if err != nil {
				return "", fmt.Errorf("resolve HEAD for spec gate diff: %w", err)
			}
			diff, err := r.getDiff(head)
			if err != nil {
				return "", fmt.Errorf("collect spec gate diff: %w", err)
			}
			return diff, nil
		},
		RenderPrompt: func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
			return r.renderer.RenderSpecGate(&prompt.SpecGateContext{
				SpecCriteria:       "spec:" + strings.TrimSpace(specName),
				TestOutput:         testOutput,
				CumulativeDiff:     diff,
				AcceptanceCriteria: formatAcceptanceCriteria(criteria),
			})
		},
		InvokeLLM: func(ctx context.Context, model, promptText string) ([]byte, error) {
			if r.router == nil {
				return nil, fmt.Errorf("spec gate router is nil")
			}

			tier := provider.TierFromLegacyModel(model)
			p, _ := r.router.Select("build", tier)
			if p == nil {
				return nil, fmt.Errorf("no providers available for tier %q", tier)
			}

			result, err := p.Run(ctx, promptText, tier)
			if err != nil && p.IsUsageLimitError(result, err) {
				r.router.MarkUnavailable(p.Name())
				p, _ = r.router.Select("build", tier)
				if p != nil {
					result, err = p.Run(ctx, promptText, tier)
				}
			}
			if err != nil {
				return nil, err
			}
			if result == nil {
				return nil, fmt.Errorf("provider returned nil result")
			}
			return []byte(result.Output), nil
		},
	}

	return gate, nil
}

var acceptanceCriteriaNumberedRE = regexp.MustCompile(`^\d+[.)]\s+(.+)$`)

func extractAcceptanceCriteria(body string) ([]string, string) {
	lines := strings.Split(body, "\n")
	inSection := false
	var blockLines []string
	criteria := make([]string, 0)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			if inSection {
				break
			}
			if strings.EqualFold(trimmed, "## Acceptance Criteria") {
				inSection = true
			}
			continue
		}

		if !inSection {
			continue
		}

		blockLines = append(blockLines, line)

		switch {
		case strings.HasPrefix(trimmed, "- "):
			criteria = append(criteria, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		case strings.HasPrefix(trimmed, "* "):
			criteria = append(criteria, strings.TrimSpace(strings.TrimPrefix(trimmed, "* ")))
		default:
			if matches := acceptanceCriteriaNumberedRE.FindStringSubmatch(trimmed); len(matches) == 2 {
				criteria = append(criteria, strings.TrimSpace(matches[1]))
			}
		}
	}

	block := strings.TrimSpace(strings.Join(blockLines, "\n"))

	return criteria, block
}

func loadSpecGateInputs(specsDir, specName string) ([]string, string, string, error) {
	specPath := filepath.Join(specsDir, specName+".md")
	_, body, err := frontmatter.ReadFile(specPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("reading spec: %w", err)
	}

	criteria, block := extractAcceptanceCriteria(body)
	if len(criteria) == 0 {
		return nil, block, body, fmt.Errorf("spec %q has no acceptance criteria", specName)
	}

	return criteria, block, body, nil
}

func formatAcceptanceCriteria(criteria []string) string {
	if len(criteria) == 0 {
		return ""
	}

	lines := make([]string, 0, len(criteria))
	for _, item := range criteria {
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n")
}
