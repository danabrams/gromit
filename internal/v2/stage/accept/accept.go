package accept

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/presentation"
	v2prompt "github.com/danabrams/gromit/internal/v2/prompt"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

const (
	defaultGromitDir       = ".gromit"
	v2DirName              = "v2"
	gapFileName            = "gap-analysis.md"
	defaultSpecsDir        = ".gromit/specs"
	defaultPromptBase      = "You are evaluating a single acceptance criterion. Use the provided diff and criterion text to determine whether the implementation satisfies the criterion. Respond with a JSON object containing \"pass\" (true/false) and \"summary\" (explain your reasoning). Output only the JSON object."
	maxEvalParseRetries    = 1
	outputPreviewMaxLen    = 500
)

const defaultAcceptFragment = `# Acceptance Criterion Evaluation Instructions

You are evaluating whether a single acceptance criterion has been satisfied by the implementation.

## Evaluation Process

1. Read the criterion text carefully
2. Examine the diff for evidence that the criterion is met
3. Consider edge cases — does the implementation fully satisfy the criterion?
4. Check that the implementation matches the spirit of the criterion

## Decision Rules

- PASS: The diff clearly demonstrates the criterion is satisfied
- FAIL: The criterion is not met, only partially met, or does not match the spec's intent

## Output Format

Output ONLY a JSON object:
{"pass": true, "summary": "Brief explanation."}

Do NOT output markdown or anything other than the JSON object.
`

// GitDiffer provides the git diff capability needed by the accept stage.
type GitDiffer interface {
	Diff(ctx context.Context, worktree string) (string, error)
	DiffFromBase(ctx context.Context, worktree string) (string, error)
}

// AcceptArtifacts captures acceptance evaluation results produced by the stage.
type AcceptArtifacts struct {
	Results    []presentation.AcceptanceResult
	GapSummary string
}

// GetGapSummary returns the gap summary, or empty string if the receiver is nil.
func (a *AcceptArtifacts) GetGapSummary() string {
	if a == nil {
		return ""
	}
	return a.GapSummary
}

// Stage evaluates acceptance criteria against the current worktree.
type Stage struct {
	name     string
	cfg      *config.Config
	git      GitDiffer
	llm      llmtypes.LLMProvider
	base     string
	project  string
	fragment string
}

// New constructs an accept stage with the provided dependencies.
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
		fragment = defaultAcceptFragment
	}
	return &Stage{
		name:     stagedesc.Describe("accept", cfg),
		cfg:      cfg,
		git:      git,
		llm:      provider,
		base:     base,
		project:  project,
		fragment: fragment,
	}, nil
}

var _ stagepkg.Stage = (*Stage)(nil)

// Name returns the canonical accept stage identifier.
func (s *Stage) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Run executes the acceptance evaluation for each criterion.
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

	root := s.resolveRoot(req)
	specPath, err := specFilePath(cfg, root, specID)
	if err != nil {
		return nil, err
	}

	specData, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("read spec %s: %w", specID, err)
	}

	criteria, err := coverage.ParseCriteria(string(specData))
	if err != nil {
		return nil, fmt.Errorf("parse acceptance criteria: %w", err)
	}

	if len(criteria) == 0 {
		return &stagepkg.Result{
			Decision:  stagepkg.DecisionProceed,
			Artifacts: &AcceptArtifacts{},
		}, nil
	}

	diff, err := s.git.DiffFromBase(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	provider := s.llm
	if req.Provider != nil {
		provider = req.Provider
	}

	results := make([]presentation.AcceptanceResult, 0, len(criteria))
	failures := make([]string, 0)

	for _, criterion := range criteria {
		promptText := s.buildPrompt(specID, criterion, diff)
		model := s.selectModel(cfg, req)

		var pass bool
		var summary string
		var lastOutput string

		for attempt := 0; attempt <= maxEvalParseRetries; attempt++ {
			currentPrompt := promptText
			if attempt > 0 && lastOutput != "" {
				currentPrompt = buildRepairPrompt(lastOutput)
			}
			resp, err := provider.Invoke(ctx, llmtypes.LLMInvokeRequest{Prompt: currentPrompt, Model: model, Dir: req.Worktree})
			if err != nil {
				return nil, fmt.Errorf("evaluate criterion %d: %w", criterion.Number, err)
			}
			if resp == nil {
				return nil, fmt.Errorf("evaluate criterion %d: provider returned nil response", criterion.Number)
			}
			if !resp.Success {
				detail := strings.TrimSpace(resp.Output)
				if detail == "" {
					detail = "no detail available"
				}
				return nil, fmt.Errorf("evaluate criterion %d: provider reported unsuccessful invocation: %s", criterion.Number, detail)
			}

			lastOutput = resp.Output
			var parseErr error
			pass, summary, parseErr = parseEvaluation(resp.Output)
			if parseErr == nil {
				break
			}
			if attempt == maxEvalParseRetries {
				preview := lastOutput
				if len(preview) > outputPreviewMaxLen {
					preview = preview[:outputPreviewMaxLen] + "... (truncated)"
				}
				return nil, fmt.Errorf("parse criterion %d evaluation: %w\nLLM output preview: %s", criterion.Number, parseErr, preview)
			}
		}

		trimmed := strings.TrimSpace(criterion.Text)
		if trimmed == "" {
			trimmed = fmt.Sprintf("criterion %d", criterion.Number)
		}
		score := "PASS"
		if !pass {
			score = "FAIL"
			failures = append(failures, fmt.Sprintf("Criterion %d failed: %s — %s", criterion.Number, trimmed, summaryOrDefault(summary)))
		}

		results = append(results, presentation.AcceptanceResult{
			Title:       trimmed,
			Description: fmt.Sprintf("%s: %s", score, summaryOrDefault(summary)),
		})
	}

	artifacts := &AcceptArtifacts{Results: results}
	if len(failures) > 0 {
		gapSummary := strings.Join(failures, "\n")
		artifacts.GapSummary = gapSummary
		if err := s.writeGapAnalysis(root, cfg, gapSummary); err != nil {
			return nil, fmt.Errorf("write gap analysis: %w", err)
		}
		return &stagepkg.Result{Decision: stagepkg.DecisionFail, Artifacts: artifacts}, nil
	}

	return &stagepkg.Result{Decision: stagepkg.DecisionProceed, Artifacts: artifacts}, nil
}

