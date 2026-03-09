package specreview

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/v2/llmtypes"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

const defaultSpecReviewFragment = `# Spec-Level Review Instructions

You are performing a holistic code review of the entire spec implementation and evaluate the cumulative diff plus telemetry and documentation that spans multiple beads or emerges when reviewing the end-to-end change.

Review Scope:
- Correctness: look for logic errors, off-by-one mistakes, faulty conditionals, and missing invariants.
- Security: injection, auth bypass, data exposure, unsafe defaults, and missing encryption or authentication checks.
- Error handling and resilience: unchecked errors, missing context propagation, goroutines without cancellation, and missing failure telemetry.
- Test coverage: missing regression guards or brittle fixtures that mask bugs.
- Code quality: dead code, duplicated logic, exported names without comments, and package contract violations.
- Architecture: broken contracts, nil-safety gaps, improper state storage, or telemetry drift.

Severity choices: critical (blocks the spec), warning (should be fixed before merge), suggestion (nice-to-have).
Category choices: bug, security, quality, test-gap, architecture, acceptance.
Scope choices: spec (files in the diff), general (outside the diff).

Verdict logic:
- The default verdict is pass unless one or more critical findings exist.
- If the LLM verdict says pass but there is any critical finding, force the overall verdict to fail.
- Warnings and suggestions can accompany a pass verdict, but note them explicitly.

Output Format:
Return ONLY a JSON object that matches the schema this stage reads. A sample object is
{
  "verdict": "pass",
  "findings": [
    {
      "severity": "critical",
      "category": "bug",
      "scope": "spec",
      "description": "Describe the issue and its impact.",
      "affected_files": ["path/to/file.go"]
    }
  ]
}
If there are no findings, return {"verdict": "pass", "findings": []}.
`

// GitDiffer provides the diff capability needed by the spec-review stage.
type GitDiffer interface {
	DiffFromBase(ctx context.Context, worktree string) (string, error)
}

// SpecReviewArtifacts captures the result of the spec-level review.
type SpecReviewArtifacts struct {
	Verdict  string
	Findings []stagepkg.Finding
}

// Stage wraps the dependencies for the spec review.
type Stage struct {
	git      GitDiffer
	llm      llmtypes.LLMProvider
	fragment string
}

// New constructs a spec review stage.
func New(git GitDiffer, llm llmtypes.LLMProvider, fragment string) (*Stage, error) {
	if git == nil {
		return nil, fmt.Errorf("git adapter required")
	}
	if llm == nil {
		return nil, fmt.Errorf("llm provider required")
	}
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		fragment = strings.TrimSpace(defaultSpecReviewFragment)
	}
	return &Stage{git: git, llm: llm, fragment: fragment}, nil
}

// Name returns the stage identifier.
func (s *Stage) Name() string {
	return "spec-review"
}

// Run executes the holistic spec-level review.
func (s *Stage) Run(ctx context.Context, req *stagepkg.StageRequest) (*stagepkg.StageResult, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}
	worktree := strings.TrimSpace(req.Worktree)
	if worktree == "" {
		return nil, fmt.Errorf("worktree required")
	}

	diff, err := s.git.DiffFromBase(ctx, worktree)
	if err != nil {
		return nil, fmt.Errorf("diff from base: %w", err)
	}

	planPath := filepath.Join(worktree, ".gromit", "v2", "plan.md")
	planBytes, err := os.ReadFile(planPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read plan: %w", err)
	}

	prompt := s.buildPrompt(string(planBytes), diff)
	resp, err := s.llm.Invoke(ctx, llmtypes.LLMInvokeRequest{
		Prompt: prompt,
		Model:  req.Tier,
		Dir:    req.Worktree,
	})
	if err != nil {
		return nil, fmt.Errorf("llm invoke: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("llm invoke: provider returned nil response")
	}
	if !resp.Success {
		detail := strings.TrimSpace(resp.Output)
		if detail == "" {
			detail = "llm invocation failed"
		}
		return nil, fmt.Errorf("llm invoke unsuccessful: %s", detail)
	}

	artifacts, err := parseReviewOutput(resp.Output)
	if err != nil {
		return nil, fmt.Errorf("parse review output: %w", err)
	}

	artifacts.Verdict = strings.ToLower(strings.TrimSpace(artifacts.Verdict))
	if artifacts.Verdict != "pass" {
		artifacts.Verdict = "fail"
	}

	for _, finding := range artifacts.Findings {
		if finding.Severity == stagepkg.FindingSeverityCritical {
			artifacts.Verdict = "fail"
			break
		}
	}

	decision := stagepkg.DecisionProceed
	if artifacts.Verdict == "fail" {
		decision = stagepkg.DecisionFail
	}

	return &stagepkg.StageResult{
		Decision:  decision,
		Artifacts: artifacts,
	}, nil
}

func (s *Stage) buildPrompt(plan, diff string) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(s.fragment))
	sb.WriteString("\n\n## Plan\n\n")
	sb.WriteString(plan)
	sb.WriteString("\n\n## Cumulative Diff\n\n```diff\n")
	sb.WriteString(diff)
	sb.WriteString("\n```\n")
	return sb.String()
}

func parseReviewOutput(raw string) (*SpecReviewArtifacts, error) {
	raw = strings.TrimSpace(raw)
	var parsed llmReviewOutput
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("json: %w (raw: %.200s)", err, raw)
	}

	findings := make([]stagepkg.Finding, 0, len(parsed.Findings))
	for _, f := range parsed.Findings {
		findings = append(findings, stagepkg.Finding{
			Severity:      stagepkg.FindingSeverity(strings.ToLower(strings.TrimSpace(f.Severity))),
			Category:      stagepkg.FindingCategory(strings.ToLower(strings.TrimSpace(f.Category))),
			Scope:         stagepkg.FindingScope(strings.ToLower(strings.TrimSpace(f.Scope))),
			Description:   f.Description,
			AffectedFiles: f.AffectedFiles,
		})
	}

	verdict := strings.ToLower(strings.TrimSpace(parsed.Verdict))
	if verdict == "" {
		verdict = "fail"
	}

	return &SpecReviewArtifacts{Verdict: verdict, Findings: findings}, nil
}

type llmReviewOutput struct {
	Verdict  string `json:"verdict"`
	Findings []struct {
		Severity      string   `json:"severity"`
		Category      string   `json:"category"`
		Scope         string   `json:"scope"`
		Description   string   `json:"description"`
		AffectedFiles []string `json:"affected_files"`
	} `json:"findings"`
}
