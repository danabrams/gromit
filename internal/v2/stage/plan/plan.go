package plan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/prompt"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

const (
	defaultSpecsDir  = ".gromit/specs"
	defaultGromitDir = ".gromit"
	planDirName      = "v2"
	planFileName     = "plan.md"
	modelOpus        = "opus"
	maxPlanRetries   = 1
)

// planInstructions provides non-interactive plan generation instructions.
// This is used as the FRAGMENT layer when no external fragment is provided.
const planInstructions = `# Plan Generation Instructions

You are generating an implementation plan for a specification. You are running in non-interactive mode — produce the plan directly without asking questions or requesting confirmation.

## Output Format

Output ONLY the plan content in this structure:

` + "```" + `markdown
---
id: <spec-name>
source_spec: <spec-name>
created: <YYYY-MM-DD>
decomposed: false
---

# <Title> Implementation Plan

**Goal:** [1-sentence summary]
**Architecture:** [1-2 sentence approach summary]
**Spec:** ` + "`" + `.gromit/specs/<spec-name>.md` + "`" + `

---

## Architecture

[High-level architecture: components, integration points, data flow, files to modify/create, tradeoffs]

## Test Strategy

[Test levels, key test cases, mocking strategy, coverage goals]

## Implementation Tasks

### Task 1: [Title]
**Files:** [files to modify/create/test]
**What to Do:** [clear description]
**Acceptance Criteria:** [1-3 concrete, testable criteria]
**Dependencies:** [other tasks this depends on, if any]

### Task 2: [Title]
...
` + "```" + `

## Task Sizing Rules

- One concern per task — a single file or two tightly coupled files
- 1-3 acceptance criteria per task — split if more than 3
- Max 2-3 files touched per task
- Start with foundational tasks (types, interfaces, core logic)
- Never split natural units: interface + implementation + mock, implementation + tests
- Make dependencies explicit

## Important

- Do NOT ask questions or request confirmation
- Do NOT suggest next steps or ask to execute
- Output ONLY the plan markdown content
- Explore the spec thoroughly and produce a complete, actionable plan
- Do NOT use Write, Edit, or any file-writing tools — output the plan as your text response
`

// Stage produces the initial implementation plan for a spec.
type Stage struct {
	name     string
	cfg      *config.Config
	llm      llmtypes.LLMProvider
	base     string
	project  string
	fragment string
}

var _ stagepkg.Stage = (*Stage)(nil)

// PlanArtifacts expose the generated plan and metadata from the stage.
type PlanArtifacts struct {
	SpecID string
	Plan   string
	Path   string
	Model  string
}

// New constructs a plan stage backed by the provided configuration.
func New(cfg *config.Config, provider llmtypes.LLMProvider, base, project, fragment string) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if provider == nil {
		return nil, fmt.Errorf("llm provider required")
	}

	if strings.TrimSpace(fragment) == "" {
		fragment = planInstructions
	}

	name := stagedesc.Describe("plan", cfg)
	return &Stage{
		name:     name,
		cfg:      cfg,
		llm:      provider,
		base:     base,
		project:  project,
		fragment: fragment,
	}, nil
}

// Name returns the stage identifier consumed by the loop.
func (s *Stage) Name() string {
	return s.name
}

