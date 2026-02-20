package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/frontmatter"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/scope"
	"github.com/danabrams/gromit/internal/specgate"
)

const (
	specGateInvocationFailureName = "spec gate analysis"
)

// SpecGateValidationRunner executes validation commands for spec gate checks.
type SpecGateValidationRunner interface {
	RunDirect(ctx context.Context, commands []string, workDir string) (*claude.Result, error)
}

// GateFailure is a structured spec gate failure.
type GateFailure struct {
	TestName     string `json:"test_name"`
	Message      string `json:"message"`
	SuggestedFix string `json:"suggested_fix"`
}

// GateResult is the spec gate verification result.
type GateResult struct {
	Passed   bool          `json:"passed"`
	Failures []GateFailure `json:"failures"`
}

// normalizeNilFields ensures nil slices are normalized to empty slices.
func (r *GateResult) normalizeNilFields() {
	if r == nil {
		return
	}
	if r.Failures == nil {
		r.Failures = []GateFailure{}
	}
}

// SpecGate verifies a spec by running validation then analyzing failures via provider.
type SpecGate struct {
	validationRunner SpecGateValidationRunner
	renderer         PromptRenderer
	router           *provider.Router
	cfg              *config.Config
	workDir          string
}

// Verify runs validation and returns a structured gate result.
func (g *SpecGate) Verify(ctx context.Context, specName, specContent string) (*GateResult, error) {
	if err := g.validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(specName) == "" {
		return nil, fmt.Errorf("spec name is empty")
	}

	validationResult, err := g.validationRunner.RunDirect(ctx, g.cfg.Validation.FullCommandsOrDefault(), g.workDir)
	if err != nil {
		return nil, fmt.Errorf("running spec gate validation: %w", err)
	}
	if validationResult == nil {
		return nil, fmt.Errorf("spec gate validation returned nil result")
	}

	if validationResult.Success {
		return &GateResult{Passed: true, Failures: []GateFailure{}}, nil
	}

	failureOutput := strings.TrimSpace(validationResult.Output)
	if failureOutput == "" {
		failureOutput = "validation failed"
	}

	analysis, err := g.analyzeValidationFailure(ctx, specContent, failureOutput)
	if err != nil {
		return g.providerInvocationFailure(err), nil
	}

	analysis.Passed = false
	analysis.normalizeNilFields()
	if len(analysis.Failures) == 0 {
		analysis.Failures = []GateFailure{{
			TestName: specGateInvocationFailureName,
			Message:  failureOutput,
		}}
	}

	return analysis, nil
}

func (g *SpecGate) validate() error {
	if g == nil {
		return fmt.Errorf("spec gate is nil")
	}
	if g.cfg == nil {
		return fmt.Errorf("spec gate config is nil")
	}
	if g.validationRunner == nil {
		return fmt.Errorf("spec gate validation runner is nil")
	}
	if g.renderer == nil {
		return fmt.Errorf("spec gate renderer is nil")
	}
	return nil
}

func (g *SpecGate) analyzeValidationFailure(ctx context.Context, specContent, failureOutput string) (*GateResult, error) {
	criteria, criteriaBlock := extractAcceptanceCriteria(specContent)
	acceptance := criteriaBlock
	if strings.TrimSpace(acceptance) == "" {
		acceptance = formatAcceptanceCriteria(criteria)
	}

	promptText, err := g.renderer.RenderSpecGate(&prompt.SpecGateContext{
		SpecCriteria:       specContent,
		FailureOutput:      failureOutput,
		AcceptanceCriteria: acceptance,
	})
	if err != nil {
		return nil, fmt.Errorf("rendering spec gate prompt: %w", err)
	}

	rawResult, err := g.invokeProvider(ctx, promptText)
	if err != nil {
		return nil, err
	}

	parsed, err := parseGateResult(rawResult)
	if err != nil {
		return nil, fmt.Errorf("parsing spec gate result: %w", err)
	}
	return parsed, nil
}

