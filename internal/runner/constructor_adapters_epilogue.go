package runner

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline/epilogue"
	"github.com/danabrams/gromit/internal/pipeline/prepare"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/runtypes"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/validate"
)

// beadLifecycleAdapter wraps tracker.Client to satisfy epilogue.BeadLifecycle.
type beadLifecycleAdapter struct {
	tracker tracker.Client
}

func (a *beadLifecycleAdapter) Close(ctx context.Context, id string) error {
	return a.tracker.Close(ctx, id)
}

func (a *beadLifecycleAdapter) Sync(ctx context.Context) error {
	return a.tracker.Sync(ctx)
}

// statusWriterAdapter wraps runner.StatusWriter to satisfy epilogue.StatusWriter.
type statusWriterAdapter struct {
	sw *StatusWriter
}

func (a *statusWriterAdapter) Write(iteration int, beadID, beadTitle, model string, maxIterations, timeBudgetMinutes int) error {
	if a.sw == nil {
		return nil
	}
	return a.sw.Write(iteration, beadID, beadTitle, model, true, maxIterations, timeBudgetMinutes)
}

// epilogueCommandRunnerAdapter wraps a command runner function to satisfy epilogue.CommandRunner.
type epilogueCommandRunnerAdapter struct {
	runner func(ctx context.Context, command, workDir string) (string, string, int, error)
}

func (a *epilogueCommandRunnerAdapter) Run(ctx context.Context, command string) (string, string, int, error) {
	return a.runner(ctx, command, "")
}

// iterationLogWriterAdapter wraps *logger.Logger to satisfy epilogue.IterationLogWriter.
type iterationLogWriterAdapter struct {
	logger       *logger.Logger
	trendUpdater trendTrigger
}

func (a *iterationLogWriterAdapter) Write(log *logger.IterationLog) error {
	if a.logger == nil {
		return nil
	}
	if err := a.logger.LogIteration(log); err != nil {
		return err
	}
	if a.trendUpdater != nil {
		a.trendUpdater.Trigger()
	}
	return nil
}

type trendTrigger interface {
	Trigger()
}

// scopeGateSubBead represents a single sub-bead from the LLM decomposition response.
type scopeGateSubBead struct {
	Title           string   `json:"title"`
	ExpectedOutputs []string `json:"expected_outputs"`
}

// decomposerAdapter uses provider routing to invoke LLM-powered decomposition of oversized beads.
type decomposerAdapter struct {
	tracker     tracker.Client
	beads       BeadClient
	router      *provider.Router
	maxSubBeads int

	mu               sync.Mutex
	createdChildKeys map[string]map[string]struct{}
}

