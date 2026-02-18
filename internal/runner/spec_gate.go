package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/danabrams/gromit/internal/frontmatter"
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

var acceptanceCriteriaNumberedRE = regexp.MustCompile(`^\\d+[.)]\\s+(.+)$`)

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

	b, err := c.beads.CreateWithParentAndDescription(title, priorityInt, labels, nil, "", description)
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
