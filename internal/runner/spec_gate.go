package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/danabrams/gromit/internal/frontmatter"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/scope"
	"github.com/danabrams/gromit/internal/specgate"
)

func (r *Runner) maybeRunSpecGate(ctx context.Context, st *runLoopState, specName string) error {
	if r == nil || st == nil || r.cfg == nil {
		return nil
	}
	if specName == "" {
		return nil
	}
	if !r.cfg.SpecGate.IsEnabled() || !r.cfg.SpecGate.IsAutoTrigger() {
		return nil
	}
	if r.specGate == nil || r.beads == nil {
		return nil
	}

	specsDir := r.cfg.Paths.Specs
	if err := scope.ValidateSpec(specsDir, specName); err != nil {
		return err
	}
	labels := scope.ResolveSpec(specName)
	if len(labels) == 0 {
		return fmt.Errorf("no label found for spec %q", specName)
	}

	beads, err := r.beads.ListWithLabel(labels[0])
	if err != nil {
		return err
	}
	for _, b := range beads {
		if b != nil && strings.EqualFold(b.Status, "open") {
			return nil
		}
	}

	if st.specGateCycles == nil {
		st.specGateCycles = make(map[string]int)
	}
	if r.cfg.SpecGate.MaxCycles > 0 && st.specGateCycles[specName] >= r.cfg.SpecGate.MaxCycles {
		return nil
	}

	criteria, _, _, err := loadSpecGateInputs(specsDir, specName)
	if err != nil {
		return err
	}

	verdict, err := r.specGate.Run(ctx, specName, criteria)
	if err != nil {
		return err
	}
	st.specGateCycles[specName]++

	if verdict != nil && !verdict.Passed {
		creator := &specGateBeadCreator{beads: r.beads}
		if _, err := specgate.SynthesizeFixBeads(ctx, specName, verdict.FailedCriteria(), "P1", creator); err != nil {
			return err
		}
	}
	return nil
}

const (
	specGateTestCommand = "go test -tags acceptance ./..."
	specGateDiffCommand = "git diff"
)

func (r *Runner) buildSpecGate() (*specgate.Gate, error) {
	if r == nil || r.cfg == nil {
		return nil, fmt.Errorf("runner config is nil")
	}
	if r.renderer == nil {
		return nil, fmt.Errorf("prompt renderer is nil")
	}

	specsDir := r.cfg.Paths.Specs

	gate := &specgate.Gate{
		Model:     r.cfg.SpecGate.Model,
		MaxCycles: r.cfg.SpecGate.MaxCycles,
		RunTests: func(ctx context.Context) (string, error) {
			return r.runSpecGateCommand(ctx, specGateTestCommand)
		},
		GetDiff: func(ctx context.Context) (string, error) {
			return r.runSpecGateCommand(ctx, specGateDiffCommand)
		},
		RenderPrompt: func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
			_, criteriaBlock, specBody, err := loadSpecGateInputs(specsDir, specName)
			if err != nil {
				return "", err
			}
			acceptance := criteriaBlock
			if strings.TrimSpace(acceptance) == "" {
				acceptance = formatAcceptanceCriteria(criteria)
			}
			promptCtx := &prompt.SpecGateContext{
				SpecCriteria:       specBody,
				TestOutput:         testOutput,
				CumulativeDiff:     diff,
				AcceptanceCriteria: acceptance,
			}
			return r.renderer.RenderSpecGate(promptCtx)
		},
		InvokeLLM: func(ctx context.Context, model, promptText string) ([]byte, error) {
			return r.invokeSpecGateLLM(ctx, model, promptText)
		},
	}

	return gate, nil
}

func (r *Runner) runSpecGateCommand(ctx context.Context, command string) (string, error) {
	stdout, stderr, exitCode, err := r.runCmd(ctx, command, "")
	if err != nil {
		return "", err
	}

	output := strings.TrimSpace(strings.Join([]string{stdout, stderr}, "\n"))
	if exitCode != 0 {
		if output == "" {
			output = fmt.Sprintf("%s (exit %d)", command, exitCode)
		} else {
			output = fmt.Sprintf("%s (exit %d)\n%s", command, exitCode, output)
		}
	}

	return strings.TrimSpace(output), nil
}

func (r *Runner) invokeSpecGateLLM(ctx context.Context, model, promptText string) ([]byte, error) {
	if r.router == nil {
		return nil, fmt.Errorf("router is nil")
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

type specGateBeadCreator struct {
	beads BeadClient
}

func (c *specGateBeadCreator) Create(ctx context.Context, title, description, priority string, labels []string) (string, error) {
	if c == nil || c.beads == nil {
		return "", fmt.Errorf("bead client is nil")
	}

	priorityInt, err := parseBeadPriority(priority)
	if err != nil {
		return "", err
	}

	expectedOutputs := []string{}
	if strings.TrimSpace(title) != "" {
		expectedOutputs = []string{strings.TrimSpace(title)}
	}
	b, err := c.beads.CreateWithParentAndDescription(title, priorityInt, labels, expectedOutputs, "", description)
	if err != nil {
		return "", err
	}
	if b == nil {
		return "", fmt.Errorf("bead creation returned nil")
	}
	return b.ID, nil
}

func parseBeadPriority(priority string) (int, error) {
	trimmed := strings.TrimSpace(priority)
	if trimmed == "" {
		return 0, fmt.Errorf("priority is empty")
	}
	if strings.HasPrefix(strings.ToUpper(trimmed), "P") {
		trimmed = strings.TrimSpace(trimmed[1:])
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid priority %q", priority)
	}
	return value, nil
}

var _ specgate.BeadCreator = (*specGateBeadCreator)(nil)
