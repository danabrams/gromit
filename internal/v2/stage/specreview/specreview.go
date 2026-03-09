package specreview

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	legacyReview "github.com/danabrams/gromit/internal/review"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/prompt"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

const (
	defaultGromitDir  = ".gromit"
	v2DirName         = "v2"
	planFileName      = "plan.md"
	maxParseRetries   = 1
	retryInstructions = "IMPORTANT: Your previous response contained invalid JSON. Output ONLY the JSON object that represents the ReviewResult (no markdown, explanations, or code fences)."
)

const defaultSpecReviewFragment = `# Spec-Level Code Review Instructions

You are performing a holistic review of all changes made during this spec's implementation.
This review evaluates the CUMULATIVE diff — the combined output of all beads in the spec.

## Review Dimensions

### 1. Correctness
- Does the code work beyond the test coverage?
- Are error conditions handled?
- Edge cases not accounted for?

### 2. Security (OWASP Top 10)
- SQL/command/template injection risks?
- Authentication/authorization bypass?
- Data exposure, logging of secrets, missing input validation?

### 3. Error Handling
- Are errors propagated, not swallowed?
- Are sentinel errors used for callers to distinguish?
- Missing nil checks on external returns?

### 4. Test Coverage Gaps
- Untested code paths?
- Missing edge case tests?
- Are tests asserting behavior, or just coverage?

### 5. Code Quality
- Dead code, unused imports?
- Overly complex logic that should be simplified?
- Naming convention violations?

### 6. Architectural Fit
- Does new code follow the project's existing patterns?
- Are packages used at the right abstraction level?
- Does new behavior belong in the right layer?

## Scope Classification

For each finding, classify scope:
- "spec": the issue is in code introduced or modified by this spec
- "general": the issue exists in pre-existing code this spec did not touch

## Output Format

Respond with ONLY a JSON object:

{"verdict":"pass","findings":[{"severity":"critical","category":"bug","scope":"spec","description":"...","affected_files":["path/file.go"]}]}

Verdict rules:
- "fail" if ANY finding has severity "critical"
- "pass" if all findings are "warning" or "suggestion" (or no findings)

severity values: "critical", "warning", "suggestion"
category values: "bug", "security", "quality", "test-gap", "architecture"
scope values: "spec", "general"

Respond with ONLY the JSON object. No markdown wrapper, no explanation.
`

// GitDiffer provides the diff-from-base capability needed by specreview.
type GitDiffer interface {
	DiffFromBase(ctx context.Context, worktree string) (string, error)
}

// SpecReviewArtifacts carries the review result produced by the stage.
type SpecReviewArtifacts struct {
	Result   *legacyReview.ReviewResult
	PlanPath string
	Plan     string
	Diff     string
}

// Stage executes the spec-level review stage.
type Stage struct {
	name     string
	cfg      *config.Config
	git      GitDiffer
	llm      llmtypes.LLMProvider
	base     string
	project  string
	fragment string
}

var _ stagepkg.Stage = (*Stage)(nil)

// New constructs the specreview stage with the provided dependencies.
func New(cfg *config.Config, git GitDiffer, provider llmtypes.LLMProvider, base, project, fragment string) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if git == nil {
		return nil, fmt.Errorf("git adapter required")
	}
	if provider == nil {
		return nil, fmt.Errorf("llm provider required")
	}
	if strings.TrimSpace(fragment) == "" {
		fragment = defaultSpecReviewFragment
	}
	return &Stage{
		name:     stagedesc.Describe("specreview", cfg),
		cfg:      cfg,
		git:      git,
		llm:      provider,
		base:     base,
		project:  project,
		fragment: fragment,
	}, nil
}