func (s *Stage) resolveConfig(req *stagepkg.Request) (*config.Config, error) {
	if req != nil && req.Config != nil {
		return req.Config, nil
	}
	if s.cfg != nil {
		return s.cfg, nil
	}
	return nil, fmt.Errorf("config required")
}

func (s *Stage) resolveRoot(req *stagepkg.Request) string {
	if req != nil {
		if trimmed := strings.TrimSpace(req.Worktree); trimmed != "" {
			return trimmed
		}
	}
	if s.cfg != nil && strings.TrimSpace(s.cfg.ProjectRoot) != "" {
		return s.cfg.ProjectRoot
	}
	return "."
}

func specFilePath(cfg *config.Config, root, specID string) (string, error) {
	specDir := cfg.Paths.Specs
	if specDir == "" {
		specDir = defaultSpecsDir
	}
	specDir = resolveCandidatePath(root, specDir)

	name := specID
	if filepath.Ext(name) == "" {
		name += ".md"
	}
	return filepath.Join(specDir, name), nil
}

func (s *Stage) buildPrompt(specID string, criterion coverage.Criterion, diff string) string {
	instance := s.buildInstanceLayer(specID, criterion, diff)
	assembler := v2prompt.NewPromptAssembler(s.baseLayer(), s.project, instance, s.fragment)
	return assembler.Assemble("accept", v2prompt.BeadInfo{})
}

func (s *Stage) baseLayer() string {
	if trimmed := strings.TrimSpace(s.base); trimmed != "" {
		return trimmed
	}
	return defaultPromptBase
}

func (s *Stage) buildInstanceLayer(specID string, criterion coverage.Criterion, diff string) string {
	trimmed := strings.TrimSpace(criterion.Text)
	if trimmed == "" {
		trimmed = fmt.Sprintf("criterion %d", criterion.Number)
	}
	diffText := strings.TrimSpace(diff)
	if diffText == "" {
		diffText = "(no diff provided)"
	}
	return fmt.Sprintf("Spec: %s\nCriterion %d: %s\n\nDiff:\n%s", specID, criterion.Number, trimmed, diffText)
}

func (s *Stage) selectModel(cfg *config.Config, req *stagepkg.Request) string {
	if req != nil {
		if trimmed := strings.TrimSpace(req.Model); trimmed != "" {
			return trimmed
		}
	}
	if cfg != nil {
		if trimmed := strings.TrimSpace(cfg.SpecGate.Model); trimmed != "" {
			return trimmed
		}
		if trimmed := strings.TrimSpace(cfg.Models.P1); trimmed != "" {
			return trimmed
		}
	}
	return config.ModelSonnet
}

func buildRepairPrompt(previousOutput string) string {
	return fmt.Sprintf(`Your previous response was not valid JSON. Here is what you wrote:

---
%s
---

Please convert your evaluation above into ONLY a JSON object with this exact format:
{"pass": true, "summary": "Brief explanation."}

Set "pass" to true if the criterion is satisfied, false otherwise. Output ONLY the JSON object, nothing else.`, previousOutput)
}

func parseEvaluation(output string) (bool, string, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return false, "", fmt.Errorf("evaluation output empty")
	}
	var eval struct {
		Pass    bool   `json:"pass"`
		Summary string `json:"summary"`
	}

	if err := jsonutil.ExtractObject(trimmed, &eval); err != nil {
		return false, "", fmt.Errorf("parse evaluation output: %w", err)
	}
	return eval.Pass, strings.TrimSpace(eval.Summary), nil
}

func summaryOrDefault(summary string) string {
	if trimmed := strings.TrimSpace(summary); trimmed != "" {
		return trimmed
	}
	return "no additional details provided"
}

func (s *Stage) writeGapAnalysis(root string, cfg *config.Config, summary string) error {
	if strings.TrimSpace(summary) == "" {
		return nil
	}
	path := s.gapFilePath(root, cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create gap file dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(summary), 0o644); err != nil {
		return fmt.Errorf("write gap file: %w", err)
	}
	return nil
}

func (s *Stage) gapFilePath(root string, cfg *config.Config) string {
	gromitDir := cfg.Paths.GromitDir
	if gromitDir == "" {
		gromitDir = defaultGromitDir
	}
	gromitDir = resolveCandidatePath(root, gromitDir)
	return filepath.Join(gromitDir, v2DirName, gapFileName)
}

func resolveCandidatePath(root, candidate string) string {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		return root
	}
	if filepath.IsAbs(trimmed) {
		return trimmed
	}
	return filepath.Join(root, trimmed)
}
