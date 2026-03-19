package accept

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/coverage"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/presentation"
	v2prompt "github.com/danabrams/gromit/internal/v2/prompt"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

const (
	defaultGromitDir    = ".gromit"
	v2DirName           = "v2"
	gapFileName         = "gap-analysis.md"
	defaultSpecsDir     = ".gromit/specs"
	defaultPromptBase   = "You are evaluating a single acceptance criterion. Use the provided diff and criterion text to determine whether the implementation satisfies the criterion. Respond with a JSON object containing \"pass\" (true/false) and \"summary\" (explain your reasoning). Output only the JSON object."
	maxEvalParseRetries = 1
	outputPreviewMaxLen = 500
	// batchDiffThreshold is the diff size (in bytes) above which the accept
	// stage switches from per-criterion invocations to a single batch
	// invocation that evaluates all criteria in one pass over the diff.
	batchDiffThreshold = 50000 // ~12K tokens
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

// GetAcceptanceResults returns the acceptance results, or nil if the receiver is nil.
func (a *AcceptArtifacts) GetAcceptanceResults() []presentation.AcceptanceResult {
	if a == nil {
		return nil
	}
	return a.Results
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
	name               string
	cfg                *config.Config
	git                GitDiffer
	llm                llmtypes.LLMProvider
	base               string
	project            string
	fragment           string
	batchDiffThreshold int // 0 means use default
	events.EmitterMixin
}

// diffThreshold returns the effective batch diff threshold.
// A negative value disables both batch and targeted evaluation
// (forces per-criterion mode for all diffs).
func (s *Stage) diffThreshold() int {
	if s.batchDiffThreshold != 0 {
		return s.batchDiffThreshold
	}
	return batchDiffThreshold
}

// WithEmitter attaches an emitter for logging criterion evaluations.
func (s *Stage) WithEmitter(emitter *events.Emitter) *Stage {
	s.EmitterMixin.SetEmitter(emitter)
	return s
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

	// Small diffs: evaluate all criteria in one invocation (cheap, fast).
	// Large diffs: first identify relevant files per criterion, then evaluate
	// each criterion with only its relevant portion of the diff.
	threshold := s.diffThreshold()
	if threshold > 0 {
		if len(diff) <= threshold {
			s.Log("info", "accept: diff is %d bytes (≤%d threshold), using batch evaluation for %d criteria", len(diff), threshold, len(criteria))
			return s.runBatchEvaluation(ctx, provider, specID, criteria, diff, cfg, req, root)
		}

		s.Log("info", "accept: diff is %d bytes (>%d threshold), using targeted evaluation for %d criteria", len(diff), threshold, len(criteria))
		results, failures, evalErr := s.runTargetedEvaluation(ctx, provider, specID, criteria, diff, cfg, req)
		if evalErr != nil {
			return nil, evalErr
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

	// Per-criterion fallback (threshold disabled or negative).
	results, failures, evalErr := s.runPerCriterionEvaluation(ctx, provider, specID, criteria, diff, cfg, req)
	if evalErr != nil {
		return nil, evalErr
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

// runPerCriterionEvaluation evaluates each criterion in a separate LLM invocation.
// Used when the diff is small enough that per-criterion calls are practical.
func (s *Stage) runPerCriterionEvaluation(ctx context.Context, provider llmtypes.LLMProvider, specID string, criteria []coverage.Criterion, diff string, cfg *config.Config, req *stagepkg.Request) ([]presentation.AcceptanceResult, []string, error) {
	results := make([]presentation.AcceptanceResult, 0, len(criteria))
	failures := make([]string, 0)

	for _, criterion := range criteria {
		if ctx.Err() != nil {
			return nil, nil, fmt.Errorf("accept interrupted: %w", ctx.Err())
		}

		trimmed := strings.TrimSpace(criterion.Text)
		if trimmed == "" {
			trimmed = fmt.Sprintf("criterion %d", criterion.Number)
		}

		s.Log("info", "accept: evaluating criterion %d/%d: %s", criterion.Number, len(criteria), trimmed)
		start := time.Now()

		pass, summary, evalErr := s.evaluateCriterion(ctx, provider, specID, criterion, diff, cfg, req)
		elapsed := time.Since(start)

		if evalErr != nil {
			if ctx.Err() != nil {
				return nil, nil, fmt.Errorf("accept interrupted during criterion %d: %w", criterion.Number, evalErr)
			}
			if isDeadlineExceeded(evalErr) {
				s.Log("info", "accept: criterion %d timed out after %s, marking FAIL", criterion.Number, elapsed.Truncate(time.Second))
				pass = false
				summary = fmt.Sprintf("evaluation timed out after %s", elapsed.Truncate(time.Second))
			} else {
				return nil, nil, fmt.Errorf("evaluate criterion %d: %w", criterion.Number, evalErr)
			}
		}

		score := "PASS"
		if !pass {
			score = "FAIL"
			failures = append(failures, fmt.Sprintf("Criterion %d failed: %s — %s", criterion.Number, trimmed, summaryOrDefault(summary)))
		}

		s.Log("info", "accept: criterion %d %s (%s)", criterion.Number, score, elapsed.Truncate(time.Second))

		results = append(results, presentation.AcceptanceResult{
			Title:       trimmed,
			Description: fmt.Sprintf("%s: %s", score, summaryOrDefault(summary)),
		})
	}

	return results, failures, nil
}

// runBatchEvaluation evaluates all criteria in a single LLM invocation.
// Used for large diffs to send the diff only once instead of N times.
// runTargetedEvaluation handles large diffs by first asking the LLM to map
// criteria to relevant files (cheap — only diff stat), then evaluating each
// criterion with only its relevant diff hunks.
func (s *Stage) runTargetedEvaluation(ctx context.Context, provider llmtypes.LLMProvider, specID string, criteria []coverage.Criterion, diff string, cfg *config.Config, req *stagepkg.Request) ([]presentation.AcceptanceResult, []string, error) {
	model := s.selectModel(cfg, req)
	fileDiffs := splitDiffByFile(diff)

	// Build a file list from the diff for the mapping phase.
	fileNames := make([]string, 0, len(fileDiffs))
	for name := range fileDiffs {
		fileNames = append(fileNames, name)
	}

	// Phase 1: Map criteria to relevant files using a cheap LLM call.
	fileMapping, err := s.mapCriteriaToFiles(ctx, provider, model, specID, criteria, fileNames, req)
	if err != nil {
		if isDeadlineExceeded(err) {
			s.Log("info", "accept: criteria-to-file mapping timed out, falling back to full diff per criterion")
			// Fall back to per-criterion with full diff (original behavior).
			return s.runPerCriterionEvaluation(ctx, provider, specID, criteria, diff, cfg, req)
		}
		return nil, nil, fmt.Errorf("map criteria to files: %w", err)
	}

	// Phase 2: Evaluate each criterion with only its relevant diff.
	results := make([]presentation.AcceptanceResult, 0, len(criteria))
	failures := make([]string, 0)

	for _, criterion := range criteria {
		if ctx.Err() != nil {
			return nil, nil, fmt.Errorf("accept interrupted: %w", ctx.Err())
		}

		trimmed := strings.TrimSpace(criterion.Text)
		if trimmed == "" {
			trimmed = fmt.Sprintf("criterion %d", criterion.Number)
		}

		// Build a targeted diff with only the relevant files.
		relevantFiles := fileMapping[criterion.Number]
		targetedDiff := buildTargetedDiff(relevantFiles, fileDiffs)

		s.Log("info", "accept: evaluating criterion %d/%d with %d/%d files: %s",
			criterion.Number, len(criteria), len(relevantFiles), len(fileDiffs), trimmed)
		start := time.Now()

		pass, summary, evalErr := s.evaluateCriterion(ctx, provider, specID, criterion, targetedDiff, cfg, req)
		elapsed := time.Since(start)

		if evalErr != nil {
			if ctx.Err() != nil {
				return nil, nil, fmt.Errorf("accept interrupted during criterion %d: %w", criterion.Number, evalErr)
			}
			if isDeadlineExceeded(evalErr) {
				s.Log("info", "accept: criterion %d timed out after %s, marking FAIL", criterion.Number, elapsed.Truncate(time.Second))
				pass = false
				summary = fmt.Sprintf("evaluation timed out after %s", elapsed.Truncate(time.Second))
			} else {
				return nil, nil, fmt.Errorf("evaluate criterion %d: %w", criterion.Number, evalErr)
			}
		}

		score := "PASS"
		if !pass {
			score = "FAIL"
			failures = append(failures, fmt.Sprintf("Criterion %d failed: %s — %s", criterion.Number, trimmed, summaryOrDefault(summary)))
		}

		s.Log("info", "accept: criterion %d %s (%s)", criterion.Number, score, elapsed.Truncate(time.Second))

		results = append(results, presentation.AcceptanceResult{
			Title:       trimmed,
			Description: fmt.Sprintf("%s: %s", score, summaryOrDefault(summary)),
		})
	}

	return results, failures, nil
}

// mapCriteriaToFiles asks the LLM to identify which files are relevant to each
// criterion based on the file list and criterion text. Returns a map from
// criterion number to list of relevant file paths.
func (s *Stage) mapCriteriaToFiles(ctx context.Context, provider llmtypes.LLMProvider, model, specID string, criteria []coverage.Criterion, files []string, req *stagepkg.Request) (map[int][]string, error) {
	var criteriaList strings.Builder
	for _, c := range criteria {
		text := strings.TrimSpace(c.Text)
		if text == "" {
			text = fmt.Sprintf("criterion %d", c.Number)
		}
		fmt.Fprintf(&criteriaList, "  %d. %s\n", c.Number, text)
	}

	prompt := fmt.Sprintf(`You are mapping acceptance criteria to relevant source files.

Spec: %s

Acceptance Criteria:
%s
Changed Files:
%s

For each criterion, identify which changed files are relevant to evaluating whether that criterion is satisfied. Include files that implement, test, or configure the behavior described by the criterion. When in doubt, include the file.

Output ONLY a JSON object mapping criterion numbers to arrays of file paths:
{"1": ["path/to/file.go", "path/to/other.go"], "2": ["path/to/file.go"], ...}

Do NOT output markdown or anything other than the JSON object.`,
		specID, criteriaList.String(), strings.Join(files, "\n"))

	resp, err := provider.Invoke(ctx, llmtypes.LLMInvokeRequest{Prompt: prompt, Model: model, Dir: req.Worktree})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("provider returned nil response")
	}
	if !resp.Success {
		return nil, fmt.Errorf("provider reported unsuccessful invocation")
	}

	// Parse the mapping response.
	var rawMapping map[string][]string
	if err := jsonutil.ExtractObject(strings.TrimSpace(resp.Output), &rawMapping); err != nil {
		// If parsing fails, fall back to including all files for every criterion.
		s.Log("info", "accept: failed to parse file mapping, using all files for every criterion: %v", err)
		result := make(map[int][]string, len(criteria))
		for _, c := range criteria {
			result[c.Number] = files
		}
		return result, nil
	}

	result := make(map[int][]string, len(criteria))
	for _, c := range criteria {
		key := fmt.Sprintf("%d", c.Number)
		if mapped, ok := rawMapping[key]; ok && len(mapped) > 0 {
			result[c.Number] = mapped
		} else {
			// No mapping found — include all files as safety fallback.
			result[c.Number] = files
		}
	}
	return result, nil
}

// splitDiffByFile splits a unified diff into per-file segments.
// Returns a map from file path to the diff content for that file.
func splitDiffByFile(diff string) map[string]string {
	result := make(map[string]string)
	segments := strings.Split(diff, "diff --git ")
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		// Extract file path from "a/path b/path" header.
		firstLine := seg
		if idx := strings.IndexByte(seg, '\n'); idx >= 0 {
			firstLine = seg[:idx]
		}
		// Parse "a/foo/bar.go b/foo/bar.go"
		parts := strings.Fields(firstLine)
		var filePath string
		if len(parts) >= 2 {
			filePath = strings.TrimPrefix(parts[1], "b/")
		} else if len(parts) == 1 {
			filePath = strings.TrimPrefix(parts[0], "a/")
		}
		if filePath == "" {
			continue
		}
		result[filePath] = "diff --git " + seg
	}
	return result
}

// buildTargetedDiff assembles a diff containing only the specified files.
func buildTargetedDiff(files []string, fileDiffs map[string]string) string {
	if len(files) == 0 {
		return "(no relevant files identified)"
	}
	var b strings.Builder
	for _, f := range files {
		if d, ok := fileDiffs[f]; ok {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(d)
		}
	}
	if b.Len() == 0 {
		// Files were mapped but not found in diff — include a note.
		return fmt.Sprintf("(mapped files not found in diff: %s)", strings.Join(files, ", "))
	}
	return b.String()
}

func (s *Stage) runBatchEvaluation(ctx context.Context, provider llmtypes.LLMProvider, specID string, criteria []coverage.Criterion, diff string, cfg *config.Config, req *stagepkg.Request, root string) (*stagepkg.Result, error) {
	model := s.selectModel(cfg, req)

	var criteriaList strings.Builder
	for _, c := range criteria {
		text := strings.TrimSpace(c.Text)
		if text == "" {
			text = fmt.Sprintf("criterion %d", c.Number)
		}
		fmt.Fprintf(&criteriaList, "  %d. %s\n", c.Number, text)
	}

	batchPrompt := fmt.Sprintf(`You are evaluating whether acceptance criteria have been satisfied by the implementation.

Spec: %s

Acceptance Criteria:
%s
Diff:
%s

## Instructions

Evaluate EACH criterion against the diff. For each criterion, determine:
- PASS: The diff clearly demonstrates the criterion is satisfied
- FAIL: The criterion is not met, only partially met, or does not match the spec's intent

## Output Format

Output ONLY a JSON array with one object per criterion, in order:
[{"criterion": 1, "pass": true, "summary": "Brief explanation."}, {"criterion": 2, "pass": false, "summary": "Brief explanation."}, ...]

Do NOT output markdown, commentary, or anything other than the JSON array.`,
		specID, criteriaList.String(), strings.TrimSpace(diff))

	instance := batchPrompt
	assembler := v2prompt.NewPromptAssembler(s.baseLayer(), s.project, instance, s.fragment)
	fullPrompt := assembler.Assemble("accept", v2prompt.BeadInfo{})

	s.Log("info", "accept: batch evaluating %d criteria in single invocation", len(criteria))
	start := time.Now()

	var lastOutput string
	for attempt := 0; attempt <= maxEvalParseRetries; attempt++ {
		currentPrompt := fullPrompt
		if attempt > 0 && lastOutput != "" {
			currentPrompt = buildBatchRepairPrompt(lastOutput, len(criteria))
		}
		resp, err := provider.Invoke(ctx, llmtypes.LLMInvokeRequest{Prompt: currentPrompt, Model: model, Dir: req.Worktree})
		elapsed := time.Since(start)

		if err != nil {
			if isDeadlineExceeded(err) {
				s.Log("info", "accept: batch evaluation timed out after %s, marking all criteria FAIL", elapsed.Truncate(time.Second))
				return s.allCriteriaFailed(criteria, fmt.Sprintf("batch evaluation timed out after %s", elapsed.Truncate(time.Second)), root, cfg)
			}
			return nil, fmt.Errorf("batch evaluate: %w", err)
		}
		if resp == nil {
			return nil, fmt.Errorf("batch evaluate: provider returned nil response")
		}
		if !resp.Success {
			detail := strings.TrimSpace(resp.Output)
			if detail == "" {
				detail = "no detail available"
			}
			return nil, fmt.Errorf("batch evaluate: provider reported unsuccessful invocation: %s", detail)
		}

		lastOutput = resp.Output
		batchResults, parseErr := parseBatchEvaluation(resp.Output, len(criteria))
		if parseErr == nil {
			s.Log("info", "accept: batch evaluation completed in %s", elapsed.Truncate(time.Second))
			return s.buildBatchResult(criteria, batchResults, root, cfg)
		}
		if attempt == maxEvalParseRetries {
			preview := lastOutput
			if len(preview) > outputPreviewMaxLen {
				preview = preview[:outputPreviewMaxLen] + "... (truncated)"
			}
			return nil, fmt.Errorf("parse batch evaluation: %w\nLLM output preview: %s", parseErr, preview)
		}
	}
	return nil, fmt.Errorf("unreachable: batch evaluation loop exited without result")
}

func (s *Stage) allCriteriaFailed(criteria []coverage.Criterion, reason string, root string, cfg *config.Config) (*stagepkg.Result, error) {
	results := make([]presentation.AcceptanceResult, 0, len(criteria))
	failures := make([]string, 0, len(criteria))
	for _, c := range criteria {
		text := strings.TrimSpace(c.Text)
		if text == "" {
			text = fmt.Sprintf("criterion %d", c.Number)
		}
		results = append(results, presentation.AcceptanceResult{
			Title:       text,
			Description: fmt.Sprintf("FAIL: %s", reason),
		})
		failures = append(failures, fmt.Sprintf("Criterion %d failed: %s — %s", c.Number, text, reason))
	}
	artifacts := &AcceptArtifacts{Results: results, GapSummary: strings.Join(failures, "\n")}
	if err := s.writeGapAnalysis(root, cfg, artifacts.GapSummary); err != nil {
		return nil, fmt.Errorf("write gap analysis: %w", err)
	}
	return &stagepkg.Result{Decision: stagepkg.DecisionFail, Artifacts: artifacts}, nil
}

type batchEvalResult struct {
	Criterion int    `json:"criterion"`
	Pass      bool   `json:"pass"`
	Summary   string `json:"summary"`
}

func parseBatchEvaluation(output string, expectedCount int) ([]batchEvalResult, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil, fmt.Errorf("batch evaluation output empty")
	}

	// Try parsing as array first.
	var results []batchEvalResult
	if err := jsonutil.ExtractObject(trimmed, &results); err == nil && len(results) > 0 {
		return results, nil
	}

	// Fall back to single object (common when there's only one criterion,
	// or when the LLM ignores the array instruction).
	var single batchEvalResult
	if err := jsonutil.ExtractObject(trimmed, &single); err != nil {
		return nil, fmt.Errorf("parse batch evaluation output: %w", err)
	}
	if single.Criterion == 0 {
		single.Criterion = 1
	}
	return []batchEvalResult{single}, nil
}

func (s *Stage) buildBatchResult(criteria []coverage.Criterion, batchResults []batchEvalResult, root string, cfg *config.Config) (*stagepkg.Result, error) {
	// Index batch results by criterion number for lookup.
	resultMap := make(map[int]batchEvalResult, len(batchResults))
	for _, br := range batchResults {
		resultMap[br.Criterion] = br
	}

	results := make([]presentation.AcceptanceResult, 0, len(criteria))
	failures := make([]string, 0)

	for _, c := range criteria {
		text := strings.TrimSpace(c.Text)
		if text == "" {
			text = fmt.Sprintf("criterion %d", c.Number)
		}

		br, found := resultMap[c.Number]
		pass := found && br.Pass
		summary := "no evaluation returned for this criterion"
		if found {
			summary = strings.TrimSpace(br.Summary)
		}

		score := "PASS"
		if !pass {
			score = "FAIL"
			failures = append(failures, fmt.Sprintf("Criterion %d failed: %s — %s", c.Number, text, summaryOrDefault(summary)))
		}

		s.Log("info", "accept: criterion %d %s", c.Number, score)

		results = append(results, presentation.AcceptanceResult{
			Title:       text,
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

func buildBatchRepairPrompt(previousOutput string, criteriaCount int) string {
	return fmt.Sprintf(`Your previous response was not valid JSON. Here is what you wrote:

---
%s
---

Please convert your evaluation above into ONLY a JSON array with one object per criterion (%d total), in order:
[{"criterion": 1, "pass": true, "summary": "Brief explanation."}, ...]

Output ONLY the JSON array, nothing else.`, previousOutput, criteriaCount)
}

// evaluateCriterion runs the LLM evaluation for a single criterion with retry on parse failure.
func (s *Stage) evaluateCriterion(ctx context.Context, provider llmtypes.LLMProvider, specID string, criterion coverage.Criterion, diff string, cfg *config.Config, req *stagepkg.Request) (bool, string, error) {
	promptText := s.buildPrompt(specID, criterion, diff)
	model := s.selectModel(cfg, req)

	var lastOutput string

	for attempt := 0; attempt <= maxEvalParseRetries; attempt++ {
		currentPrompt := promptText
		if attempt > 0 && lastOutput != "" {
			currentPrompt = buildRepairPrompt(lastOutput)
		}
		resp, err := provider.Invoke(ctx, llmtypes.LLMInvokeRequest{Prompt: currentPrompt, Model: model, Dir: req.Worktree})
		if err != nil {
			return false, "", err
		}
		if resp == nil {
			return false, "", fmt.Errorf("provider returned nil response")
		}
		if !resp.Success {
			detail := strings.TrimSpace(resp.Output)
			if detail == "" {
				detail = "no detail available"
			}
			return false, "", fmt.Errorf("provider reported unsuccessful invocation: %s", detail)
		}

		lastOutput = resp.Output
		pass, summary, parseErr := parseEvaluation(resp.Output)
		if parseErr == nil {
			return pass, summary, nil
		}
		if attempt == maxEvalParseRetries {
			preview := lastOutput
			if len(preview) > outputPreviewMaxLen {
				preview = preview[:outputPreviewMaxLen] + "... (truncated)"
			}
			return false, "", fmt.Errorf("parse evaluation: %w\nLLM output preview: %s", parseErr, preview)
		}
	}
	return false, "", fmt.Errorf("unreachable: evaluation loop exited without result")
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

// isDeadlineExceeded checks whether err represents a context deadline exceeded,
// using both errors.Is and string matching. Go's exec.CommandContext with custom
// Cancel functions can produce error chains where errors.Is misses the match.
func isDeadlineExceeded(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return strings.Contains(err.Error(), "context deadline exceeded")
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