// Run reads the spec, assembles the prompt, invokes the LLM, and persists the plan.
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

	specPath, err := specFilePath(cfg, specID)
	if err != nil {
		return nil, err
	}

	specData, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("read spec %s: %w", specID, err)
	}

	model := s.selectModel(req, cfg)

	provider := s.llm
	if req.Provider != nil {
		provider = req.Provider
	}

	retrySuffix := ""
	var planText string
	var lastValidationErr error
	for attempt := 0; attempt <= maxPlanRetries; attempt++ {
		promptPayload := prompt.NewPromptAssembler(s.base, s.project, string(specData), s.fragment+retrySuffix).Assemble("plan", prompt.BeadInfo{Title: req.Bead.Title})
		resp, err := provider.Invoke(ctx, llmtypes.LLMInvokeRequest{Prompt: promptPayload, Model: model, Dir: req.Worktree})
		if err != nil {
			return nil, fmt.Errorf("invoke llm: %w", err)
		}
		if resp == nil {
			return nil, fmt.Errorf("invoke llm: provider returned nil response")
		}
		if !resp.Success {
			detail := strings.TrimSpace(resp.Output)
			if detail == "" {
				detail = "no detail available"
			}
			return nil, fmt.Errorf("invoke llm: provider reported unsuccessful invocation: %s", detail)
		}

		planText = resp.Output
		lastValidationErr = ValidatePlanContent(planText)
		if lastValidationErr == nil {
			break
		}
		if attempt < maxPlanRetries {
			retrySuffix = "\n\nIMPORTANT: Your previous response did not contain a valid plan. You MUST output the complete plan as your text response. Do NOT use Write, Edit, or any file-writing tools. Do NOT summarize or reference other files. Output the full plan markdown directly."
		}
	}
	if lastValidationErr != nil {
		preview := strings.TrimSpace(planText)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("%w; after %d retries, got: %q", lastValidationErr, maxPlanRetries, preview)
	}

	planPath, err := writePlanFile(cfg, req.Worktree, planText)
	if err != nil {
		return nil, err
	}

	return &stagepkg.Result{
		Decision: stagepkg.DecisionProceed,
		Artifacts: &PlanArtifacts{
			SpecID: specID,
			Plan:   planText,
			Path:   planPath,
			Model:  model,
		},
	}, nil
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

func (s *Stage) selectModel(req *stagepkg.Request, cfg *config.Config) string {
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
	return modelOpus
}

func specFilePath(cfg *config.Config, specID string) (string, error) {
	specDir := cfg.Paths.Specs
	if specDir == "" {
		specDir = defaultSpecsDir
	}
	specDir = resolvePath(cfg, specDir)

	specName := specID
	if !strings.EqualFold(filepath.Ext(specName), ".md") {
		specName += ".md"
	}

	return filepath.Join(specDir, specName), nil
}

var planSectionPattern = regexp.MustCompile(`(?m)^##[#]?\s+(Implementation Tasks|Task\s+\d|Architecture)`)

const minPlanLength = 200

// ValidatePlanContent checks that a plan has sufficient length and the
// expected section headers.  Exported so the spec-loop resume path can
// reject stale/corrupt plan files before handing them to decompose.
func ValidatePlanContent(content string) error {
	trimmed := strings.TrimSpace(content)
	if len(trimmed) < minPlanLength {
		preview := trimmed
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		return fmt.Errorf("plan content validation failed: output too short (%d chars, need %d); got: %q", len(trimmed), minPlanLength, preview)
	}
	if !planSectionPattern.MatchString(trimmed) {
		preview := trimmed
		if len(preview) > 120 {
			preview = preview[:120] + "..."
		}
		return fmt.Errorf("plan content validation failed: missing required section headers (expected '## Implementation Tasks', '### Task N', or '## Architecture'); got: %q", preview)
	}
	return nil
}

func writePlanFile(cfg *config.Config, worktree string, plan string) (string, error) {
	root := strings.TrimSpace(worktree)
	if root == "" {
		root = cfg.ProjectRoot
	}
	if root == "" {
		root = "."
	}

	gromitDir := cfg.Paths.GromitDir
	if gromitDir == "" {
		gromitDir = defaultGromitDir
	}
	if !filepath.IsAbs(gromitDir) {
		gromitDir = filepath.Join(root, gromitDir)
	}

	planDir := filepath.Join(gromitDir, planDirName)
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		return "", fmt.Errorf("create plan directory: %w", err)
	}

	planPath := filepath.Join(planDir, planFileName)
	if err := os.WriteFile(planPath, []byte(plan), 0o644); err != nil {
		return "", fmt.Errorf("write plan file: %w", err)
	}

	return planPath, nil
}

func resolvePath(cfg *config.Config, candidate string) string {
	if filepath.IsAbs(candidate) {
		return candidate
	}
	return filepath.Join(rootPath(cfg), candidate)
}

func rootPath(cfg *config.Config) string {
	if cfg.ProjectRoot != "" {
		return cfg.ProjectRoot
	}
	return "."
}