func (g *SpecGate) invokeProvider(ctx context.Context, promptText string) ([]byte, error) {
	if g.router == nil {
		return nil, fmt.Errorf("spec gate router is nil")
	}

	tier := provider.TierFromLegacyModel(g.cfg.SpecGate.Model)
	p, _ := g.router.Select("build", tier)
	if p == nil {
		return nil, fmt.Errorf("no providers available for tier %q", tier)
	}

	result, err := p.Run(ctx, promptText, tier)
	if err != nil && p.IsUsageLimitError(result, err) {
		g.router.MarkUnavailable(p.Name())
		p, _ = g.router.Select("build", tier)
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

func (g *SpecGate) providerInvocationFailure(err error) *GateResult {
	result := &GateResult{
		Passed: false,
		Failures: []GateFailure{{
			TestName: specGateInvocationFailureName,
			Message:  fmt.Sprintf("provider invocation failed: %v", err),
		}},
	}
	result.normalizeNilFields()
	return result
}

func parseGateResult(data []byte) (*GateResult, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}

	var result GateResult
	if err := json.Unmarshal(data, &result); err == nil {
		if _, hasFailures := fields["failures"]; hasFailures {
			result.normalizeNilFields()
			return &result, nil
		}
	}

	legacy, err := specgate.ParseVerdict(data)
	if err != nil {
		return nil, err
	}

	converted := &GateResult{Passed: legacy.Passed, Failures: []GateFailure{}}
	for _, criterion := range legacy.FailedCriteria() {
		converted.Failures = append(converted.Failures, GateFailure{
			TestName: criterion.Criterion,
			Message:  criterion.Evidence,
		})
	}
	converted.normalizeNilFields()
	return converted, nil
}

func (r *Runner) maybeRunSpecGate(ctx context.Context, specName string) error {
	if r == nil || r.cfg == nil {
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
	specLabels := scope.ResolveSpec(specName)
	if len(specLabels) == 0 {
		return fmt.Errorf("no label found for spec %q", specName)
	}
	specLabel := specLabels[0]
	if !isScopedRunLabel(r.labelFilters, specLabel) {
		return nil
	}

	beads, err := r.beads.ListWithLabel(specLabel)
	if err != nil {
		return err
	}
	if hasOpenBeads(beads) {
		return nil
	}

	if r.specGateCycles == nil {
		r.specGateCycles = make(map[string]int)
	}
	currentCycles := r.specGateCycles[specName]
	if r.cfg.SpecGate.MaxCycles > 0 && currentCycles >= r.cfg.SpecGate.MaxCycles {
		return nil
	}

	_, _, specBody, err := loadSpecGateInputs(specsDir, specName)
	if err != nil {
		return err
	}

	result, err := r.specGate.Verify(ctx, specName, specBody)
	if err != nil {
		return err
	}
	r.specGateCycles[specName] = currentCycles + 1

	if result != nil && !result.Passed {
		if _, err := SynthesizeFixBeads(ctx, specName, result.Failures, r.beads); err != nil {
			return err
		}
	}
	return nil
}

func hasOpenBeads(beads []*bead.Bead) bool {
	for _, b := range beads {
		if b != nil && strings.EqualFold(b.Status, "open") {
			return true
		}
	}
	return false
}

func isScopedRunLabel(filters []string, label string) bool {
	if len(filters) == 0 {
		return false
	}
	for _, filter := range filters {
		if strings.EqualFold(strings.TrimSpace(filter), label) {
			return true
		}
	}
	return false
}

func (r *Runner) buildSpecGate() (*SpecGate, error) {
	if r == nil || r.cfg == nil {
		return nil, fmt.Errorf("runner config is nil")
	}
	if r.renderer == nil {
		return nil, fmt.Errorf("prompt renderer is nil")
	}
	if r.validationRunner == nil {
		return nil, fmt.Errorf("validation runner is nil")
	}

	return &SpecGate{
		cfg:              r.cfg,
		validationRunner: r.validationRunner,
		renderer:         r.renderer,
		router:           r.router,
	}, nil
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
