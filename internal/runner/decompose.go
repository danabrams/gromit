package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// decomposeDepthWarnThreshold is the depth at which a warning is logged during decomposition.
// Depths above this threshold are suspicious even if not yet at the hard cap.
const decomposeDepthWarnThreshold = 5

const decomposeOutputPreviewLimit = 200

// defaultMaxDecomposeDepth is the fallback when Loop.MaxDecomposeDepth is not configured.
// Depth is measured by counting "." separators in the bead ID (e.g., "gromit-abc.1.2" = depth 2).
// This prevents runaway decomposition from creating IDs that exceed maxIDLength (128).
const defaultMaxDecomposeDepth = 10

// DecomposeTask calls Claude (opus) to decompose a task and returns parsed sub-tasks
// Does NOT create beads - just gets the decomposition
func (r *Runner) DecomposeTask(ctx context.Context, b *bead.Bead) ([]SubTask, error) {
	if r == nil {
		return nil, fmt.Errorf("runner is nil")
	}
	if b == nil {
		return nil, fmt.Errorf("bead is nil")
	}
	maxDepth := defaultMaxDecomposeDepth
	if r.cfg != nil && r.cfg.Loop.MaxDecomposeDepth > 0 {
		maxDepth = r.cfg.Loop.MaxDecomposeDepth
	}
	if depth := strings.Count(b.ID, "."); depth >= maxDepth {
		return nil, fmt.Errorf("bead %s is at decomposition depth %d (max %d): refusing to decompose further", b.ID, depth, maxDepth)
	} else if depth > decomposeDepthWarnThreshold {
		r.log("Warning: bead %s is at decomposition depth %d (warn threshold %d, max %d): consider splitting into more focused beads", b.ID, depth, decomposeDepthWarnThreshold, maxDepth)
	}

	if r.beads == nil {
		return nil, fmt.Errorf("runner beads client is nil")
	}
	if r.renderer == nil {
		return nil, fmt.Errorf("runner renderer is nil")
	}
	if r.router == nil {
		return nil, fmt.Errorf("runner router is nil")
	}

	// Get parent bead if exists
	parent, err := r.beads.GetParent(b)
	if err != nil {
		// Log warning but continue - decomposition can work without parent
		r.log("Warning: failed to get parent bead: %v", err)
	}

	// Build decompose context
	atddActive := bead.IsMethodologyActive(b.Labels, "atdd", r.cfg.Methodology.ATDD)
	decomposeCtx := &prompt.DecomposeContext{
		Bead:       b,
		ParentBead: parent,
		ATDDActive: atddActive,
	}

	// Render decompose prompt
	decomposedPrompt, err := r.renderer.RenderDecompose(decomposeCtx)
	if err != nil {
		return nil, fmt.Errorf("rendering decompose prompt: %w", err)
	}

	// Select provider via router (phase="decompose", tier="high" for opus-level complexity)
	p, _ := r.router.Select("decompose", provider.TierHigh)
	if p == nil {
		return nil, fmt.Errorf("no provider available for decomposition")
	}

	// Invoke provider with high tier
	result, err := p.Run(ctx, decomposedPrompt, provider.TierHigh)
	if err != nil {
		return nil, fmt.Errorf("decomposition invocation: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("decomposition returned nil result")
	}

	// Some providers may return non-zero exit codes even when they emit valid JSON.
	// Try to salvage parsable decomposition output before failing hard.
	if !result.Success && strings.TrimSpace(result.Output) == "" {
		return nil, fmt.Errorf("decomposition failed with exit code %d", result.ExitCode)
	}

	// Parse the output
	subTasks, err := parseDecomposeOutput(result.Output)
	if err != nil {
		outputPreview := previewDecomposeOutput(result.Output)
		if !result.Success {
			return nil, fmt.Errorf("decomposition failed with exit code %d and parsing failed: %v (output: %q)", result.ExitCode, err, outputPreview)
		}
		return nil, fmt.Errorf("parsing decomposition (output: %q): %w", outputPreview, err)
	}

	return subTasks, nil
}

// parseDecomposeOutput parses Claude's JSON array decompose output into []SubTask
// It's resilient to non-pure JSON output (e.g., explanatory text before/after the JSON)
func parseDecomposeOutput(output string) ([]SubTask, error) {
	if output == "" {
		return nil, fmt.Errorf("decompose output is empty")
	}

	var subTasks []SubTask
	parseArrayErr := jsonutil.ExtractArray(output, &subTasks)
	if parseArrayErr != nil {
		if strings.Contains(output, "[") && !strings.Contains(output, "]") {
			return nil, fmt.Errorf("parsing decompose output: malformed JSON array: missing closing bracket")
		}

		// Fallback for wrapper formats:
		// {"sub_tasks":[...]} / {"subtasks":[...]} / {"tasks":[...]} / {"items":[...]}
		var wrapped struct {
			SubTasks []SubTask `json:"sub_tasks"`
			Subtasks []SubTask `json:"subtasks"`
			Tasks    []SubTask `json:"tasks"`
			Items    []SubTask `json:"items"`
		}
		if err := jsonutil.ExtractObject(output, &wrapped); err == nil {
			switch {
			case len(wrapped.SubTasks) > 0:
				subTasks = wrapped.SubTasks
			case len(wrapped.Subtasks) > 0:
				subTasks = wrapped.Subtasks
			case len(wrapped.Tasks) > 0:
				subTasks = wrapped.Tasks
			case len(wrapped.Items) > 0:
				subTasks = wrapped.Items
			default:
				return nil, fmt.Errorf("parsing decompose output: %w", parseArrayErr)
			}
		} else {
			return nil, fmt.Errorf("parsing decompose output: %w", parseArrayErr)
		}
	}

	if len(subTasks) == 0 {
		return nil, fmt.Errorf("decompose output contains no sub-tasks")
	}

	for i := range subTasks {
		subTasks[i].NormalizeNilFields()
	}

	return subTasks, nil
}

func previewDecomposeOutput(output string) string {
	if len(output) > decomposeOutputPreviewLimit {
		return output[:decomposeOutputPreviewLimit] + "..."
	}
	return output
}

func buildSubTaskDescription(subTask SubTask) string {
	if subTask.Description == "" {
		return ""
	}

	if len(subTask.AcceptanceCriteria) == 0 {
		return subTask.Description
	}

	var descriptionBuilder strings.Builder
	descriptionBuilder.WriteString(subTask.Description)
	descriptionBuilder.WriteString("\n\nAcceptance criteria:\n")
	for _, ac := range subTask.AcceptanceCriteria {
		descriptionBuilder.WriteString("- ")
		descriptionBuilder.WriteString(ac)
		descriptionBuilder.WriteString("\n")
	}
	return descriptionBuilder.String()
}

// CreateSubBeads creates child beads from decomposed sub-tasks, comments on the original
// bead with the new sub-bead IDs, and closes the original bead.
func (r *Runner) CreateSubBeads(ctx context.Context, b *bead.Bead, subTasks []SubTask) error {
	if r == nil {
		return fmt.Errorf("runner is nil")
	}
	if b == nil {
		return fmt.Errorf("bead is nil")
	}
	if len(subTasks) == 0 {
		return fmt.Errorf("no sub-tasks to create")
	}
	if len(subTasks) == 1 {
		r.log("Warning: bead %s decomposed into a single child — scope is already atomic; skipping decomposition and executing directly", b.ID)
		return fmt.Errorf("single child decomposition for bead %s: scope is atomic, not decomposing further", b.ID)
	}
	if r.beads == nil {
		return fmt.Errorf("runner beads client is nil")
	}

	// Create beads for each sub-task
	var createdIDs []string
	for i, subTask := range subTasks {
		r.log("Creating sub-bead %d/%d: %s", i+1, len(subTasks), subTask.Title)

		// Build description from description and acceptance criteria
		description := buildSubTaskDescription(subTask)

		// Inherit labels from parent and inject methodology labels if needed
		labels := r.injectMethodologyLabels(b.Labels)

		createdBead, err := r.beads.CreateWithParentAndDescription(
			subTask.Title,
			b.Priority, // Inherit priority from parent
			labels,     // Inherit labels from parent with methodology injection
			nil,        // No expected outputs
			b.ID,       // Set parent to original bead
			description,
		)
		if err != nil {
			r.log("Warning: failed to create sub-bead: %v", err)
			continue
		}

		createdIDs = append(createdIDs, createdBead.ID)
		r.log("Created sub-bead: %s", createdBead.ID)

		// Log warning about DependsOn not yet supported
		if subTask.DependsOn != nil {
			r.log("Warning: DependsOn field not yet supported for sub-task %d (index %d)", *subTask.DependsOn, i)
		}
	}

	if len(createdIDs) == 0 {
		return fmt.Errorf("failed to create any sub-beads")
	}

	// Comment on original bead listing the new sub-bead IDs
	var commentBuilder strings.Builder
	commentBuilder.WriteString(fmt.Sprintf("Decomposed into %d sub-beads:\n", len(createdIDs)))
	for i, id := range createdIDs {
		commentBuilder.WriteString(fmt.Sprintf("%d. %s\n", i+1, id))
	}
	comment := commentBuilder.String()
	if err := r.beads.AddComment(b.ID, comment); err != nil {
		r.log("Warning: failed to add comment to bead: %v", err)
	}

	// Close the original bead
	if err := r.beads.Close(b.ID); err != nil {
		r.log("Warning: failed to close bead: %v", err)
	}

	// Sync bd state
	if err := r.beads.Sync(); err != nil {
		r.log("Warning: failed to sync beads: %v", err)
	}

	r.log("Successfully created %d sub-beads", len(createdIDs))
	return nil
}

// injectMethodologyLabels takes parent labels and adds methodology labels when
// the methodology is globally active but not already present in the parent's labels.
// This ensures sub-beads inherit methodology even when set globally rather than via labels.
func (r *Runner) injectMethodologyLabels(parentLabels []string) []string {
	if r == nil || r.cfg == nil {
		return parentLabels
	}

	// Start with a copy of parent labels
	labels := make([]string, len(parentLabels))
	copy(labels, parentLabels)

	// Check if ATDD label should be added
	if r.cfg.Methodology.ATDD {
		if !hasMethodologyLabel(labels, "atdd") {
			labels = append(labels, "atdd:true")
		}
	}

	// Check if TDD label should be added
	if r.cfg.Methodology.TDD {
		if !hasMethodologyLabel(labels, "tdd") {
			labels = append(labels, "tdd:true")
		}
	}

	return labels
}

func hasMethodologyLabel(labels []string, methodology string) bool {
	trueLabel := methodology + ":true"
	falseLabel := methodology + ":false"
	for _, label := range labels {
		if label == trueLabel || label == falseLabel {
			return true
		}
	}
	return false
}

type _ = runtypes.SubTask
