package specreview

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/prompt"
	"github.com/danabrams/gromit/internal/v2/routing"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	"github.com/danabrams/gromit/internal/v2/stage/finding"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

const maxParseRetries = 1

// GitDiffer exposes the diff management needed for the stage.
type GitDiffer interface {
	DiffFromBase(ctx context.Context, worktree string) (string, error)
}

// SpecReviewArtifacts captures the verdict and findings emitted by the stage.
type SpecReviewArtifacts struct {
	Verdict  string
	Findings []finding.Finding
}

// Stage implements the spec-level review stage.
type Stage struct {
	name     string
	cfg      *config.Config
	git      GitDiffer
	llm      llmtypes.LLMProvider
	base     string
	project  string
	fragment string
	emitter  *event.Emitter
}

// New constructs a spec review stage backed by the provided dependencies.
func New(cfg *config.Config, git GitDiffer, provider llmtypes.LLMProvider, base, project, fragment string) (*Stage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config required")
	}
	if git == nil {
		return nil, fmt.Errorf("git differ required")
	}
	if provider == nil {
		return nil, fmt.Errorf("llm provider required")
	}
	if strings.TrimSpace(fragment) == "" {
		return nil, fmt.Errorf("fragment required")
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

// WithTypedEmitter attaches the provided typed event emitter.
func (s *Stage) WithTypedEmitter(emitter *event.Emitter) *Stage {
	if s == nil {
		return s
	}
	s.emitter = emitter
	return s
}

var _ stagepkg.Stage = (*Stage)(nil)

// Name returns the canonical stage identifier.
func (s *Stage) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Run executes the spec-level review stage.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}
	if req.Bead.ID == "" {
		return nil, fmt.Errorf("bead metadata required")
	}
	root := s.resolveRoot(req)
	diff, err := s.git.DiffFromBase(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("spec review: diff from base: %w", err)
	}
	planText, err := s.readPlan(root)
	if err != nil {
		return nil, fmt.Errorf("spec review: %w", err)
	}

	instance := buildInstanceLayer(diff, planText)
	assembler := prompt.NewPromptAssembler(s.base, s.project, instance, s.fragment)
	promptText := assembler.Assemble("review", prompt.BeadInfo{Title: req.Bead.Title})

	provider := s.llm
	if req.Provider != nil {
		provider = req.Provider
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = config.ModelOpus
	}

	dir := strings.TrimSpace(req.Worktree)
	metadata := map[string]string{"tier": routing.TierHigh}

	var parsed *specReviewResponse
	var lastOutput string
	for attempt := 0; attempt <= maxParseRetries; attempt++ {
		currentPrompt := promptText
		if attempt > 0 && lastOutput != "" {
			currentPrompt = buildRepairPrompt(lastOutput)
		}

		resp, err := provider.Invoke(ctx, llmtypes.LLMInvokeRequest{
			Prompt:   currentPrompt,
			Model:    model,
			Dir:      dir,
			Metadata: metadata,
		})
		if err != nil {
			return nil, fmt.Errorf("spec review: invoking llm: %w", err)
		}
		if resp == nil {
			return nil, fmt.Errorf("spec review: llm response nil")
		}
		if !resp.Success {
			detail := strings.TrimSpace(resp.Output)
			if detail == "" {
				detail = "no detail available"
			}
			return nil, fmt.Errorf("spec review: llm invocation failed: %s", detail)
		}

		lastOutput = resp.Output
		parsed, err = parseResponse(resp.Output)
		if err == nil {
			break
		}
		if attempt == maxParseRetries {
			preview := resp.Output
			if len(preview) > 500 {
				preview = preview[:500] + "... (truncated)"
			}
			return nil, fmt.Errorf("spec review: parse response: %w\nLLM output preview: %s", err, preview)
		}
	}

	findings := convertFindings(parsed.Findings)
	verdict := determineVerdict(findings)
	success := verdict == "pass"
	artifacts := &SpecReviewArtifacts{Verdict: verdict, Findings: findings}
	decision := stagepkg.DecisionProceed
	if !success {
		decision = stagepkg.DecisionFail
	}
	s.emitCompletion(req.Bead.ID, root, verdict, success, findings)

	return &stagepkg.Result{Decision: decision, Artifacts: artifacts}, nil
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

func (s *Stage) readPlan(root string) (string, error) {
	path := filepath.Join(root, ".gromit", "v2", "plan.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read plan: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func buildInstanceLayer(diff, plan string) string {
	var sections []string
	if trimmed := strings.TrimSpace(diff); trimmed != "" {
		sections = append(sections, fmt.Sprintf("## Diff\n%s", trimmed))
	}
	if trimmed := strings.TrimSpace(plan); trimmed != "" {
		sections = append(sections, fmt.Sprintf("## Plan\n%s", trimmed))
	}
	return strings.Join(sections, "\n\n")
}

type specReviewResponse struct {
	Verdict  string              `json:"verdict"`
	Findings []specReviewFinding `json:"findings"`
}

type specReviewFinding struct {
	Severity      string   `json:"severity"`
	Category      string   `json:"category"`
	Scope         string   `json:"scope"`
	Description   string   `json:"description"`
	AffectedFiles []string `json:"affected_files"`
}

func parseResponse(output string) (*specReviewResponse, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil, fmt.Errorf("response empty")
	}
	var resp specReviewResponse
	if err := jsonutil.ExtractObject(trimmed, &resp); err != nil {
		return nil, fmt.Errorf("extract json: %w", err)
	}
	if resp.Findings == nil {
		resp.Findings = []specReviewFinding{}
	}
	return &resp, nil
}

func convertFindings(src []specReviewFinding) []finding.Finding {
	out := make([]finding.Finding, 0, len(src))
	for _, raw := range src {
		f := finding.Finding{
			Severity:      finding.Severity(strings.ToLower(strings.TrimSpace(raw.Severity))),
			Category:      finding.Category(strings.ToLower(strings.TrimSpace(raw.Category))),
			Scope:         strings.TrimSpace(raw.Scope),
			Description:   strings.TrimSpace(raw.Description),
			AffectedFiles: append([]string(nil), raw.AffectedFiles...),
		}
		f.NormalizeNilFields()
		out = append(out, f)
	}
	return out
}

func determineVerdict(findings []finding.Finding) string {
	if finding.HasCritical(findings) {
		return "fail"
	}
	return "pass"
}

func buildRepairPrompt(previous string) string {
	return fmt.Sprintf(`Your previous response was not valid JSON.
Here is what you wrote:

---
%s
---

Please respond with ONLY a JSON object that matches this schema exactly:
{"verdict":"pass|fail","findings":[{"severity":"critical|warning|suggestion","category":"bug|security|quality|test_gap|architecture|acceptance","scope":"spec:<id>|general","description":"...","affected_files":["..."]}]}
`, previous)
}

func (s *Stage) emitCompletion(specID, worktree, verdict string, success bool, findings []finding.Finding) {
	if s.emitter == nil {
		return
	}
	critical := 0
	for _, f := range findings {
		if f.Severity == finding.SeverityCritical {
			critical++
		}
	}
	s.emitter.Emit(&event.SpecReviewCompletedEvent{
		Event: event.Event{
			SchemaVersion: event.SchemaVersion,
			Timestamp:     time.Now(),
			Type:          event.EventTypeSpecReviewCompleted,
		},
		SpecID:           specID,
		Worktree:         worktree,
		Verdict:          verdict,
		FindingCount:     len(findings),
		CriticalFindings: critical,
		Success:          success,
	})
}