// DecomposeToSubTasks invokes the LLM to decompose a bead into sub-tasks,
// returning them as runtypes.SubTask structs without creating child beads.
func (a *decomposerAdapter) DecomposeToSubTasks(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
	if a.router == nil {
		return nil, fmt.Errorf("decomposerAdapter: no router configured for LLM decomposition")
	}

	p, _ := a.router.Select("decompose", provider.TierMedium)
	if p == nil {
		return nil, fmt.Errorf("decomposerAdapter: no provider available for decomposition")
	}

	promptText := buildScopeGateDecomposePrompt(b)
	result, err := p.Run(ctx, promptText, provider.TierMedium)
	if err != nil {
		return nil, fmt.Errorf("decomposerAdapter: LLM invocation failed: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("decomposerAdapter: LLM returned failure (exit %d)", result.ExitCode)
	}

	var subBeads []scopeGateSubBead
	if err := jsonutil.ExtractJSON(strings.TrimSpace(result.Output), &subBeads); err != nil {
		return nil, fmt.Errorf("decomposerAdapter: parsing LLM output: %w", err)
	}
	if err := validateRuntimeScopeGateDecomposeOutput(subBeads, b.Title, a.maxSubBeads); err != nil {
		return nil, err
	}

	tasks := make([]runtypes.SubTask, 0, len(subBeads))
	for _, sb := range subBeads {
		tasks = append(tasks, runtypes.SubTask{
			Title:              sb.Title,
			AcceptanceCriteria: sb.ExpectedOutputs,
		})
	}
	return tasks, nil
}

// CreateSubBeads creates child beads from sub-tasks and closes the parent bead.
func (a *decomposerAdapter) CreateSubBeads(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error {
	labels := a.resolveInheritedLabels(b)
	successfullyCreatedCount := 0
	for _, task := range tasks {
		sb := scopeGateSubBead{
			Title:           task.Title,
			ExpectedOutputs: task.AcceptanceCriteria,
		}
		dedupeLabel := scopeGateChildDedupeLabel(b.ID, sb)
		if a.hasCreatedChildKey(b.ID, dedupeLabel) {
			continue
		}
		exists, err := a.childWithDedupeLabelExistsWithClient(ctx, b.ID, dedupeLabel)
		if err != nil {
			return fmt.Errorf("decomposerAdapter: checking existing child bead %q: %w", task.Title, err)
		}
		if exists {
			a.rememberCreatedChildKey(b.ID, dedupeLabel)
			continue
		}

		childLabels := append(append([]string(nil), labels...), dedupeLabel)
		beadClient := a.beads
		if beadClient == nil {
			return fmt.Errorf("decomposerAdapter: unable to access bead client")
		}
		if _, err := beadClient.CreateWithParent(ctx, task.Title, b.Priority, childLabels, task.AcceptanceCriteria, b.ID); err != nil {
			if successfullyCreatedCount > 0 {
				return fmt.Errorf("decomposerAdapter: partial decomposition state: %w", escalation.ErrPartialDecompositionState)
			}
			return fmt.Errorf("decomposerAdapter: creating child bead %q: %w", task.Title, err)
		}
		a.rememberCreatedChildKey(b.ID, dedupeLabel)
		successfullyCreatedCount++
	}

	beadClient := a.beads
	if beadClient == nil {
		return fmt.Errorf("decomposerAdapter: unable to access bead client for closing")
	}
	if err := beadClient.Close(ctx, b.ID); err != nil {
		return fmt.Errorf("decomposerAdapter: closing parent bead: %w", err)
	}
	return nil
}

func (a *decomposerAdapter) Decompose(ctx context.Context, b *bead.Bead) error {
	if a.router == nil {
		return fmt.Errorf("decomposerAdapter: no router configured for LLM decomposition")
	}

	p, _ := a.router.Select("decompose", provider.TierMedium)
	if p == nil {
		return fmt.Errorf("decomposerAdapter: no provider available for decomposition")
	}

	promptText := buildScopeGateDecomposePrompt(b)
	result, err := p.Run(ctx, promptText, provider.TierMedium)
	if err != nil {
		return fmt.Errorf("decomposerAdapter: LLM invocation failed: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("decomposerAdapter: LLM returned failure (exit %d)", result.ExitCode)
	}

	var subBeads []scopeGateSubBead
	if err := jsonutil.ExtractJSON(strings.TrimSpace(result.Output), &subBeads); err != nil {
		return fmt.Errorf("decomposerAdapter: parsing LLM output: %w", err)
	}
	if err := validateRuntimeScopeGateDecomposeOutput(subBeads, b.Title, a.maxSubBeads); err != nil {
		return err
	}

	labels := a.resolveInheritedLabels(b)
	successfullyCreatedCount := 0
	for _, sb := range subBeads {
		dedupeLabel := scopeGateChildDedupeLabel(b.ID, sb)
		if a.hasCreatedChildKey(b.ID, dedupeLabel) {
			continue
		}
		exists, err := a.childWithDedupeLabelExistsWithClient(ctx, b.ID, dedupeLabel)
		if err != nil {
			return fmt.Errorf("decomposerAdapter: checking existing child bead %q: %w", sb.Title, err)
		}
		if exists {
			a.rememberCreatedChildKey(b.ID, dedupeLabel)
			continue
		}

		childLabels := append(append([]string(nil), labels...), dedupeLabel)
		beadClient := a.beads
		if beadClient == nil {
			return fmt.Errorf("decomposerAdapter: unable to access bead client")
		}
		if _, err := beadClient.CreateWithParent(ctx, sb.Title, b.Priority, childLabels, sb.ExpectedOutputs, b.ID); err != nil {
			// If we've already created some children and now one fails, this is a partial state
			if successfullyCreatedCount > 0 {
				return fmt.Errorf("decomposerAdapter: partial decomposition state: %w", escalation.ErrPartialDecompositionState)
			}
			return fmt.Errorf("decomposerAdapter: creating child bead %q: %w", sb.Title, err)
		}
		a.rememberCreatedChildKey(b.ID, dedupeLabel)
		successfullyCreatedCount++
	}

	beadClient := a.beads
	if beadClient == nil {
		return fmt.Errorf("decomposerAdapter: unable to access bead client for closing")
	}
	if err := beadClient.Close(ctx, b.ID); err != nil {
		return fmt.Errorf("decomposerAdapter: closing parent bead: %w", err)
	}
	return nil
}

func scopeGateSubBeadsToCandidates(subBeads []scopeGateSubBead) []validate.BeadCandidate {
	candidates := make([]validate.BeadCandidate, 0, len(subBeads))
	for _, sb := range subBeads {
		candidates = append(candidates, validate.BeadCandidate{
			Title:           sb.Title,
			ExpectedOutputs: sb.ExpectedOutputs,
		})
	}
	return candidates
}

func validateRuntimeScopeGateDecomposeOutput(subBeads []scopeGateSubBead, parentTitle string, maxSubBeads int) error {
	if maxSubBeads <= 0 {
		maxSubBeads = validate.MaxSubBeads
	}

	validation := validate.ValidateDecomposeOutputWithMax(
		scopeGateSubBeadsToCandidates(subBeads),
		validate.DecomposeValidationModeRuntime,
		parentTitle,
		maxSubBeads,
	)
	if len(validation.BatchViolations) > 0 {
		v := validation.BatchViolations[0]
		return fmt.Errorf("decomposerAdapter: decomposition contract violation [%s]: %s", v.Rule, v.Message)
	}
	if len(validation.Violations) > 0 {
		v := validation.Violations[0]
		return fmt.Errorf("decomposerAdapter: decomposition contract violation [%s]: bead %d: %s", v.Rule, v.BeadIndex, v.Message)
	}

	return nil
}

func scopeGateChildDedupeLabel(parentID string, sb scopeGateSubBead) string {
	const prefix = "scope_decomp:"

	outputs := make([]string, 0, len(sb.ExpectedOutputs))
	for _, output := range sb.ExpectedOutputs {
		outputs = append(outputs, normalizeScopeGateDedupeText(output))
	}
	sort.Strings(outputs)
	sum := sha1.Sum([]byte(parentID + "\x00" + normalizeScopeGateDedupeText(sb.Title) + "\x00" + strings.Join(outputs, "\x00")))
	return prefix + hex.EncodeToString(sum[:8])
}

func normalizeScopeGateDedupeText(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func (a *decomposerAdapter) hasCreatedChildKey(parentID, key string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	perParent := a.createdChildKeys[parentID]
	if perParent == nil {
		return false
	}
	_, ok := perParent[key]
	return ok
}

func (a *decomposerAdapter) rememberCreatedChildKey(parentID, key string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.createdChildKeys == nil {
		a.createdChildKeys = make(map[string]map[string]struct{})
	}
	if a.createdChildKeys[parentID] == nil {
		a.createdChildKeys[parentID] = make(map[string]struct{})
	}
	a.createdChildKeys[parentID][key] = struct{}{}
}

// childWithDedupeLabelExistsWithClient accepts a context parameter and threads it through to ListWithLabel.
func (a *decomposerAdapter) childWithDedupeLabelExistsWithClient(ctx context.Context, parentID, dedupeLabel string) (bool, error) {
	beadClient := a.beads
	if beadClient == nil {
		return false, fmt.Errorf("bead client is nil")
	}

	matches, err := beadClient.ListWithLabel(ctx, dedupeLabel)
	if err != nil {
		return false, err
	}
	for _, existing := range matches {
		if existing != nil && existing.Parent == parentID {
			return true, nil
		}
	}
	return false, nil
}

func (a *decomposerAdapter) resolveInheritedLabels(parent *bead.Bead) []string {
	const buildStrategyPrefix = "build_strategy:"
	const specPrefix = "spec:"

	labels := make([]string, 0, 4)
	appendUnique := func(label string) {
		if label == "" {
			return
		}
		for _, existing := range labels {
			if existing == label {
				return
			}
		}
		labels = append(labels, label)
	}

	for _, label := range labelsWithPrefix(parent.Labels, specPrefix) {
		appendUnique(label)
	}
	appendUnique(findLabelWithPrefix(parent.Labels, buildStrategyPrefix))

	beadClient := a.beads
	if beadClient == nil || parent.ID == "" {
		return labels
	}
	fullParent, err := beadClient.Show(context.Background(), parent.ID)
	if err != nil || fullParent == nil {
		return labels
	}
	for _, label := range labelsWithPrefix(fullParent.Labels, specPrefix) {
		appendUnique(label)
	}
	appendUnique(findLabelWithPrefix(fullParent.Labels, buildStrategyPrefix))

	return labels
}

func findLabelWithPrefix(labels []string, prefix string) string {
	for _, label := range labels {
		if strings.HasPrefix(label, prefix) {
			return label
		}
	}
	return ""
}

func labelsWithPrefix(labels []string, prefix string) []string {
	matches := make([]string, 0, len(labels))
	for _, label := range labels {
		if strings.HasPrefix(label, prefix) {
			matches = append(matches, label)
		}
	}
	return matches
}

// buildScopeGateDecomposePrompt builds the LLM prompt for scope gate decomposition.
func buildScopeGateDecomposePrompt(b *bead.Bead) string {
	outputs := strings.Join(b.ExpectedOutputs, "\n- ")
	if outputs != "" {
		outputs = "- " + outputs
	}
	return fmt.Sprintf(`You are decomposing an oversized task into smaller sub-tasks that each touch 5 or fewer files.

## Oversized Task
Title: %s
Description: %s
Expected outputs (too many):
%s

## Instructions
Split this task into 2-5 sub-tasks. Each sub-task must:
- Have a clear, specific title
- Touch 5 or fewer files (expected_outputs list)
- Together cover all the work of the original task

## Output
Output ONLY a JSON array. No markdown, no explanation.
Each element: {"title": "...", "expected_outputs": ["file1", "file2", ...]}
`, b.Title, b.Description, outputs)
}

// failureLearnerAdapter wraps analyzer and related dependencies to satisfy epilogue.FailureLearner.
type failureLearnerAdapter struct {
	renderer *prompt.Renderer
	router   *provider.Router
	analyzer FailureAnalyzer
	logFn    func(string, ...interface{})
}

func (a *failureLearnerAdapter) ExtractFailureLearning(ctx context.Context, beadID, beadTitle, failureOutput string) error {
	if a.analyzer == nil {
		return nil
	}
	b := &bead.Bead{ID: beadID, Title: beadTitle}
	analysis, err := a.analyzer.Analyze(ctx, b, failureOutput)
	if err != nil {
		if a.logFn != nil {
			a.logFn("Warning: failure analysis error: %v", err)
		}
		return nil
	}
	if analysis == nil || analysis.Learning == nil {
		return nil
	}
	if a.renderer != nil {
		lf := a.renderer.GetLearningsFile()
		if lf != nil {
			_, _ = lf.Add(beadID, *analysis.Learning, analysis.LearningCategory())
		}
	}
	return nil
}

var (
	_ epilogue.BeadLifecycle      = (*beadLifecycleAdapter)(nil)
	_ epilogue.StatusWriter       = (*statusWriterAdapter)(nil)
	_ epilogue.CommandRunner      = (*epilogueCommandRunnerAdapter)(nil)
	_ epilogue.IterationLogWriter = (*iterationLogWriterAdapter)(nil)
	_ epilogue.FailureLearner     = (*failureLearnerAdapter)(nil)
	_ prepare.Decomposer          = (*decomposerAdapter)(nil)
)
