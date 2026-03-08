package triage

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/prompt"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

// Category describes the triage classification.
type Category string

const (
	CategoryDecompose   Category = "decompose"    // task too complex, break it down
	CategoryRetry       Category = "retry"        // transient error, retry same model
	CategoryUnclearSpec Category = "unclear_spec" // spec is ambiguous, surface to human
	CategoryUnsafe      Category = "unsafe"       // dangerous operation, hard stop
)

// TriageArtifacts captures the triage decision.
type TriageArtifacts struct {
	Category  Category
	Reasoning string
}

// Stage executes the triage stage of the v2 run loop.
type Stage struct {
	name     string
	cfg      *config.Config
	llm      llmtypes.LLMProvider
	base     string
	project  string
	fragment string
}

var _ stagepkg.Stage = (*Stage)(nil)

// New constructs a triage stage with the provided dependencies.
// base, project, and fragment are prompt layers passed to the PromptAssembler.
// If fragment is empty, defaultTriageFragment is used as a fallback.
func New(cfg *config.Config, provider llmtypes.LLMProvider, base, project, fragment string) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if provider == nil {
		return nil, fmt.Errorf("llm provider required")
	}
	if strings.TrimSpace(fragment) == "" {
		fragment = defaultTriageFragment
	}
	return &Stage{
		name:     stagedesc.Describe("triage", cfg),
		cfg:      cfg,
		llm:      provider,
		base:     base,
		project:  project,
		fragment: fragment,
	}, nil
}

// Name returns the canonical stage name.
func (s *Stage) Name() string {
	return s.name
}

const defaultTriageFragment = `# Build Failure Triage Instructions

You are a build failure triage system. Categorize the build failure described in the instance context above.

## Categories

Choose exactly one category:

- decompose: the task is too large or complex for a single implementation pass. Signs: partial implementation, too many files to change, multiple unrelated concerns.
- retry: transient error like network failure, rate limit, timeout, or environment issue. The same attempt would likely succeed.
- unclear_spec: the specification or requirements are ambiguous, contradictory, or missing information needed to implement.
- unsafe: the operation would be destructive, irreversible, or violate safety constraints.

## Decision Guidelines

- Prefer decompose when the failure output shows partial work or mentions multiple concerns
- Prefer retry when the failure is clearly environmental (network, timeout, process killed)
- Prefer unclear_spec when the LLM output shows confusion about requirements
- Use unsafe sparingly — only for genuinely dangerous operations

## Output Format

Output ONLY a JSON object with "category" and "reasoning" fields. Example:
{"category": "retry", "reasoning": "The build failed due to a network timeout downloading dependencies."}
`

// triageResponse is the internal struct for parsing the LLM JSON output.
type triageResponse struct {
	Category  string `json:"category"`
	Reasoning string `json:"reasoning"`
}

// Run categorizes the build failure using an LLM call.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}

	failureMessage := s.extractFailure(req)
	labels := strings.Join(req.Bead.Labels, ", ")
	if labels == "" {
		labels = "(none)"
	}

	instance := buildTriageInstance(req.Bead.ID, req.Bead.Title, labels, failureMessage)
	promptText := prompt.NewPromptAssembler(s.base, s.project, instance, s.fragment).Assemble("triage", prompt.BeadInfo{Title: req.Bead.Title})

	provider := s.llm
	if req.Provider != nil {
		provider = req.Provider
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = s.cfg.Models.P2
		if model == "" {
			model = config.ModelHaiku
		}
	}

	resp, err := provider.Invoke(ctx, llmtypes.LLMInvokeRequest{Prompt: promptText, Model: model, Dir: req.Worktree})
	if err != nil {
		return nil, fmt.Errorf("triage: invoking llm: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("triage: llm response nil")
	}
	if !resp.Success {
		return nil, fmt.Errorf("triage: llm invocation failed: %s", resp.Output)
	}

	var parsed triageResponse
	if err := jsonutil.ExtractJSON(resp.Output, &parsed); err != nil {
		return nil, fmt.Errorf("triage: parse response: %w", err)
	}

	category, err := validateCategory(parsed.Category)
	if err != nil {
		return nil, fmt.Errorf("triage: %w", err)
	}

	artifacts := &TriageArtifacts{
		Category:  category,
		Reasoning: parsed.Reasoning,
	}
	return &stagepkg.Result{Decision: stagepkg.DecisionProceed, Artifacts: artifacts}, nil
}

func buildTriageInstance(beadID, title, labels, failure string) string {
	return fmt.Sprintf("## Bead\n- ID: %s\n- Title: %s\n- Labels: %s\n\n## Failure\n%s", beadID, title, labels, failure)
}

func (s *Stage) extractFailure(req *stagepkg.Request) string {
	if req.RetryContext != nil && len(req.RetryContext.PriorFailures) > 0 {
		return req.RetryContext.PriorFailures[len(req.RetryContext.PriorFailures)-1]
	}
	// Fall back to bead title as context when no prior failures exist.
	if req.Bead.Title != "" {
		return fmt.Sprintf("No explicit failure message. Bead title: %s", req.Bead.Title)
	}
	return "No failure information available."
}

func validateCategory(raw string) (Category, error) {
	switch Category(raw) {
	case CategoryDecompose, CategoryRetry, CategoryUnclearSpec, CategoryUnsafe:
		return Category(raw), nil
	default:
		return "", fmt.Errorf("unknown category %q", raw)
	}
}
