package plan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
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
)

// Stage produces the initial implementation plan for a spec.
type Stage struct {
	name     string
	llm      llm.LLMProvider
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
func New(cfg *config.Config, provider llm.LLMProvider, base, project, fragment string) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if provider == nil {
		return nil, fmt.Errorf("llm provider required")
	}

	name := stagedesc.Describe("plan", cfg)
	return &Stage{
		name:     name,
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
	if req == nil || req.Config == nil {
		return nil, fmt.Errorf("config required")
	}

	specID := strings.TrimSpace(req.Bead.ID)
	if specID == "" {
		return nil, fmt.Errorf("spec ID required")
	}

	specPath, err := specFilePath(req.Config, specID)
	if err != nil {
		return nil, err
	}

	specData, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("read spec %s: %w", specID, err)
	}

	promptPayload := prompt.NewPromptAssembler(s.base, s.project, string(specData), s.fragment).Assemble()
	resp, err := s.llm.Invoke(ctx, llm.InvokeRequest{Prompt: promptPayload, Model: modelOpus})
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

	planText := resp.Output
	planPath, err := writePlanFile(req.Config, req.Worktree, planText)
	if err != nil {
		return nil, err
	}

	return &stagepkg.Result{
		Decision: stagepkg.DecisionProceed,
		Artifacts: &PlanArtifacts{
			SpecID: specID,
			Plan:   planText,
			Path:   planPath,
			Model:  modelOpus,
		},
	}, nil
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