// Name returns the stage identifier.
func (s *Stage) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Run executes the spec-level review using the highest-tier model.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}
	cfg, err := s.resolveConfig(req)
	if err != nil {
		return nil, err
	}
	specID := strings.TrimSpace(req.Bead.ID)
	if specID == "" {
		return nil, fmt.Errorf("spec ID required")
	}

	root := s.resolveRoot(req, cfg)

	planPath := s.planPath(root, cfg)
	planData, err := os.ReadFile(planPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("specreview: plan not found: %s", planPath)
		}
		return nil, fmt.Errorf("specreview: read plan: %w", err)
	}

	diff, err := s.git.DiffFromBase(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("specreview: git diff: %w", err)
	}

	instance := buildInstance(diff, string(planData))
	provider := s.llm
	if req.Provider != nil {
		provider = req.Provider
	}
	model := s.selectModel(cfg, req)
	fragment := s.fragment

	for attempt := 0; attempt <= maxParseRetries; attempt++ {
		promptText := prompt.NewPromptAssembler(s.base, s.project, instance, fragment).
			Assemble("specreview", prompt.BeadInfo{Title: specID})

		resp, invokeErr := provider.Invoke(ctx, llmtypes.LLMInvokeRequest{Prompt: promptText, Model: model, Dir: root})
		if invokeErr != nil {
			return nil, fmt.Errorf("specreview: invoke: %w", invokeErr)
		}
		if resp == nil {
			return nil, fmt.Errorf("specreview: nil response")
		}
		if !resp.Success {
			detail := strings.TrimSpace(resp.Output)
			if detail == "" {
				detail = "no detail available"
			}
			return nil, fmt.Errorf("specreview: invocation failed: %s", detail)
		}

		reviewResult, parseErr := legacyReview.ParseReviewResult(resp.Output)
		if parseErr == nil {
			artifacts := &SpecReviewArtifacts{
				Result:   reviewResult,
				PlanPath: planPath,
				Plan:     string(planData),
				Diff:     diff,
			}
			decision := stagepkg.DecisionProceed
			if reviewResult != nil && !reviewResult.Passed {
				decision = stagepkg.DecisionFail
			}
			return &stagepkg.Result{Decision: decision, Artifacts: artifacts}, nil
		}
		if attempt == maxParseRetries {
			return nil, fmt.Errorf("specreview: parse response: %w", parseErr)
		}
		fragment = appendRetryInstructions(s.fragment)
	}

	return nil, fmt.Errorf("specreview: unexpected retry handling")
}

func (s *Stage) resolveConfig(req *stagepkg.Request) (*config.Config, error) {
	cfg := req.Config
	if cfg == nil {
		cfg = s.cfg
	}
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	return cfg, nil
}

func (s *Stage) resolveRoot(req *stagepkg.Request, cfg *config.Config) string {
	if root := strings.TrimSpace(req.Worktree); root != "" {
		return root
	}
	if cfg != nil && strings.TrimSpace(cfg.ProjectRoot) != "" {
		return cfg.ProjectRoot
	}
	return "."
}

func (s *Stage) planPath(root string, cfg *config.Config) string {
	gromitDir := cfg.Paths.GromitDir
	if gromitDir == "" {
		gromitDir = defaultGromitDir
	}
	if root == "" {
		root = "."
	}
	return filepath.Join(root, gromitDir, v2DirName, planFileName)
}

func (s *Stage) selectModel(cfg *config.Config, req *stagepkg.Request) string {
	if req != nil {
		if trimmed := strings.TrimSpace(req.Model); trimmed != "" {
			return trimmed
		}
	}
	if cfg != nil {
		if trimmed := strings.TrimSpace(cfg.Models.P0); trimmed != "" {
			return trimmed
		}
	}
	return config.ModelOpus
}

func buildInstance(diff, plan string) string {
	var sections []string
	if trimmed := strings.TrimSpace(diff); trimmed != "" {
		sections = append(sections, fmt.Sprintf("## Cumulative Diff\n\n%s", trimmed))
	} else {
		sections = append(sections, "## Cumulative Diff\n\n(no changes)")
	}
	if trimmed := strings.TrimSpace(plan); trimmed != "" {
		sections = append(sections, fmt.Sprintf("## Plan\n\n%s", trimmed))
	}
	return strings.Join(sections, "\n\n")
}

func appendRetryInstructions(fragment string) string {
	trimmed := strings.TrimSpace(fragment)
	if trimmed == "" {
		return retryInstructions
	}
	return trimmed + "\n\n" + retryInstructions
}
